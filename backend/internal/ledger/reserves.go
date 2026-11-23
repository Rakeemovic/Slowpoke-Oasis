package ledger

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"unicode/utf8"
)

// Errors for reserves (rules R10–R14).
var (
	ErrReservePotFixed = errors.New("a payment due every few months books from its own reserve; its pot cannot be chosen")
	ErrPotIsReserve    = errors.New("this pot is the reserve of another recurring payment")
	ErrReserveInUse    = errors.New("this pot is the reserve of an active recurring payment and cannot be archived")
)

// maxNameLength is the longest name of a pot (rule T1, see cleanName).
const maxNameLength = 50

// savesInReserve reports whether a payment saves for its due dates in a
// reserve of its own (rule R10): it is active and due every few months.
func (p RecurringPayment) savesInReserve() bool {
	return !p.Archived && p.IntervalMonths > 1
}

// activeReserveCondition is the SQL condition that pot p is the active
// reserve of payment r (rule R10). Only then do the reserve rules apply;
// otherwise the pot behaves like any other pot.
const activeReserveCondition = `r.id = p.reserve_for AND p.archived = 0
	AND r.archived = 0 AND r.interval_months > 1`

// syncReserve makes the pot of p follow rule R10, before p is stored:
//   - a payment that saves in a reserve books from its active reserve, which
//     is created if there is none (and named after the payment) and whose
//     target follows the amount;
//   - a monthly payment that still pointed to its reserve books from the
//     default pot again. Its reserve stays as it is, so the app can offer to
//     move the rest (rule R14).
//
// An archived payment keeps its pot, so it finds its reserve again later.
func syncReserve(ctx context.Context, tx *sql.Tx, ledgerID string, p *RecurringPayment) error {
	reserveID, err := activeReserveOf(ctx, tx, p.ID)
	if err != nil {
		return err
	}
	switch {
	case p.savesInReserve() && reserveID == "":
		p.PotID, err = createReserve(ctx, tx, ledgerID, *p)
		return err
	case p.savesInReserve():
		p.PotID = reserveID
		_, err = tx.ExecContext(ctx,
			`UPDATE pot SET target_cents = ? WHERE id = ?`, p.AmountCents, reserveID)
		return err
	case !p.Archived && p.PotID == reserveID:
		p.PotID, err = resolvePot(ctx, tx, ledgerID, "")
		return err
	}
	return nil
}

// activeReserveOf returns the ID of the pot that is not archived and is
// linked to the payment as its reserve, or "" if there is none.
func activeReserveOf(ctx context.Context, q queryer, paymentID string) (string, error) {
	var id string
	err := q.QueryRowContext(ctx,
		`SELECT id FROM pot WHERE reserve_for = ? AND archived = 0`, paymentID,
	).Scan(&id)
	if errors.Is(err, sql.ErrNoRows) {
		return "", nil
	}
	return id, err
}

// createReserve adds a one-time pot as the reserve of p and returns its ID.
// Its target is the amount of the payment.
func createReserve(ctx context.Context, tx *sql.Tx, ledgerID string, p RecurringPayment) (string, error) {
	name, err := freeReserveName(ctx, tx, ledgerID, p.Name)
	if err != nil {
		return "", err
	}
	id := newID()
	_, err = tx.ExecContext(ctx, `
		INSERT INTO pot (id, ledger_id, name, kind, target_cents, reserve_for)
		VALUES (?, ?, ?, 'one_time', ?, ?)`,
		id, ledgerID, name, p.AmountCents, p.ID,
	)
	return id, err
}

// freeReserveName returns a name for a new reserve that no active pot of the
// ledger has (rule T1): the payment's name, else "<name> (reserve)", else
// "<name> (reserve 2)" and so on. The payment's name is shortened to fit.
func freeReserveName(ctx context.Context, q queryer, ledgerID, paymentName string) (string, error) {
	for n := 1; ; n++ {
		name := reserveName(paymentName, n)
		var taken bool
		err := q.QueryRowContext(ctx, `
			SELECT EXISTS (SELECT 1 FROM pot
			               WHERE ledger_id = ? AND archived = 0 AND lower(name) = lower(?))`,
			ledgerID, name,
		).Scan(&taken)
		if err != nil {
			return "", err
		}
		if !taken {
			return name, nil
		}
	}
}

// reserveName returns the n-th name to try for the reserve of a payment.
func reserveName(paymentName string, n int) string {
	suffix := ""
	switch {
	case n == 2:
		suffix = " (reserve)"
	case n > 2:
		suffix = fmt.Sprintf(" (reserve %d)", n-1)
	}
	return truncateRunes(paymentName, maxNameLength-utf8.RuneCountInString(suffix)) + suffix
}

// truncateRunes shortens s to at most n characters, without a trailing space.
func truncateRunes(s string, n int) string {
	if utf8.RuneCountInString(s) <= n {
		return s
	}
	return strings.TrimSpace(string([]rune(s)[:n]))
}

// ensurePotChoosable rejects a pot chosen for a monthly payment that is the
// reserve of another payment (rule R11).
func ensurePotChoosable(ctx context.Context, q queryer, ledgerID, potID, paymentID string) error {
	var reserveFor sql.NullString
	err := q.QueryRowContext(ctx,
		`SELECT reserve_for FROM pot WHERE id = ? AND ledger_id = ?`, potID, ledgerID,
	).Scan(&reserveFor)
	if errors.Is(err, sql.ErrNoRows) {
		return nil // left to the foreign key check
	}
	if err != nil {
		return err
	}
	if reserveFor.Valid && reserveFor.String != paymentID {
		return ErrPotIsReserve
	}
	return nil
}

// ensureNotActiveReserve rejects archiving a pot that is the active reserve of
// a payment (rule R10): the payment would lose the pot it books from.
func ensureNotActiveReserve(ctx context.Context, q queryer, potID string) error {
	var active bool
	err := q.QueryRowContext(ctx, `
		SELECT EXISTS (SELECT 1 FROM pot p JOIN recurring_payment r ON `+activeReserveCondition+`
		               WHERE p.id = ?)`, potID,
	).Scan(&active)
	if err != nil {
		return err
	}
	if active {
		return ErrReserveInUse
	}
	return nil
}
