package ledger

import (
	"context"
	"time"
)

// PurposeLine is one purpose of a PurposeReport. The purpose "" stands for
// expenses whose category has none.
type PurposeLine struct {
	Purpose Purpose
	// Cents is what the purpose took in the period, always positive. For
	// saving it includes TransferCents.
	Cents int64
	// SpontaneousCents is the part of Cents booked as spontaneous (rule X20).
	SpontaneousCents int64
	// TransferCents is the part of Cents that moved into savings pots; it is
	// only set for saving (rule L10).
	TransferCents int64
	// PreviousCents is Cents of the period before.
	PreviousCents int64
	// SharePercent is Cents as a share of the income of the period, rounded to
	// whole percent; nil when no income is booked.
	SharePercent *int
}

// PurposeReport splits what a ledger spent in one payday period by purpose
// (rule L10).
type PurposeReport struct {
	// Month is the month shown, Current the month that contains the app's date.
	Month         string
	Current       string
	PreviousMonth string
	From          string
	To            string
	// IncomeCents is the income booked in the period.
	IncomeCents int64
	// Lines has need, want, saving and the expenses without purpose, in this order.
	Lines []PurposeLine
}

// purposeOrder is the order of the lines of a report.
var purposeOrder = []Purpose{PurposeNeed, PurposeWant, PurposeSaving, ""}

// purposeTotals are the sums of one period.
type purposeTotals struct {
	cents         map[Purpose]int64
	spontaneous   map[Purpose]int64
	transferCents int64
	incomeCents   int64
}

// PurposeReport returns the report for a month (YYYY-MM, empty for the
// current one) of a ledger. today is the app's local date and decides the
// current month.
func (s *Store) PurposeReport(ctx context.Context, ledgerID, month, today string) (PurposeReport, error) {
	if !validDate(today) {
		return PurposeReport{}, ErrInvalidDate
	}
	payday, err := ledgerPayday(ctx, s.db, ledgerID)
	if err != nil {
		return PurposeReport{}, err
	}
	out := PurposeReport{Month: month}
	if out.Current, err = MonthOf(today, payday); err != nil {
		return PurposeReport{}, err
	}
	if out.Month == "" {
		out.Month = out.Current
	}
	if out.From, out.To, err = PeriodFor(out.Month, payday); err != nil {
		return PurposeReport{}, err
	}
	out.PreviousMonth = monthBefore(out.Month)
	prevFrom, prevTo, err := PeriodFor(out.PreviousMonth, payday)
	if err != nil {
		return PurposeReport{}, err
	}

	now, err := sumPurposes(ctx, s.db, purposeScope{ledgerID: ledgerID, days: DayRange{out.From, out.To}})
	if err != nil {
		return PurposeReport{}, err
	}
	before, err := sumPurposes(ctx, s.db, purposeScope{ledgerID: ledgerID, days: DayRange{prevFrom, prevTo}})
	if err != nil {
		return PurposeReport{}, err
	}
	out.IncomeCents = now.incomeCents
	out.Lines = purposeLines(now, before)
	if now.incomeCents > 0 {
		for i := range out.Lines {
			share := int((out.Lines[i].Cents*100 + now.incomeCents/2) / now.incomeCents)
			out.Lines[i].SharePercent = &share
		}
	}
	return out, nil
}

// purposeLines builds the lines of a period and compares each with an earlier one.
func purposeLines(now, before purposeTotals) []PurposeLine {
	lines := make([]PurposeLine, 0, len(purposeOrder))
	for _, p := range purposeOrder {
		line := PurposeLine{
			Purpose:          p,
			Cents:            now.cents[p],
			SpontaneousCents: now.spontaneous[p],
			PreviousCents:    before.cents[p],
		}
		if p == PurposeSaving {
			line.TransferCents = now.transferCents
			line.Cents += now.transferCents
			line.PreviousCents += before.transferCents
		}
		lines = append(lines, line)
	}
	return lines
}

// monthBefore returns the month before a valid month (YYYY-MM).
func monthBefore(month string) string {
	first, _ := time.Parse(monthLayout, month)
	return first.AddDate(0, -1, 0).Format(monthLayout)
}

// purposeScope selects the bookings to add up: a ledger and a span of days,
// and only those of one user if userID is set.
type purposeScope struct {
	ledgerID string
	userID   string
	days     DayRange
}

// bookingsInScope is the WHERE part that selects the bookings t of a
// purposeScope; scope.args returns its arguments.
const bookingsInScope = `t.ledger_id = ? AND (? = '' OR t.user_id = ?)
		  AND t.deleted_at IS NULL AND t.booked_on BETWEEN ? AND ?`

func (sc purposeScope) args() []any {
	return []any{sc.ledgerID, sc.userID, sc.userID, sc.days.From, sc.days.Until}
}

// sumPurposes adds up a scope: the expenses by the purpose of their
// category, the income, and the money that moved into savings pots.
func sumPurposes(ctx context.Context, q queryer, sc purposeScope) (purposeTotals, error) {
	out := purposeTotals{cents: map[Purpose]int64{}, spontaneous: map[Purpose]int64{}}

	// Expenses of every pot count on the day they are booked, whether they
	// were paid from the main pot or a reserve; they are stored negative.
	rows, err := q.QueryContext(ctx, `
		SELECT COALESCE(c.purpose, ''), -SUM(t.amount_cents),
		       -SUM(CASE WHEN t.is_spontaneous = 1 THEN t.amount_cents ELSE 0 END)
		FROM transactions t
		JOIN category c ON c.id = t.category_id AND c.ledger_id = t.ledger_id
		WHERE `+bookingsInScope+` AND t.type = 'expense'
		GROUP BY COALESCE(c.purpose, '')`, sc.args()...)
	if err != nil {
		return out, err
	}
	defer rows.Close()
	for rows.Next() {
		var purpose Purpose
		var cents, spontaneous int64
		if err := rows.Scan(&purpose, &cents, &spontaneous); err != nil {
			return out, err
		}
		out.cents[purpose], out.spontaneous[purpose] = cents, spontaneous
	}
	if err := rows.Err(); err != nil {
		return out, err
	}
	rows.Close() // the store has one connection: finish this query before the next

	if err := q.QueryRowContext(ctx, `
		SELECT COALESCE(SUM(t.amount_cents), 0) FROM transactions t
		WHERE `+bookingsInScope+` AND t.type = 'income'`, sc.args()...,
	).Scan(&out.incomeCents); err != nil {
		return out, err
	}

	// Money that moved into savings pots, net of what came back out. Savings
	// pots are all pots except the main pot and reserves (rule L10). A
	// transfer between two of them cancels out, so it is no saving.
	var net int64
	if err := q.QueryRowContext(ctx, `
		SELECT COALESCE(SUM(CASE WHEN tp.id IS NOT NULL THEN t.amount_cents ELSE 0 END), 0)
		     - COALESCE(SUM(CASE WHEN fp.id IS NOT NULL THEN t.amount_cents ELSE 0 END), 0)
		FROM transactions t
		LEFT JOIN pot tp ON tp.id = t.counterpart_pot_id AND tp.is_default = 0 AND tp.reserve_for IS NULL
		LEFT JOIN pot fp ON fp.id = t.pot_id AND fp.is_default = 0 AND fp.reserve_for IS NULL
		WHERE `+bookingsInScope+` AND t.type = 'transfer'`, sc.args()...,
	).Scan(&net); err != nil {
		return out, err
	}
	out.transferCents = max(net, 0)
	return out, nil
}
