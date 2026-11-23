package ledger

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
)

// Errors for pots, planned amounts and the rules that connect them to transactions.
var (
	ErrPotNotFound           = errors.New("pot not found")
	ErrInvalidTarget         = errors.New("target_cents must not be negative (0 means no target)")
	ErrTargetDateNeedsTarget = errors.New("a target date needs a target amount")
	ErrInvalidAmount         = errors.New("amount_cents must not be negative")
	ErrPotArchived           = errors.New("this pot is archived and accepts no new transactions")
	ErrDefaultPotProtected   = errors.New("the default pot cannot be archived")
	ErrPotLineNotFound       = errors.New("no planned amount for this category in this pot")
	ErrCategoryKindMismatch  = errors.New("income needs an income category, expenses need an expense category")
)

// NewPot is the input for creating a custom pot such as a savings goal.
// A TargetCents of 0 means no target; an empty TargetDate means no date.
type NewPot struct {
	LedgerID    string
	Name        string
	TargetCents int64
	TargetDate  string // YYYY-MM-DD
}

// PotUpdate changes an existing pot. Nil fields stay unchanged.
// Setting TargetCents to 0 removes the target and its date.
type PotUpdate struct {
	Name        *string
	TargetCents *int64
	TargetDate  *string
	Archived    *bool
}

// PotLine is a planned amount for one category within a pot.
type PotLine struct {
	CategoryID  string
	AmountCents int64
}

// Pot is a pot with its current balance (calculated, never stored) and its plan.
type Pot struct {
	ID           string
	LedgerID     string
	Name         string
	Kind         string
	IsDefault    bool
	Archived     bool
	TargetCents  int64
	TargetDate   string
	BalanceCents int64
	Planned      []PotLine
}

// validateTarget checks rules T3 and T4.
func validateTarget(target int64, date string) error {
	if target < 0 {
		return ErrInvalidTarget
	}
	if date == "" {
		return nil
	}
	if target == 0 {
		return ErrTargetDateNeedsTarget
	}
	if !validDate(date) {
		return ErrInvalidDate
	}
	return nil
}

// CreatePot adds a custom one-time pot to a ledger and returns its ID.
func (s *Store) CreatePot(ctx context.Context, in NewPot) (string, error) {
	name, err := cleanName(in.Name)
	if err != nil {
		return "", err
	}
	if err := validateTarget(in.TargetCents, in.TargetDate); err != nil {
		return "", err
	}
	id := newID()
	_, err = s.db.ExecContext(ctx, `
		INSERT INTO pot (id, ledger_id, name, kind, is_default, target_cents, target_date, archived)
		VALUES (?, ?, ?, 'one_time', 0, ?, ?, 0)`,
		id, in.LedgerID, name, nullIfZero(in.TargetCents), nullIfEmpty(in.TargetDate),
	)
	if err != nil {
		return "", nameError(err)
	}
	return id, nil
}

// ListPots returns all pots of a ledger, the default pot first. The balance of
// each pot follows the rules in docs/data-model.md: incoming transfers and
// income, minus expenses and outgoing transfers. Deleted rows are ignored.
func (s *Store) ListPots(ctx context.Context, ledgerID string) ([]Pot, error) {
	// Read the plans first: the store uses a single connection, so one result
	// set must be finished before the next query starts.
	planned, err := s.plannedAmounts(ctx, ledgerID)
	if err != nil {
		return nil, err
	}

	rows, err := s.db.QueryContext(ctx, `
		SELECT p.id, p.name, p.kind, p.is_default, p.archived,
		       COALESCE(p.target_cents, 0), COALESCE(p.target_date, ''),
		       (SELECT COALESCE(SUM(t.amount_cents), 0) FROM transactions t
		         WHERE t.pot_id = p.id AND t.deleted_at IS NULL
		           AND t.type IN ('expense', 'income', 'transfer_out', 'transfer_in'))
		     - (SELECT COALESCE(SUM(t.amount_cents), 0) FROM transactions t
		         WHERE t.pot_id = p.id AND t.type = 'transfer' AND t.deleted_at IS NULL)
		     + (SELECT COALESCE(SUM(t.amount_cents), 0) FROM transactions t
		         WHERE t.counterpart_pot_id = p.id AND t.type = 'transfer' AND t.deleted_at IS NULL)
		FROM pot p
		WHERE p.ledger_id = ?
		ORDER BY p.is_default DESC, p.name`, ledgerID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var pots []Pot
	for rows.Next() {
		p := Pot{LedgerID: ledgerID}
		if err := rows.Scan(&p.ID, &p.Name, &p.Kind, &p.IsDefault, &p.Archived,
			&p.TargetCents, &p.TargetDate, &p.BalanceCents); err != nil {
			return nil, err
		}
		p.Planned = planned[p.ID]
		pots = append(pots, p)
	}
	return pots, rows.Err()
}

// plannedAmounts returns the planned amounts of all pots of a ledger by pot
// ID, each list ordered by category.
func (s *Store) plannedAmounts(ctx context.Context, ledgerID string) (map[string][]PotLine, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT pot_id, category_id, amount_cents
		FROM pot_line
		WHERE pot_id IN (SELECT id FROM pot WHERE ledger_id = ?)
		ORDER BY pot_id, category_id`, ledgerID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := map[string][]PotLine{}
	for rows.Next() {
		var potID string
		var l PotLine
		if err := rows.Scan(&potID, &l.CategoryID, &l.AmountCents); err != nil {
			return nil, err
		}
		out[potID] = append(out[potID], l)
	}
	return out, rows.Err()
}

// UpdatePot renames, changes the target, or archives a pot. The default pot
// can be renamed and given a target but never archived (rule T2).
func (s *Store) UpdatePot(ctx context.Context, ledgerID, potID string, u PotUpdate) error {
	return s.inTx(ctx, func(tx *sql.Tx) error {
		var name string
		var isDefault, archived bool
		var target sql.NullInt64
		var date sql.NullString
		err := tx.QueryRowContext(ctx, `
			SELECT name, is_default, archived, target_cents, target_date
			FROM pot WHERE id = ? AND ledger_id = ?`, potID, ledgerID,
		).Scan(&name, &isDefault, &archived, &target, &date)
		if errors.Is(err, sql.ErrNoRows) {
			return ErrPotNotFound
		}
		if err != nil {
			return err
		}

		if u.Name != nil {
			if name, err = cleanName(*u.Name); err != nil {
				return err
			}
		}
		newTarget := target.Int64
		if u.TargetCents != nil {
			newTarget = *u.TargetCents
		}
		newDate := date.String
		if u.TargetDate != nil {
			newDate = strings.TrimSpace(*u.TargetDate)
		}
		if u.TargetCents != nil && *u.TargetCents == 0 {
			newDate = "" // removing the target also removes its date
		}
		if u.Archived != nil {
			archived = *u.Archived
		}

		if err := validateTarget(newTarget, newDate); err != nil {
			return err
		}
		if isDefault && archived {
			return ErrDefaultPotProtected
		}

		_, err = tx.ExecContext(ctx, `
			UPDATE pot SET name = ?, target_cents = ?, target_date = ?, archived = ?
			WHERE id = ? AND ledger_id = ?`,
			name, nullIfZero(newTarget), nullIfEmpty(newDate), archived, potID, ledgerID,
		)
		return nameError(err)
	})
}

// resolvePot returns potID, or the default pot of the ledger if potID is empty
// (rule X5).
func resolvePot(ctx context.Context, q queryer, ledgerID, potID string) (string, error) {
	if potID != "" {
		return potID, nil
	}
	var id string
	err := q.QueryRowContext(ctx,
		`SELECT id FROM pot WHERE ledger_id = ? AND is_default = 1`, ledgerID,
	).Scan(&id)
	if errors.Is(err, sql.ErrNoRows) {
		return "", fmt.Errorf("default pot of ledger %q: %w", ledgerID, ErrPotNotFound)
	}
	return id, err
}

// ensurePotActive rejects transactions for archived pots (rule T5). A missing
// pot is left to the foreign key check.
func ensurePotActive(ctx context.Context, q queryer, ledgerID, potID string) error {
	var archived bool
	err := q.QueryRowContext(ctx,
		`SELECT archived FROM pot WHERE id = ? AND ledger_id = ?`, potID, ledgerID,
	).Scan(&archived)
	if errors.Is(err, sql.ErrNoRows) {
		return nil
	}
	if err != nil {
		return err
	}
	if archived {
		return ErrPotArchived
	}
	return nil
}
