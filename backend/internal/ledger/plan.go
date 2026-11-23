package ledger

import (
	"context"
	"database/sql"
	"errors"

	"github.com/Rakeemovic/slowpoke-oasis/backend/internal/store"
)

// Errors for planned amounts and monthly plans.
var (
	ErrInvalidPlanScope         = errors.New(`monthly pots need scope "this_month" or "from_now_on"; one-time pots take no scope`)
	ErrMonthlyPotNeedsMonth     = errors.New("reports of a monthly pot cover one month: give month instead of from and to")
	ErrCategoryPlannedElsewhere = errors.New("this category is already planned in another monthly pot")
)

// PlanScope says which months a change to the plan of a monthly pot affects.
type PlanScope string

// Scopes of a change to a monthly plan. The current month is the payday
// period that contains the app's date; past months cannot be changed.
const (
	ScopeThisMonth PlanScope = "this_month"  // only the current month
	ScopeFromNowOn PlanScope = "from_now_on" // the current month and the template
)

// PlanSource says where the planned amounts of a pot summary come from.
type PlanSource string

// Sources of planned amounts.
const (
	PlanOfPot     PlanSource = "pot"     // one-time pot: the plan of the pot
	PlanOfMonth   PlanSource = "month"   // monthly pot: the plan stored for the month
	PlanPreview   PlanSource = "preview" // future month: the template, not stored
	PlanNotStored PlanSource = "none"    // past month that never got a plan
)

// PlanChange sets or removes the planned amount of a category in a pot.
type PlanChange struct {
	LedgerID    string
	PotID       string
	CategoryID  string
	AmountCents int64     // ignored when removing
	Scope       PlanScope // monthly pots only
	Today       string    // YYYY-MM-DD, the app's date; decides the current month
}

// SetPlannedAmount sets the planned amount of a category in a pot (rule P1).
// An existing plan for the same category is replaced.
//
// In a one-time pot, the amount is the plan of the pot. In a monthly pot, it
// changes the current month, and with ScopeFromNowOn also the template that
// later months are created from.
func (s *Store) SetPlannedAmount(ctx context.Context, c PlanChange) error {
	if c.AmountCents < 0 {
		return ErrInvalidAmount
	}
	return s.inTx(ctx, func(tx *sql.Tx) error {
		monthly, err := isMonthlyPot(ctx, tx, c.LedgerID, c.PotID)
		if err != nil {
			return err
		}
		if !monthly {
			if c.Scope != "" {
				return ErrInvalidPlanScope
			}
			return upsertTemplateLine(ctx, tx, c)
		}
		if !c.Scope.valid() {
			return ErrInvalidPlanScope
		}
		month, err := prepareCurrentMonth(ctx, tx, c.LedgerID, c.Today)
		if err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO period_plan (pot_id, ledger_id, category_id, month, amount_cents)
			VALUES (?, ?, ?, ?, ?)
			ON CONFLICT (pot_id, category_id, month) DO UPDATE SET amount_cents = excluded.amount_cents`,
			c.PotID, c.LedgerID, c.CategoryID, month, c.AmountCents,
		); err != nil {
			return planConflict(err)
		}
		if c.Scope == ScopeThisMonth {
			return nil
		}
		if err := ensureNotPlannedInOtherMonthlyPot(ctx, tx, c); err != nil {
			return err
		}
		return upsertTemplateLine(ctx, tx, c)
	})
}

// DeletePlannedAmount removes the planned amount of a category from a pot.
// For monthly pots, the scope works as in SetPlannedAmount.
func (s *Store) DeletePlannedAmount(ctx context.Context, c PlanChange) error {
	return s.inTx(ctx, func(tx *sql.Tx) error {
		monthly, err := isMonthlyPot(ctx, tx, c.LedgerID, c.PotID)
		if err != nil {
			return err
		}
		if !monthly {
			if c.Scope != "" {
				return ErrInvalidPlanScope
			}
			n, err := deleteTemplateLine(ctx, tx, c)
			if err == nil && n == 0 {
				err = ErrPotLineNotFound
			}
			return err
		}
		if !c.Scope.valid() {
			return ErrInvalidPlanScope
		}
		month, err := prepareCurrentMonth(ctx, tx, c.LedgerID, c.Today)
		if err != nil {
			return err
		}
		res, err := tx.ExecContext(ctx, `
			DELETE FROM period_plan WHERE pot_id = ? AND ledger_id = ? AND category_id = ? AND month = ?`,
			c.PotID, c.LedgerID, c.CategoryID, month)
		if err != nil {
			return err
		}
		removed, err := res.RowsAffected()
		if err != nil {
			return err
		}
		if c.Scope == ScopeFromNowOn {
			n, err := deleteTemplateLine(ctx, tx, c)
			if err != nil {
				return err
			}
			removed += n
		}
		if removed == 0 {
			return ErrPotLineNotFound
		}
		return nil
	})
}

func (sc PlanScope) valid() bool {
	return sc == ScopeThisMonth || sc == ScopeFromNowOn
}

// isMonthlyPot reports whether a pot is a monthly pot.
func isMonthlyPot(ctx context.Context, q queryer, ledgerID, potID string) (bool, error) {
	var kind string
	err := q.QueryRowContext(ctx,
		`SELECT kind FROM pot WHERE id = ? AND ledger_id = ?`, potID, ledgerID,
	).Scan(&kind)
	if errors.Is(err, sql.ErrNoRows) {
		return false, ErrPotNotFound
	}
	return kind == "monthly", err
}

// currentMonth returns the month of the payday period that contains today.
func currentMonth(ctx context.Context, q queryer, ledgerID, today string) (string, error) {
	payday, err := ledgerPayday(ctx, q, ledgerID)
	if err != nil {
		return "", err
	}
	return MonthOf(today, payday)
}

// prepareCurrentMonth returns the current month and makes sure its plans exist.
func prepareCurrentMonth(ctx context.Context, tx *sql.Tx, ledgerID, today string) (string, error) {
	month, err := currentMonth(ctx, tx, ledgerID, today)
	if err != nil {
		return "", err
	}
	return month, ensureMonthPlans(ctx, tx, ledgerID, month)
}

// ensureMonthPlans copies the templates of all active monthly pots of a ledger
// into the plans of the month, once per ledger and month (rule P4). Calling it
// again does nothing, even if lines of the month were removed since.
//
// All pots are filled at once: if only one pot were filled, another pot could
// meanwhile plan a category for this month that its own template would add
// later, which would break rule P3.
func ensureMonthPlans(ctx context.Context, tx *sql.Tx, ledgerID, month string) error {
	res, err := tx.ExecContext(ctx,
		`INSERT INTO period_plan_month (ledger_id, month) VALUES (?, ?) ON CONFLICT DO NOTHING`,
		ledgerID, month)
	if err != nil {
		return err
	}
	if n, err := res.RowsAffected(); err != nil || n == 0 {
		return err
	}
	_, err = tx.ExecContext(ctx, `
		INSERT INTO period_plan (pot_id, ledger_id, category_id, month, amount_cents)
		SELECT pl.pot_id, pl.ledger_id, pl.category_id, ?, pl.amount_cents
		FROM pot_line pl
		JOIN pot p ON p.id = pl.pot_id
		WHERE pl.ledger_id = ? AND p.kind = 'monthly' AND p.archived = 0`,
		month, ledgerID)
	return err
}

// monthPlanStored reports whether the plans of a month were ever created.
func monthPlanStored(ctx context.Context, q queryer, ledgerID, month string) (bool, error) {
	var stored bool
	err := q.QueryRowContext(ctx,
		`SELECT EXISTS (SELECT 1 FROM period_plan_month WHERE ledger_id = ? AND month = ?)`,
		ledgerID, month,
	).Scan(&stored)
	return stored, err
}

// ensureNotPlannedInOtherMonthlyPot enforces rule P3 for the templates, which
// share the table pot_line with one-time pots and so cannot use an index.
func ensureNotPlannedInOtherMonthlyPot(ctx context.Context, tx *sql.Tx, c PlanChange) error {
	var taken bool
	err := tx.QueryRowContext(ctx, `
		SELECT EXISTS (
			SELECT 1 FROM pot_line pl
			JOIN pot p ON p.id = pl.pot_id
			WHERE pl.ledger_id = ? AND pl.category_id = ? AND pl.pot_id <> ? AND p.kind = 'monthly'
		)`, c.LedgerID, c.CategoryID, c.PotID,
	).Scan(&taken)
	if err != nil {
		return err
	}
	if taken {
		return ErrCategoryPlannedElsewhere
	}
	return nil
}

func upsertTemplateLine(ctx context.Context, tx *sql.Tx, c PlanChange) error {
	_, err := tx.ExecContext(ctx, `
		INSERT INTO pot_line (pot_id, ledger_id, category_id, amount_cents)
		VALUES (?, ?, ?, ?)
		ON CONFLICT (pot_id, category_id) DO UPDATE SET amount_cents = excluded.amount_cents`,
		c.PotID, c.LedgerID, c.CategoryID, c.AmountCents,
	)
	return err
}

func deleteTemplateLine(ctx context.Context, tx *sql.Tx, c PlanChange) (int64, error) {
	res, err := tx.ExecContext(ctx,
		`DELETE FROM pot_line WHERE pot_id = ? AND ledger_id = ? AND category_id = ?`,
		c.PotID, c.LedgerID, c.CategoryID)
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}

// planConflict turns a violation of the index for rule P3 into its error.
func planConflict(err error) error {
	if store.IsUniqueViolation(err) {
		return ErrCategoryPlannedElsewhere
	}
	return err
}
