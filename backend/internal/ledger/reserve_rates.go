package ledger

import (
	"context"
	"sort"
	"time"
)

// ReserveRate is what a reserve should receive in the current payday period
// to hold the amount of its payment on the next due date (rule R14).
type ReserveRate struct {
	Payment RecurringPayment
	// DueOn is the occurrence the reserve saves for: the oldest one that is
	// neither booked nor skipped.
	DueOn string
	// Periods counts the payday periods until DueOn, the current one included.
	Periods      int
	BalanceCents int64
	// SavedCents is what transfers in this period moved into the reserve,
	// minus what they moved out, as far as it is still there: money saved and
	// spent on the occurrence in the same period does not count for the next.
	SavedCents int64
	// RateCents is the rate of this period: what is missing without this
	// period's savings, divided by Periods and rounded up to the cent.
	RateCents int64
	// OpenCents is the part of the rate not saved yet (never below zero).
	OpenCents int64
}

// ReserveRates returns the rates of the current payday period for the active
// reserves of a ledger, ordered by due date. today is the app's local date.
// The rate corrects itself: whatever was saved too little or too much changes
// the rates of the following periods.
func (s *Store) ReserveRates(ctx context.Context, ledgerID, today string) ([]ReserveRate, error) {
	cur, err := currentPeriod(ctx, s.db, ledgerID, today)
	if err != nil {
		return nil, err
	}
	payday, month, from, to := cur.Payday, cur.Month, cur.From, cur.To
	payments, err := queryRecurringPayments(ctx, s.db,
		`WHERE ledger_id = ? AND archived = 0 AND interval_months > 1`, ledgerID)
	if err != nil {
		return nil, err
	}
	closed, err := closedOccurrences(ctx, s.db, ledgerID)
	if err != nil {
		return nil, err
	}

	out := []ReserveRate{}
	for _, p := range payments {
		due := p.nextOpenOccurrence(closed, to)
		if due == "" || p.ReservePotID == "" {
			continue // the payment has ended
		}
		dueMonth, err := MonthOf(due, payday)
		if err != nil {
			return nil, err
		}
		r := ReserveRate{Payment: p, DueOn: due, Periods: periodsUntil(month, dueMonth)}
		var moved int64
		if r.BalanceCents, moved, err = potMovements(ctx, s.db, p.ReservePotID, from, to); err != nil {
			return nil, err
		}
		r.SavedCents = min(moved, r.BalanceCents)
		r.RateCents = reserveRate(p.AmountCents, r.BalanceCents-r.SavedCents, r.Periods)
		r.OpenCents = max(0, r.RateCents-r.SavedCents)
		out = append(out, r)
	}
	sort.SliceStable(out, func(i, j int) bool { return rateBefore(out[i], out[j]) })
	return out, nil
}

// period is the payday period that contains a day.
type period struct {
	Payday int
	Month  string // YYYY-MM, the month the period starts in
	From   string
	To     string
}

// currentPeriod returns the payday period of a ledger that contains today.
func currentPeriod(ctx context.Context, q queryer, ledgerID, today string) (period, error) {
	if !validDate(today) {
		return period{}, ErrInvalidDate
	}
	payday, err := ledgerPayday(ctx, q, ledgerID)
	if err != nil {
		return period{}, err
	}
	month, err := MonthOf(today, payday)
	if err != nil {
		return period{}, err
	}
	from, to, err := PeriodFor(month, payday)
	return period{Payday: payday, Month: month, From: from, To: to}, err
}

// nextOpenOccurrence returns the oldest due date since the start of tracking
// that is neither booked nor skipped, or "" if there is none. It looks one
// interval (plus a month) beyond periodEnd, where the next due date must lie.
func (p RecurringPayment) nextOpenOccurrence(closed map[occurrenceKey]bool, periodEnd string) string {
	for _, due := range p.DueDates(p.TrackedFrom, addMonths(periodEnd, p.IntervalMonths+1)) {
		if !closed[occurrenceKey{p.ID, due}] {
			return due
		}
	}
	return ""
}

// periodsUntil counts the payday periods from month to dueMonth (both
// YYYY-MM), both included. An overdue occurrence still leaves one period.
func periodsUntil(month, dueMonth string) int {
	a, _ := time.Parse(monthLayout, month)
	b, _ := time.Parse(monthLayout, dueMonth)
	return max(1, monthsBetween(a, b)+1)
}

// reserveRate divides what is missing to amount among the periods left,
// rounded up to the cent. A reserve that holds enough needs no rate.
func reserveRate(amount, basis int64, periods int) int64 {
	missing := amount - basis
	if missing <= 0 {
		return 0
	}
	n := int64(periods)
	return (missing + n - 1) / n
}

// potMovements returns the balance of a pot (a reserve or a savings goal)
// and the net amount moved into it by transfers booked from..to (inclusive).
func potMovements(ctx context.Context, q queryer, potID, from, to string) (balance, saved int64, err error) {
	err = q.QueryRowContext(ctx, `
		SELECT `+potBalanceSQL+`,
		  (SELECT COALESCE(SUM(CASE
		      WHEN t.type = 'transfer' AND t.counterpart_pot_id = p.id THEN t.amount_cents
		      WHEN t.type = 'transfer' THEN -t.amount_cents
		      ELSE t.amount_cents END), 0)
		   FROM transactions t
		   WHERE (t.pot_id = p.id OR t.counterpart_pot_id = p.id) AND t.deleted_at IS NULL
		     AND t.type IN ('transfer', 'transfer_in', 'transfer_out')
		     AND t.booked_on BETWEEN ? AND ?)
		FROM pot p WHERE p.id = ?`, from, to, potID,
	).Scan(&balance, &saved)
	return balance, saved, err
}

// rateBefore orders rates by due date, then by payment name.
func rateBefore(a, b ReserveRate) bool {
	if a.DueOn != b.DueOn {
		return a.DueOn < b.DueOn
	}
	return a.Payment.Name < b.Payment.Name
}
