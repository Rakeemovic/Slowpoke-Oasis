package ledger

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"time"
)

// Errors for pots, planned amounts and the rules that connect them to transactions.
var (
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
	if _, err := time.Parse("2006-01-02", date); err != nil {
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
	id, err := newID()
	if err != nil {
		return "", err
	}
	_, err = s.db.ExecContext(ctx, `
		INSERT INTO pot (id, ledger_id, name, kind, is_default, target_cents, target_date, archived)
		VALUES (?, ?, ?, 'one_time', 0, ?, ?, 0)`,
		id, in.LedgerID, name, nullInt(in.TargetCents), nullIfEmpty(in.TargetDate),
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

	var pots []Pot
	for rows.Next() {
		var p Pot
		var isDefault, archived int
		if err := rows.Scan(&p.ID, &p.Name, &p.Kind, &isDefault, &archived,
			&p.TargetCents, &p.TargetDate, &p.BalanceCents); err != nil {
			rows.Close()
			return nil, err
		}
		p.LedgerID = ledgerID
		p.IsDefault = isDefault == 1
		p.Archived = archived == 1
		pots = append(pots, p)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return nil, err
	}
	// Close before the next query: the store uses a single connection.
	rows.Close()

	for i := range pots {
		lines, err := s.plannedAmounts(ctx, pots[i].ID)
		if err != nil {
			return nil, err
		}
		pots[i].Planned = lines
	}
	return pots, nil
}

// plannedAmounts returns the planned amounts of one pot, ordered by category.
func (s *Store) plannedAmounts(ctx context.Context, potID string) ([]PotLine, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT category_id, amount_cents FROM pot_line WHERE pot_id = ? ORDER BY category_id`, potID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []PotLine
	for rows.Next() {
		var l PotLine
		if err := rows.Scan(&l.CategoryID, &l.AmountCents); err != nil {
			return nil, err
		}
		out = append(out, l)
	}
	return out, rows.Err()
}

// UpdatePot renames, changes the target, or archives a pot. The default pot
// can be renamed and given a target but never archived (rule T2).
func (s *Store) UpdatePot(ctx context.Context, ledgerID, potID string, u PotUpdate) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback() // no effect after a successful Commit

	var name string
	var isDefault, archived int
	var target sql.NullInt64
	var date sql.NullString
	err = tx.QueryRowContext(ctx, `
		SELECT name, is_default, archived, target_cents, target_date
		FROM pot WHERE id = ? AND ledger_id = ?`, potID, ledgerID,
	).Scan(&name, &isDefault, &archived, &target, &date)
	if err != nil {
		return err // sql.ErrNoRows when the pot does not exist in this ledger
	}

	newName := name
	if u.Name != nil {
		n, err := cleanName(*u.Name)
		if err != nil {
			return err
		}
		newName = n
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
	newArchived := archived == 1
	if u.Archived != nil {
		newArchived = *u.Archived
	}

	if err := validateTarget(newTarget, newDate); err != nil {
		return err
	}
	if isDefault == 1 && newArchived {
		return ErrDefaultPotProtected
	}

	if _, err := tx.ExecContext(ctx, `
		UPDATE pot SET name = ?, target_cents = ?, target_date = ?, archived = ?
		WHERE id = ? AND ledger_id = ?`,
		newName, nullInt(newTarget), nullIfEmpty(newDate), boolInt(newArchived), potID, ledgerID,
	); err != nil {
		return nameError(err)
	}
	return tx.Commit()
}

// SetPlannedAmount sets the planned amount of a category in a pot (rule P1).
// An existing plan for the same category is replaced.
func (s *Store) SetPlannedAmount(ctx context.Context, ledgerID, potID, categoryID string, amountCents int64) error {
	if amountCents < 0 {
		return ErrInvalidAmount
	}
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO pot_line (pot_id, ledger_id, category_id, amount_cents)
		VALUES (?, ?, ?, ?)
		ON CONFLICT (pot_id, category_id) DO UPDATE SET amount_cents = excluded.amount_cents`,
		potID, ledgerID, categoryID, amountCents,
	)
	return err
}

// DeletePlannedAmount removes a planned amount from a pot.
func (s *Store) DeletePlannedAmount(ctx context.Context, ledgerID, potID, categoryID string) error {
	res, err := s.db.ExecContext(ctx, `
		DELETE FROM pot_line WHERE pot_id = ? AND ledger_id = ? AND category_id = ?`,
		potID, ledgerID, categoryID,
	)
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return ErrPotLineNotFound
	}
	return nil
}

// ensurePotActive rejects transactions for archived pots (rule T5). A missing
// pot is left to the foreign key check.
func (s *Store) ensurePotActive(ctx context.Context, ledgerID, potID string) error {
	var archived int
	err := s.db.QueryRowContext(ctx,
		`SELECT archived FROM pot WHERE id = ? AND ledger_id = ?`, potID, ledgerID,
	).Scan(&archived)
	if errors.Is(err, sql.ErrNoRows) {
		return nil
	}
	if err != nil {
		return err
	}
	if archived == 1 {
		return ErrPotArchived
	}
	return nil
}

// ensureCategoryKind checks that income uses income categories and expenses
// use expense categories (rule K3). An unknown category is left to the foreign key.
func (s *Store) ensureCategoryKind(ctx context.Context, ledgerID, categoryID string, typ TxType) error {
	var isIncome int
	err := s.db.QueryRowContext(ctx,
		`SELECT is_income FROM category WHERE id = ? AND ledger_id = ?`, categoryID, ledgerID,
	).Scan(&isIncome)
	if errors.Is(err, sql.ErrNoRows) {
		return nil
	}
	if err != nil {
		return err
	}
	if (isIncome == 1) != (typ == TypeIncome) {
		return ErrCategoryKindMismatch
	}
	return nil
}

// nullInt maps 0 to SQL NULL, used for optional targets.
func nullInt(v int64) any {
	if v == 0 {
		return nil
	}
	return v
}

func boolInt(b bool) int {
	if b {
		return 1
	}
	return 0
}
