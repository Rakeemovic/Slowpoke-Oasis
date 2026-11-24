package ledger

import (
	"context"
	"slices"
	"sort"
)

// Budget is the plan-versus-actual view of the main pot of a ledger for one
// month (rule P7). Each category has its freely planned part and, below it,
// lines for the monthly recurring payments and the reserve rates of the
// category, which are never part of the plan (rule P8).
type Budget struct {
	PotID string
	// Month is the month shown, Current the month that contains the app's
	// date. Only the current month can be changed.
	Month      string
	Current    string
	From       string
	To         string
	PlanSource PlanSource
	Categories []BudgetCategory
	// ReserveDues are the payments of the month that are paid from their
	// reserve and so do not weigh on the month (rule P10).
	ReserveDues []ReserveDue
}

// BudgetCategory is one category of a budget. Its embedded line compares the
// plan with the bookings that belong to no recurring payment.
type BudgetCategory struct {
	CategoryLine
	Recurring []RecurringLine
	Reserves  []ReserveLine
}

// RecurringLine is one occurrence of a monthly recurring payment in the main
// pot (rule P9). Bookings count in the month of their due date, not of their
// booking date.
type RecurringLine struct {
	PaymentID       string
	Name            string
	DueOn           string
	PlannedCents    int64 // the amount of the payment; 0 when skipped or not due
	ActualCents     int64 // what was booked for this occurrence, positive
	DifferenceCents int64 // planned minus actual
	Skipped         bool
}

// ReserveLine is the rate a reserve should receive in the month (rule P10).
type ReserveLine struct {
	PaymentID string
	Name      string
	PotID     string
	DueOn     string // the due date the rate saves for; "" for past months
	// PlannedCents is the rate, or nil for past months, whose rates were never
	// stored. For future months it is a preview.
	PlannedCents *int64
	ActualCents  int64 // net transfers into the reserve in the month
	Preview      bool
}

// ReserveDue is an occurrence of a payment that is paid from its reserve.
type ReserveDue struct {
	PaymentID string
	Name      string
	PotID     string
	DueOn     string
	// AmountCents is the amount of the payment; CoveredCents the part the
	// reserve holds today. Booked occurrences are covered by definition.
	AmountCents  int64
	CoveredCents int64
	Booked       bool
}

// BudgetQuery chooses the ledger and month of a budget.
type BudgetQuery struct {
	LedgerID string
	Month    string // YYYY-MM; empty means the current month
	Today    string // YYYY-MM-DD, the app's date; decides the current month
}

// Budget returns the budget of the main pot of a ledger for a month.
func (s *Store) Budget(ctx context.Context, q BudgetQuery) (Budget, error) {
	if !validDate(q.Today) {
		return Budget{}, ErrInvalidDate
	}
	potID, err := resolvePot(ctx, s.db, q.LedgerID, "")
	if err != nil {
		return Budget{}, err
	}
	p, err := s.planPeriod(ctx, q.LedgerID, q.Month, q.Today)
	if err != nil {
		return Budget{}, err
	}
	out := Budget{PotID: potID, Month: p.Month, Current: p.Current, From: p.From, To: p.To, PlanSource: p.Source}

	lines, err := categoryLines(ctx, s.db, lineQuery{
		LedgerID: q.LedgerID, PotID: potID, Month: p.Month, From: p.From, To: p.To,
		Source: p.Source, WithoutRecurring: true,
	})
	if err != nil {
		return Budget{}, err
	}
	payments, err := queryRecurringPayments(ctx, s.db, `WHERE ledger_id = ?`, q.LedgerID)
	if err != nil {
		return Budget{}, err
	}
	recurring, err := s.recurringLines(ctx, q.LedgerID, potID, p, payments)
	if err != nil {
		return Budget{}, err
	}
	reserves, err := s.reserveLines(ctx, q.LedgerID, q.Today, p, payments)
	if err != nil {
		return Budget{}, err
	}
	if out.ReserveDues, err = s.reserveDues(ctx, q.LedgerID, p, payments); err != nil {
		return Budget{}, err
	}
	out.Categories, err = s.groupByCategory(ctx, q.LedgerID, lines, recurring, reserves)
	return out, err
}

// categorized is a line of a budget together with the category it belongs to.
type categorized[T any] struct {
	CategoryID string
	Line       T
}

// recurringLines returns the occurrences of monthly payments of the main pot
// that are due in the period, and every occurrence of the period that has
// bookings in the main pot, whatever the payment is now.
func (s *Store) recurringLines(ctx context.Context, ledgerID, potID string, p planPeriod, payments []RecurringPayment) ([]categorized[RecurringLine], error) {
	booked, err := bookedOccurrences(ctx, s.db, ledgerID, potID, p.From, p.To)
	if err != nil {
		return nil, err
	}
	skipped, err := skippedOccurrences(ctx, s.db, ledgerID, p.From, p.To)
	if err != nil {
		return nil, err
	}

	var out []categorized[RecurringLine]
	for _, pay := range payments {
		var scheduled []string
		if pay.IntervalMonths == 1 && pay.PotID == potID && !pay.Archived {
			scheduled = pay.DueDates(max(p.From, pay.TrackedFrom), p.To)
		}
		dues := slices.Clone(scheduled)
		for due := range booked[pay.ID] {
			if !slices.Contains(dues, due) {
				dues = append(dues, due)
			}
		}
		sort.Strings(dues)
		for _, due := range dues {
			l := RecurringLine{
				PaymentID: pay.ID, Name: pay.Name, DueOn: due,
				ActualCents: booked[pay.ID][due], Skipped: skipped[occurrenceKey{pay.ID, due}],
			}
			if !l.Skipped && slices.Contains(scheduled, due) {
				l.PlannedCents = pay.AmountCents
			}
			l.DifferenceCents = l.PlannedCents - l.ActualCents
			out = append(out, categorized[RecurringLine]{pay.CategoryID, l})
		}
	}
	return out, nil
}

// bookedOccurrences sums the bookings of recurring payments in a pot per
// payment and due date, for due dates from..to. The sums are positive.
func bookedOccurrences(ctx context.Context, q queryer, ledgerID, potID, from, to string) (map[string]map[string]int64, error) {
	rows, err := q.QueryContext(ctx, `
		SELECT recurring_payment_id, recurring_due_on, -SUM(amount_cents)
		FROM transactions
		WHERE ledger_id = ? AND pot_id = ? AND deleted_at IS NULL
		  AND recurring_payment_id IS NOT NULL AND recurring_due_on BETWEEN ? AND ?
		GROUP BY recurring_payment_id, recurring_due_on`,
		ledgerID, potID, from, to,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := map[string]map[string]int64{}
	for rows.Next() {
		var id, due string
		var cents int64
		if err := rows.Scan(&id, &due, &cents); err != nil {
			return nil, err
		}
		if out[id] == nil {
			out[id] = map[string]int64{}
		}
		out[id][due] = cents
	}
	return out, rows.Err()
}

// skippedOccurrences returns the skipped occurrences of a ledger's payments
// with due dates from..to.
func skippedOccurrences(ctx context.Context, q queryer, ledgerID, from, to string) (map[occurrenceKey]bool, error) {
	rows, err := q.QueryContext(ctx, `
		SELECT s.recurring_payment_id, s.due_on FROM recurring_skip s
		JOIN recurring_payment p ON p.id = s.recurring_payment_id
		WHERE p.ledger_id = ? AND s.due_on BETWEEN ? AND ?`,
		ledgerID, from, to,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := map[occurrenceKey]bool{}
	for rows.Next() {
		var k occurrenceKey
		if err := rows.Scan(&k.paymentID, &k.dueOn); err != nil {
			return nil, err
		}
		out[k] = true
	}
	return out, rows.Err()
}

// reserveLines returns the reserve rates of the month (rule P10): the real
// rates for the current month, a preview for future months, and for past
// months only what was transferred, as their rates were never stored.
func (s *Store) reserveLines(ctx context.Context, ledgerID, today string, p planPeriod, payments []RecurringPayment) ([]categorized[ReserveLine], error) {
	switch {
	case p.Month == p.Current:
		return s.currentReserveLines(ctx, ledgerID, today)
	case p.Month > p.Current:
		return s.previewReserveLines(ctx, ledgerID, today, p)
	default:
		return s.pastReserveLines(ctx, p, payments)
	}
}

func (s *Store) currentReserveLines(ctx context.Context, ledgerID, today string) ([]categorized[ReserveLine], error) {
	rates, err := s.ReserveRates(ctx, ledgerID, today)
	if err != nil {
		return nil, err
	}
	out := make([]categorized[ReserveLine], 0, len(rates))
	for _, r := range rates {
		rate := r.RateCents
		out = append(out, categorized[ReserveLine]{r.Payment.CategoryID, ReserveLine{
			PaymentID: r.Payment.ID, Name: r.Payment.Name, PotID: r.Payment.ReservePotID,
			DueOn: r.DueOn, PlannedCents: &rate, ActualCents: r.SavedCents,
		}})
	}
	return out, nil
}

// previewReserveLines assumes that the current rates are saved as planned:
// until the due date the current rate saves for, the rate stays the same;
// after it, the reserve starts from nothing and spreads the amount over the
// interval of the payment.
func (s *Store) previewReserveLines(ctx context.Context, ledgerID, today string, p planPeriod) ([]categorized[ReserveLine], error) {
	rates, err := s.ReserveRates(ctx, ledgerID, today)
	if err != nil {
		return nil, err
	}
	payday, err := ledgerPayday(ctx, s.db, ledgerID)
	if err != nil {
		return nil, err
	}

	var out []categorized[ReserveLine]
	for _, r := range rates {
		pay := r.Payment
		due, rate := r.DueOn, r.RateCents
		dueMonth, err := MonthOf(due, payday)
		if err != nil {
			return nil, err
		}
		if dueMonth < p.Month {
			// A later cycle: the first due date in or after the month.
			dates := pay.DueDates(p.From, addMonths(p.To, pay.IntervalMonths+1))
			if len(dates) == 0 {
				continue // the payment has ended
			}
			due = dates[0]
			rate = reserveRate(pay.AmountCents, 0, pay.IntervalMonths)
		}
		out = append(out, categorized[ReserveLine]{pay.CategoryID, ReserveLine{
			PaymentID: pay.ID, Name: pay.Name, PotID: pay.ReservePotID,
			DueOn: due, PlannedCents: &rate, Preview: true,
		}})
	}
	return out, nil
}

func (s *Store) pastReserveLines(ctx context.Context, p planPeriod, payments []RecurringPayment) ([]categorized[ReserveLine], error) {
	var out []categorized[ReserveLine]
	for _, pay := range payments {
		if pay.ReservePotID == "" {
			continue
		}
		_, saved, err := reserveMovements(ctx, s.db, pay.ReservePotID, p.From, p.To)
		if err != nil {
			return nil, err
		}
		if saved == 0 {
			continue
		}
		out = append(out, categorized[ReserveLine]{pay.CategoryID, ReserveLine{
			PaymentID: pay.ID, Name: pay.Name, PotID: pay.ReservePotID, ActualCents: saved,
		}})
	}
	return out, nil
}

// reserveDues returns the occurrences of the month that are paid from a
// reserve, skipped ones left out (rule P10).
func (s *Store) reserveDues(ctx context.Context, ledgerID string, p planPeriod, payments []RecurringPayment) ([]ReserveDue, error) {
	skipped, err := skippedOccurrences(ctx, s.db, ledgerID, p.From, p.To)
	if err != nil {
		return nil, err
	}
	out := []ReserveDue{}
	for _, pay := range payments {
		if pay.Archived || pay.IntervalMonths == 1 || pay.ReservePotID == "" {
			continue
		}
		for _, due := range pay.DueDates(max(p.From, pay.TrackedFrom), p.To) {
			if skipped[occurrenceKey{pay.ID, due}] {
				continue
			}
			d := ReserveDue{
				PaymentID: pay.ID, Name: pay.Name, PotID: pay.ReservePotID,
				DueOn: due, AmountCents: pay.AmountCents,
			}
			if d.Booked, err = occurrenceBooked(ctx, s.db, pay.ID, due); err != nil {
				return nil, err
			}
			d.CoveredCents = pay.AmountCents
			if !d.Booked {
				balance, _, err := reserveMovements(ctx, s.db, pay.ReservePotID, p.From, p.To)
				if err != nil {
					return nil, err
				}
				d.CoveredCents = min(pay.AmountCents, max(0, balance))
			}
			out = append(out, d)
		}
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].DueOn < out[j].DueOn })
	return out, nil
}

// groupByCategory puts the recurring and reserve lines under their
// categories, ordered by name. A category appears if it has any line.
func (s *Store) groupByCategory(ctx context.Context, ledgerID string, lines []CategoryLine,
	recurring []categorized[RecurringLine], reserves []categorized[ReserveLine],
) ([]BudgetCategory, error) {
	names, err := categoryNames(ctx, s.db, ledgerID)
	if err != nil {
		return nil, err
	}
	byID := map[string]*BudgetCategory{}
	get := func(id string) *BudgetCategory {
		if c, ok := byID[id]; ok {
			return c
		}
		c := &BudgetCategory{CategoryLine: names[id]}
		byID[id] = c
		return c
	}
	for _, l := range lines {
		get(l.CategoryID).CategoryLine = l
	}
	for _, r := range recurring {
		c := get(r.CategoryID)
		c.Recurring = append(c.Recurring, r.Line)
	}
	for _, r := range reserves {
		c := get(r.CategoryID)
		c.Reserves = append(c.Reserves, r.Line)
	}

	out := make([]BudgetCategory, 0, len(byID))
	for _, c := range byID {
		out = append(out, *c)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Name != out[j].Name {
			return out[i].Name < out[j].Name
		}
		return out[i].CategoryID < out[j].CategoryID
	})
	return out, nil
}

// categoryNames returns an empty line (id, name, kind) for every category of
// a ledger, keyed by id.
func categoryNames(ctx context.Context, q queryer, ledgerID string) (map[string]CategoryLine, error) {
	rows, err := q.QueryContext(ctx, `SELECT id, name, is_income FROM category WHERE ledger_id = ?`, ledgerID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := map[string]CategoryLine{}
	for rows.Next() {
		var l CategoryLine
		if err := rows.Scan(&l.CategoryID, &l.Name, &l.IsIncome); err != nil {
			return nil, err
		}
		out[l.CategoryID] = l
	}
	return out, rows.Err()
}
