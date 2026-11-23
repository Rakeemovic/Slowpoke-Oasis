package ledger

import (
	"context"
	"database/sql"
	"errors"
	"sort"
)

// Errors for the occurrences of recurring payments.
var (
	ErrNotAnOccurrence       = errors.New("this date is not a due date of the recurring payment")
	ErrOccurrenceBooked      = errors.New("this occurrence of the recurring payment is already booked")
	ErrOccurrenceSkipped     = errors.New("this occurrence is skipped; undo the skip to book it")
	ErrOccurrenceNotSkipped  = errors.New("this occurrence is not skipped")
	ErrRecurringLinkRequired = errors.New("recurring_payment_id and recurring_due_on must be given together")
)

// OccurrenceStatus says how an open occurrence relates to today.
type OccurrenceStatus string

const (
	StatusOverdue  OccurrenceStatus = "overdue"
	StatusDue      OccurrenceStatus = "due"
	StatusUpcoming OccurrenceStatus = "upcoming"
)

// Occurrence is one open due date of a recurring payment: neither booked nor
// skipped.
type Occurrence struct {
	Payment RecurringPayment
	DueOn   string
	Status  OccurrenceStatus
}

// OpenOccurrences returns the open occurrences of the active recurring
// payments of a ledger, from the start of tracking (rule R5) to the end of the
// current payday period, oldest first. today is the app's local date.
func (s *Store) OpenOccurrences(ctx context.Context, ledgerID, today string) ([]Occurrence, error) {
	if !validDate(today) {
		return nil, ErrInvalidDate
	}
	periodEnd, err := currentPeriodEnd(ctx, s.db, ledgerID, today)
	if err != nil {
		return nil, err
	}
	payments, err := queryRecurringPayments(ctx, s.db, `WHERE ledger_id = ? AND archived = 0`, ledgerID)
	if err != nil {
		return nil, err
	}
	closed, err := closedOccurrences(ctx, s.db, ledgerID)
	if err != nil {
		return nil, err
	}

	out := []Occurrence{}
	for _, p := range payments {
		for _, due := range p.DueDates(p.TrackedFrom, periodEnd) {
			if !closed[occurrenceKey{p.ID, due}] {
				out = append(out, Occurrence{Payment: p, DueOn: due, Status: statusOf(due, today)})
			}
		}
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].DueOn != out[j].DueOn {
			return out[i].DueOn < out[j].DueOn
		}
		return out[i].Payment.Name < out[j].Payment.Name
	})
	return out, nil
}

// currentPeriodEnd returns the last day of the payday period that contains today.
func currentPeriodEnd(ctx context.Context, q queryer, ledgerID, today string) (string, error) {
	payday, err := ledgerPayday(ctx, q, ledgerID)
	if err != nil {
		return "", err
	}
	month, err := MonthOf(today, payday)
	if err != nil {
		return "", err
	}
	_, end, err := PeriodFor(month, payday)
	return end, err
}

// statusOf compares a due date with today.
func statusOf(due, today string) OccurrenceStatus {
	switch {
	case due < today:
		return StatusOverdue
	case due == today:
		return StatusDue
	}
	return StatusUpcoming
}

// occurrenceKey names one occurrence of a recurring payment.
type occurrenceKey struct {
	paymentID string
	dueOn     string
}

// closedOccurrences returns the occurrences of a ledger's recurring payments
// that are booked (by a booking that is not deleted) or skipped.
func closedOccurrences(ctx context.Context, q queryer, ledgerID string) (map[occurrenceKey]bool, error) {
	rows, err := q.QueryContext(ctx, `
		SELECT recurring_payment_id, recurring_due_on FROM transactions
		WHERE ledger_id = ? AND recurring_payment_id IS NOT NULL AND deleted_at IS NULL
		UNION
		SELECT s.recurring_payment_id, s.due_on FROM recurring_skip s
		JOIN recurring_payment p ON p.id = s.recurring_payment_id
		WHERE p.ledger_id = ?`,
		ledgerID, ledgerID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	closed := map[occurrenceKey]bool{}
	for rows.Next() {
		var k occurrenceKey
		if err := rows.Scan(&k.paymentID, &k.dueOn); err != nil {
			return nil, err
		}
		closed[k] = true
	}
	return closed, rows.Err()
}

// NextDueOn returns the first due date on or after today, or "" if the
// payment has ended. Booked or skipped occurrences count too: it says when
// the payment is due next, not what is still open.
func (p RecurringPayment) NextDueOn(today string) string {
	if p.LastMonth != "" && today[:7] > p.LastMonth {
		return ""
	}
	// The next due date lies within one interval plus one month from the
	// later of today and the start of the payment.
	from := max(today, p.FirstMonth+"-01")
	if dates := p.DueDates(from, addMonths(from, p.IntervalMonths+1)); len(dates) > 0 {
		return dates[0]
	}
	return ""
}

// SkipOccurrence marks an occurrence as not to be booked (rule R7). Every
// member may skip; skipping twice is fine.
func (s *Store) SkipOccurrence(ctx context.Context, ledgerID, paymentID, dueOn string) error {
	return s.inTx(ctx, func(tx *sql.Tx) error {
		if err := ensureBookableOccurrence(ctx, tx, ledgerID, paymentID, dueOn); err != nil {
			return err
		}
		booked, err := occurrenceBooked(ctx, tx, paymentID, dueOn)
		if err != nil {
			return err
		}
		if booked {
			return ErrOccurrenceBooked
		}
		_, err = tx.ExecContext(ctx,
			`INSERT OR IGNORE INTO recurring_skip (recurring_payment_id, due_on) VALUES (?, ?)`,
			paymentID, dueOn)
		return err
	})
}

// UndoSkip opens a skipped occurrence again (rule R7). Every member may do it.
func (s *Store) UndoSkip(ctx context.Context, ledgerID, paymentID, dueOn string) error {
	res, err := s.db.ExecContext(ctx, `
		DELETE FROM recurring_skip
		WHERE recurring_payment_id = ? AND due_on = ?
		  AND recurring_payment_id IN (SELECT id FROM recurring_payment WHERE ledger_id = ?)`,
		paymentID, dueOn, ledgerID,
	)
	if err != nil {
		return err
	}
	return requireAffected(res, ErrOccurrenceNotSkipped)
}

// ensureBookableOccurrence checks that dueOn is a due date of an active
// recurring payment of the ledger (rule R6). Whether it is already booked is
// left to the caller.
func ensureBookableOccurrence(ctx context.Context, q queryer, ledgerID, paymentID, dueOn string) error {
	if !validDate(dueOn) {
		return ErrInvalidDate
	}
	p, err := loadRecurringPayment(ctx, q, ledgerID, paymentID)
	if err != nil {
		return err
	}
	if p.Archived {
		return ErrRecurringArchived
	}
	if !p.IsDueOn(dueOn) {
		return ErrNotAnOccurrence
	}
	return nil
}

// ensureOccurrenceOpen checks that a new booking may pay the occurrence: it
// belongs to an active payment of the ledger (rule R6), is not skipped (rule
// R7) and not booked yet (rule R6). The unique index on transactions still
// guards against two bookings that arrive at the same time.
func ensureOccurrenceOpen(ctx context.Context, q queryer, ledgerID, paymentID, dueOn string) error {
	if err := ensureBookableOccurrence(ctx, q, ledgerID, paymentID, dueOn); err != nil {
		return err
	}
	var skipped, booked bool
	err := q.QueryRowContext(ctx, `
		SELECT EXISTS (SELECT 1 FROM recurring_skip WHERE recurring_payment_id = ? AND due_on = ?),
		       EXISTS (SELECT 1 FROM transactions
		               WHERE recurring_payment_id = ? AND recurring_due_on = ? AND deleted_at IS NULL)`,
		paymentID, dueOn, paymentID, dueOn,
	).Scan(&skipped, &booked)
	switch {
	case err != nil:
		return err
	case skipped:
		return ErrOccurrenceSkipped
	case booked:
		return ErrOccurrenceBooked
	}
	return nil
}

// occurrenceBooked reports whether a booking that is not deleted pays the occurrence.
func occurrenceBooked(ctx context.Context, q queryer, paymentID, dueOn string) (bool, error) {
	var booked bool
	err := q.QueryRowContext(ctx, `
		SELECT EXISTS (SELECT 1 FROM transactions
		               WHERE recurring_payment_id = ? AND recurring_due_on = ? AND deleted_at IS NULL)`,
		paymentID, dueOn,
	).Scan(&booked)
	return booked, err
}
