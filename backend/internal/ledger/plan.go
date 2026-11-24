package ledger

import (
	"context"
	"database/sql"
	"errors"

	"github.com/Rakeemovic/slowpoke-oasis/backend/internal/store"
)

// Errors for planned amounts and monthly plans.
var (
	ErrInvalidPlanScope         = errors.New(`scope must be "this_month" or "from_now_on"`)
	ErrBudgetOnlyMainPot        = errors.New("only the main pot of a ledger has a budget")
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

// SetPlannedAmount sets the planned amount of a category in the budget of a
// ledger (rule P1), which belongs to its main pot (rule P7). An existing plan
// for the same category is replaced. The change applies to the current month,
// and with ScopeFromNowOn also to the template that later months are created
// from.
func (s *Store) SetPlannedAmount(ctx context.Context, c PlanChange) error {
	if c.AmountCents < 0 {
		return ErrInvalidAmount
	}
	return s.inTx(ctx, func(tx *sql.Tx) error {
		month, err := prepareBudgetChange(ctx, tx, c)
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
		return upsertTemplateLine(ctx, tx, c)
	})
}

// DeletePlannedAmount removes the planned amount of a category from the
// budget. The scope works as in SetPlannedAmount.
func (s *Store) DeletePlannedAmount(ctx context.Context, c PlanChange) error {
	return s.inTx(ctx, func(tx *sql.Tx) error {
		month, err := prepareBudgetChange(ctx, tx, c)
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
// prepareBudgetChange checks a change to the budget and returns the current
// month, whose plan it makes sure exists.
func prepareBudgetChange(ctx context.Context, tx *sql.Tx, c PlanChange) (string, error) {
	if err := ensureMainPot(ctx, tx, c.LedgerID, c.PotID); err != nil {
		return "", err
	}
	if !c.Scope.valid() {
		return "", ErrInvalidPlanScope
	}
	return prepareCurrentMonth(ctx, tx, c.LedgerID, c.Today)
}

// ensureMainPot rejects pots other than the main pot of the ledger (rule P7).
func ensureMainPot(ctx context.Context, q queryer, ledgerID, potID string) error {
	var isDefault bool
	err := q.QueryRowContext(ctx,
		`SELECT is_default FROM pot WHERE id = ? AND ledger_id = ?`, potID, ledgerID,
	).Scan(&isDefault)
	if errors.Is(err, sql.ErrNoRows) {
		return ErrPotNotFound
	}
	if err == nil && !isDefault {
		return ErrBudgetOnlyMainPot
	}
	return err
}

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

// ensureMonthPlans copies the template of the main pot of a ledger into the
// plan of the month, once per ledger and month (rule P4). Calling it again
// does nothing, even if lines of the month were removed since. Other pots
// have no budget (rule P7): their old templates stay, but are not copied.
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
		WHERE pl.ledger_id = ? AND p.is_default = 1`,
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
