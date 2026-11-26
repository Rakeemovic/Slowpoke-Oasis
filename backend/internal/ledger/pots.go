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
	ErrInvalidRate           = errors.New("monthly_rate_cents must not be negative (0 means no fixed rate)")
	ErrRateWithTargetDate    = errors.New("a fixed monthly rate and a target date exclude each other")
	ErrGoalRateNotAllowed    = errors.New("the main pot and reserves cannot save a fixed monthly rate")
	ErrSeparateNotAllowed    = errors.New("the main pot and reserves cannot be separate accounts")
	ErrInvalidMultiplier     = errors.New("round_up_multiplier must be 0 (off), 1, 2, 3 or 5")
	ErrRoundUpNotAllowed     = errors.New("round-ups go to a pot that is neither the main pot nor a reserve, in a personal ledger")
	ErrInvalidAmount         = errors.New("amount_cents must not be negative")
	ErrPotArchived           = errors.New("this pot is archived and accepts no new transactions")
	ErrDefaultPotProtected   = errors.New("the default pot cannot be archived")
	ErrPotLineNotFound       = errors.New("no planned amount for this category in this pot")
	ErrCategoryKindMismatch  = errors.New("income needs an income category, expenses need an expense category")
)

// NewPot is the input for creating a custom pot such as a savings goal.
// A TargetCents of 0 means no target; an empty TargetDate means no date; a
// MonthlyRateCents of 0 means no fixed rate (rules T3, T4, T6).
type NewPot struct {
	LedgerID         string
	Name             string
	TargetCents      int64
	TargetDate       string // YYYY-MM-DD
	MonthlyRateCents int64
	// SeparateAccount marks a real sub-account at the bank (rule T9).
	SeparateAccount bool
	// RoundUpMultiplier makes the new pot collect round-ups (0 = no); Today is
	// the first day that is rounded up. Both work as in PotUpdate (rule T10).
	RoundUpMultiplier int
	Today             string
}

// PotUpdate changes an existing pot. Nil fields stay unchanged.
// Setting TargetCents to 0 removes the target and its date; setting
// MonthlyRateCents to 0 removes the fixed rate.
type PotUpdate struct {
	Name             *string
	TargetCents      *int64
	TargetDate       *string
	MonthlyRateCents *int64
	SeparateAccount  *bool
	// RoundUpMultiplier switches the round-up of the pot: 0 off, otherwise
	// 1, 2, 3 or 5 (rule T10). Today (the app's date, YYYY-MM-DD) is the
	// first day that is rounded up when it is switched on.
	RoundUpMultiplier *int
	Today             string
	Archived          *bool
}

// PotLine is a planned amount for one category within a pot.
type PotLine struct {
	CategoryID  string
	AmountCents int64
}

// Pot is a pot with its current balance (calculated, never stored) and its plan.
type Pot struct {
	ID          string
	LedgerID    string
	Name        string
	Kind        string
	IsDefault   bool
	Archived    bool
	TargetCents int64
	TargetDate  string
	// MonthlyRateCents is the fixed rate the pot saves per payday period, or
	// 0 (rule T6).
	MonthlyRateCents int64
	// IsSeparateAccount: the pot is a real sub-account at the bank (rule T9).
	IsSeparateAccount bool
	// RoundUpMultiplier is 0 or the multiplier of the round-up the pot
	// collects; RoundUpFrom the first booking day it applies to (rule T10).
	RoundUpMultiplier int
	RoundUpFrom       string
	// ReserveFor is the recurring payment this pot is the reserve of, or ""
	// (rule R10).
	ReserveFor   string
	BalanceCents int64
	Planned      []PotLine
}

// validateGoal checks rules T3, T4 and T6: the target, its date and the
// fixed rate of a pot.
func validateGoal(target int64, date string, rate int64) error {
	if err := validateTarget(target, date); err != nil {
		return err
	}
	if rate < 0 {
		return ErrInvalidRate
	}
	if rate > 0 && date != "" {
		return ErrRateWithTargetDate
	}
	return nil
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

// CreatePot adds a custom one-time pot to a ledger and returns its ID. With a
// round-up multiplier it also takes over the round-up from the pot that
// collected it before, as switching it on later would (rule T10).
func (s *Store) CreatePot(ctx context.Context, in NewPot) (string, error) {
	name, err := cleanName(in.Name)
	if err != nil {
		return "", err
	}
	if err := validateGoal(in.TargetCents, in.TargetDate, in.MonthlyRateCents); err != nil {
		return "", err
	}
	id := newID()
	err = s.inTx(ctx, func(tx *sql.Tx) error {
		_, err := tx.ExecContext(ctx, `
			INSERT INTO pot (id, ledger_id, name, kind, is_default, target_cents, target_date, monthly_rate_cents, is_separate_account, archived)
			VALUES (?, ?, ?, 'one_time', 0, ?, ?, ?, ?, 0)`,
			id, in.LedgerID, name, nullIfZero(in.TargetCents), nullIfEmpty(in.TargetDate), nullIfZero(in.MonthlyRateCents), in.SeparateAccount,
		)
		if err != nil {
			return nameError(err)
		}
		if in.RoundUpMultiplier == 0 {
			return nil
		}
		return updatePot(ctx, tx, in.LedgerID, id, PotUpdate{RoundUpMultiplier: &in.RoundUpMultiplier, Today: in.Today})
	})
	if err != nil {
		return "", err
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
		       COALESCE(p.target_cents, 0), COALESCE(p.target_date, ''), COALESCE(p.monthly_rate_cents, 0),
		       p.is_separate_account, COALESCE(p.round_up_multiplier, 0), COALESCE(p.round_up_from, ''),
		       COALESCE(p.reserve_for, ''), `+potBalanceSQL+`
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
			&p.TargetCents, &p.TargetDate, &p.MonthlyRateCents, &p.IsSeparateAccount, &p.RoundUpMultiplier, &p.RoundUpFrom, &p.ReserveFor, &p.BalanceCents); err != nil {
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

// UpdatePot renames, changes the goal, or archives a pot. The default pot
// can be renamed and given a target but never archived (rule T2); neither it
// nor a reserve saves a fixed rate (rule T7).
func (s *Store) UpdatePot(ctx context.Context, ledgerID, potID string, u PotUpdate) error {
	return s.inTx(ctx, func(tx *sql.Tx) error { return updatePot(ctx, tx, ledgerID, potID, u) })
}

// updatePot is UpdatePot inside a transaction, so CreatePot can use it too.
func updatePot(ctx context.Context, tx *sql.Tx, ledgerID, potID string, u PotUpdate) error {
	var name string
	var isDefault, archived, separate bool
	var target, rate, multiplier sql.NullInt64
	var date, reserveFor, roundUpFrom sql.NullString
	err := tx.QueryRowContext(ctx, `
		SELECT name, is_default, archived, target_cents, target_date, monthly_rate_cents, is_separate_account, reserve_for,
		       round_up_multiplier, round_up_from
		FROM pot WHERE id = ? AND ledger_id = ?`, potID, ledgerID,
	).Scan(&name, &isDefault, &archived, &target, &date, &rate, &separate, &reserveFor, &multiplier, &roundUpFrom)
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
	newRate := rate.Int64
	if u.MonthlyRateCents != nil {
		newRate = *u.MonthlyRateCents
	}
	if u.SeparateAccount != nil {
		separate = *u.SeparateAccount
	}
	if u.Archived != nil {
		archived = *u.Archived
	}

	if err := validateGoal(newTarget, newDate, newRate); err != nil {
		return err
	}
	if newRate > 0 && (isDefault || reserveFor.Valid) {
		return ErrGoalRateNotAllowed
	}
	if separate && (isDefault || reserveFor.Valid) {
		return ErrSeparateNotAllowed
	}
	roundUp, err := nextRoundUp(ctx, tx, ledgerID, potID, u, roundUpState{multiplier: int(multiplier.Int64), from: roundUpFrom.String}, isDefault || reserveFor.Valid, archived)
	if err != nil {
		return err
	}
	if isDefault && archived {
		return ErrDefaultPotProtected
	}
	if u.Archived != nil && *u.Archived {
		if err := ensureNotActiveReserve(ctx, tx, potID); err != nil {
			return err
		}
	}

	_, err = tx.ExecContext(ctx, `
		UPDATE pot SET name = ?, target_cents = ?, target_date = ?, monthly_rate_cents = ?, is_separate_account = ?,
		               round_up_multiplier = ?, round_up_from = ?, archived = ?
		WHERE id = ? AND ledger_id = ?`,
		name, nullIfZero(newTarget), nullIfEmpty(newDate), nullIfZero(newRate), separate,
		nullIfZero(int64(roundUp.multiplier)), nullIfEmpty(roundUp.from), archived, potID, ledgerID,
	)
	return nameError(err)
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
