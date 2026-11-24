package ledger

import (
	"context"
	"database/sql"
	"errors"
	"time"
)

// Errors for round-ups.
var ErrRoundUpTransaction = errors.New("a round-up belongs to its expense; change or delete the expense instead")

// roundUpState is the round-up setting of a pot: multiplier 0 means off.
type roundUpState struct {
	multiplier int
	from       string // YYYY-MM-DD, empty when off
}

// nextRoundUp works out the round-up setting a pot has after an update (rule
// T10). Without a change it stays as it was; archiving a pot switches it off.
// Switching it on moves the round-up from the pot that collected it before,
// because a ledger has only one collecting pot. The first day it applies to
// is u.Today; changing the multiplier of a pot that already collects keeps
// the day.
func nextRoundUp(ctx context.Context, tx *sql.Tx, ledgerID, potID string, u PotUpdate, current roundUpState, cannotCollect, archived bool) (roundUpState, error) {
	if u.RoundUpMultiplier != nil && !validMultiplier(*u.RoundUpMultiplier) {
		return roundUpState{}, ErrInvalidMultiplier
	}
	if archived {
		if u.RoundUpMultiplier != nil && *u.RoundUpMultiplier > 0 {
			return roundUpState{}, ErrPotArchived
		}
		return roundUpState{}, nil
	}
	if u.RoundUpMultiplier == nil {
		return current, nil
	}
	if *u.RoundUpMultiplier == 0 {
		return roundUpState{}, nil
	}

	if cannotCollect {
		return roundUpState{}, ErrRoundUpNotAllowed
	}
	var personal bool
	if err := tx.QueryRowContext(ctx, `SELECT household_id IS NULL FROM ledger WHERE id = ?`, ledgerID).Scan(&personal); err != nil {
		return roundUpState{}, err
	}
	if !personal {
		return roundUpState{}, ErrRoundUpNotAllowed
	}
	next := roundUpState{multiplier: *u.RoundUpMultiplier, from: current.from}
	if current.multiplier == 0 {
		if !validDate(u.Today) {
			return roundUpState{}, ErrInvalidDate
		}
		next.from = u.Today
	}
	_, err := tx.ExecContext(ctx, `
		UPDATE pot SET round_up_multiplier = NULL, round_up_from = NULL
		WHERE ledger_id = ? AND id <> ? AND round_up_multiplier IS NOT NULL`, ledgerID, potID)
	return next, err
}

func validMultiplier(m int) bool {
	return m == 0 || m == 1 || m == 2 || m == 3 || m == 5
}

// roundUpCents is what an expense of amountCents rounds up to the next whole
// euro, times the multiplier. A whole euro amount has nothing to round up.
func roundUpCents(amountCents int64, multiplier int) int64 {
	rest := amountCents % 100
	if rest == 0 {
		return 0
	}
	return (100 - rest) * int64(multiplier)
}

// syncRoundUp makes the round-up of an expense match the rules (rule X21):
// an expense of the main pot of a personal ledger, booked on or after the day
// the round-up was switched on and not paying a recurring payment, is rounded
// up to the next whole euro, times the multiplier, by a transfer into the
// collecting pot on the same day. The transfer belongs to the expense: it is
// created, changed or removed here whenever the expense changes. The expense
// must not be deleted.
func syncRoundUp(ctx context.Context, tx *sql.Tx, ledgerID, expenseID string, now time.Time) error {
	var (
		typ, potID, bookedOn, userID string
		amount                       int64
		recurring                    sql.NullString
	)
	err := tx.QueryRowContext(ctx, `
		SELECT type, pot_id, abs(amount_cents), booked_on, user_id, recurring_payment_id
		FROM transactions WHERE id = ? AND ledger_id = ?`, expenseID, ledgerID,
	).Scan(&typ, &potID, &amount, &bookedOn, &userID, &recurring)
	if err != nil {
		return err
	}

	target, cents, err := wantedRoundUp(ctx, tx, ledgerID, potID, TxType(typ), recurring.Valid, bookedOn, amount)
	if err != nil {
		return err
	}
	var existing string
	err = tx.QueryRowContext(ctx, `SELECT id FROM transactions WHERE round_up_of = ?`, expenseID).Scan(&existing)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return err
	}
	exists := err == nil
	err = nil // "no row" only means there is no round-up yet

	ts := timestamp(now)
	switch {
	case cents == 0 && exists:
		_, err = tx.ExecContext(ctx, `DELETE FROM transactions WHERE id = ?`, existing)
	case cents > 0 && exists:
		// The pot it goes into stays as it was booked.
		_, err = tx.ExecContext(ctx, `
			UPDATE transactions SET amount_cents = ?, booked_on = ?, updated_at = ? WHERE id = ?`,
			cents, bookedOn, ts, existing)
	case cents > 0:
		_, err = tx.ExecContext(ctx, `
			INSERT INTO transactions
				(id, ledger_id, user_id, type, pot_id, counterpart_pot_id, amount_cents, booked_on,
				 round_up_of, created_at, updated_at)
			VALUES (?, ?, ?, 'transfer', ?, ?, ?, ?, ?, ?, ?)`,
			newID(), ledgerID, userID, potID, target, cents, bookedOn, expenseID, ts, ts)
	}
	return err
}

// wantedRoundUp returns the pot a round-up of this booking goes into and its
// amount; 0 when the booking is not rounded up.
func wantedRoundUp(ctx context.Context, q queryer, ledgerID, potID string, typ TxType, paysRecurring bool, bookedOn string, amount int64) (string, int64, error) {
	if typ != TypeExpense || paysRecurring {
		return "", 0, nil
	}
	var target, from string
	var multiplier int
	var isMain bool
	err := q.QueryRowContext(ctx, `
		SELECT c.id, c.round_up_multiplier, c.round_up_from, m.is_default
		FROM pot c JOIN pot m ON m.id = ? AND m.ledger_id = c.ledger_id
		JOIN ledger l ON l.id = c.ledger_id
		WHERE c.ledger_id = ? AND c.round_up_multiplier IS NOT NULL AND c.archived = 0
		  AND l.household_id IS NULL`, potID, ledgerID,
	).Scan(&target, &multiplier, &from, &isMain)
	if errors.Is(err, sql.ErrNoRows) {
		return "", 0, nil // no pot collects round-ups
	}
	if err != nil || !isMain || bookedOn < from {
		return "", 0, err
	}
	return target, roundUpCents(amount, multiplier), nil
}

// ensureNotRoundUp rejects changes to a round-up itself (rule X21): it follows
// its expense.
func ensureNotRoundUp(ctx context.Context, q queryer, ledgerID, id string) error {
	var roundUp bool
	err := q.QueryRowContext(ctx, `SELECT round_up_of IS NOT NULL FROM transactions WHERE id = ? AND ledger_id = ?`, id, ledgerID).Scan(&roundUp)
	if errors.Is(err, sql.ErrNoRows) {
		return nil // the caller reports "not found"
	}
	if err == nil && roundUp {
		return ErrRoundUpTransaction
	}
	return err
}
