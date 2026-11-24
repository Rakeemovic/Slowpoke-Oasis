package ledger

import (
	"context"
	"database/sql"
)

// CategoryLine compares the plan and the actual bookings of one category in a pot.
//
// ActualCents is the amount spent (expense categories) or received (income
// categories), always positive. DifferenceCents is what is left of the plan:
// planned minus spent for expenses, received minus planned for income.
type CategoryLine struct {
	CategoryID      string
	Name            string
	IsIncome        bool
	PlannedCents    int64
	ActualCents     int64
	DifferenceCents int64
}

// PotSummary is the plan-versus-actual view of one pot for a date range.
// An empty From or To means the range is not limited on that side. Month is
// set for monthly pots and when a one-time pot was asked for a month.
type PotSummary struct {
	PotID             string
	Month             string
	From              string
	To                string
	PlanSource        PlanSource
	Lines             []CategoryLine
	TransfersInCents  int64
	TransfersOutCents int64
}

// CategoryTotal is the sum of all bookings of one category in a ledger.
// TotalCents is always positive.
type CategoryTotal struct {
	CategoryID string
	Name       string
	IsIncome   bool
	Count      int64
	TotalCents int64
}

// SummaryQuery chooses the pot and the period of a pot summary.
type SummaryQuery struct {
	LedgerID string
	PotID    string
	Month    string // YYYY-MM; for monthly pots, empty means the current month
	From     string // YYYY-MM-DD, one-time pots only
	To       string // YYYY-MM-DD, one-time pots only
	Today    string // YYYY-MM-DD, the app's date; decides the current month
}

// PotSummary returns the planned and actual amounts of a pot per category, and
// the transfers into and out of the pot, for the given period.
//
// A monthly pot is always reported for one month. The plan of the current
// month is created from the templates on first access; a future month shows
// the template as a preview; a past month shows what was stored, if anything.
func (s *Store) PotSummary(ctx context.Context, q SummaryQuery) (PotSummary, error) {
	monthly, err := isMonthlyPot(ctx, s.db, q.LedgerID, q.PotID)
	if err != nil {
		return PotSummary{}, err
	}
	out := PotSummary{PotID: q.PotID, Month: q.Month, PlanSource: PlanOfPot}
	if monthly {
		err = s.resolveMonthlyPeriod(ctx, q, &out)
	} else {
		out.From, out.To, err = s.ResolvePeriod(ctx, q.LedgerID, q.Month, q.From, q.To)
	}
	if err != nil {
		return PotSummary{}, err
	}
	ledgerID, potID, from, to := q.LedgerID, q.PotID, out.From, out.To

	if out.Lines, err = categoryLines(ctx, s.db, lineQuery{
		LedgerID: ledgerID, PotID: potID, Month: out.Month, From: from, To: to, Source: out.PlanSource,
	}); err != nil {
		return PotSummary{}, err
	}

	rc, rargs := rangeClause(from, to)
	transferArgs := append([]any{potID, potID, ledgerID, potID, potID}, rargs...)
	var in, outgoing sql.NullInt64
	if err := s.db.QueryRowContext(ctx, `
		SELECT SUM(CASE WHEN type = 'transfer' AND counterpart_pot_id = ? THEN amount_cents
		                WHEN type = 'transfer_in' THEN amount_cents END),
		       SUM(CASE WHEN type = 'transfer' AND pot_id = ? THEN amount_cents
		                WHEN type = 'transfer_out' THEN -amount_cents END)
		FROM transactions
		WHERE ledger_id = ? AND deleted_at IS NULL
		  AND type IN ('transfer', 'transfer_in', 'transfer_out')
		  AND (pot_id = ? OR counterpart_pot_id = ?)`+rc,
		transferArgs...,
	).Scan(&in, &outgoing); err != nil {
		return PotSummary{}, err
	}
	out.TransfersInCents = in.Int64
	out.TransfersOutCents = outgoing.Int64
	return out, nil
}

// resolveMonthlyPeriod sets the month, the date range and the source of the
// plan of a monthly pot summary.
func (s *Store) resolveMonthlyPeriod(ctx context.Context, q SummaryQuery, out *PotSummary) error {
	if q.From != "" || q.To != "" {
		return ErrMonthlyPotNeedsMonth
	}
	p, err := s.planPeriod(ctx, q.LedgerID, q.Month, q.Today)
	if err != nil {
		return err
	}
	out.Month, out.From, out.To, out.PlanSource = p.Month, p.From, p.To, p.Source
	return nil
}

// planPeriod is one payday period of a monthly plan.
type planPeriod struct {
	Month   string // YYYY-MM, the month the period starts in
	Current string // the month of the period that contains today
	From    string
	To      string
	Source  PlanSource
}

// planPeriod resolves month (empty: the current month) to its payday period
// and says where its plan comes from. The plan of the current month is
// created from the templates on first access; a future month shows the
// template as a preview; a past month shows what was stored, if anything.
func (s *Store) planPeriod(ctx context.Context, ledgerID, month, today string) (planPeriod, error) {
	payday, err := ledgerPayday(ctx, s.db, ledgerID)
	if err != nil {
		return planPeriod{}, err
	}
	p := planPeriod{Month: month}
	if p.Current, err = MonthOf(today, payday); err != nil {
		return planPeriod{}, err
	}
	if p.Month == "" {
		p.Month = p.Current
	}
	if p.From, p.To, err = PeriodFor(p.Month, payday); err != nil {
		return planPeriod{}, err
	}

	switch {
	case p.Month > p.Current: // the format YYYY-MM sorts like time
		p.Source = PlanPreview
	case p.Month == p.Current:
		p.Source = PlanOfMonth
		err = s.inTx(ctx, func(tx *sql.Tx) error {
			return ensureMonthPlans(ctx, tx, ledgerID, p.Current)
		})
	default:
		var stored bool
		stored, err = monthPlanStored(ctx, s.db, ledgerID, p.Month)
		p.Source = PlanNotStored
		if stored {
			p.Source = PlanOfMonth
		}
	}
	return p, err
}

// lineQuery chooses the pot, period and plan of categoryLines.
type lineQuery struct {
	LedgerID string
	PotID    string
	Month    string
	From     string
	To       string
	Source   PlanSource
	// WithoutRecurring leaves out bookings of recurring payments, which the
	// budget shows in lines of their own.
	WithoutRecurring bool
}

// categoryLines compares the plan of a pot with its bookings per category.
// Categories with neither a plan nor bookings are left out.
func categoryLines(ctx context.Context, db queryer, q lineQuery) ([]CategoryLine, error) {
	rc, rargs := rangeClause(q.From, q.To)
	if q.WithoutRecurring {
		rc += " AND recurring_payment_id IS NULL"
	}
	planSQL, planArgs := planLinesQuery(q.Source, q.PotID, q.Month)

	args := append(planArgs, q.LedgerID, q.PotID)
	args = append(args, rargs...)
	args = append(args, q.LedgerID)
	rows, err := db.QueryContext(ctx, `
		SELECT c.id, c.name, c.is_income, COALESCE(pl.amount_cents, 0), COALESCE(a.total, 0)
		FROM category c
		LEFT JOIN (`+planSQL+`) pl ON pl.category_id = c.id
		LEFT JOIN (
			SELECT category_id, SUM(amount_cents) AS total
			FROM transactions
			WHERE ledger_id = ? AND pot_id = ? AND deleted_at IS NULL
			  AND type IN ('expense', 'income')`+rc+`
			GROUP BY category_id
		) a ON a.category_id = c.id
		WHERE c.ledger_id = ? AND (pl.amount_cents IS NOT NULL OR a.total IS NOT NULL)
		ORDER BY c.name`,
		args...,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []CategoryLine
	for rows.Next() {
		var l CategoryLine
		var net int64
		if err := rows.Scan(&l.CategoryID, &l.Name, &l.IsIncome, &l.PlannedCents, &net); err != nil {
			return nil, err
		}
		if l.IsIncome {
			l.ActualCents = net
			l.DifferenceCents = net - l.PlannedCents
		} else {
			l.ActualCents = -net
			l.DifferenceCents = l.PlannedCents - l.ActualCents
		}
		out = append(out, l)
	}
	return out, rows.Err()
}

// planLinesQuery returns a query for the planned amounts (category_id,
// amount_cents) of a pot from the given source, and its arguments.
func planLinesQuery(source PlanSource, potID, month string) (string, []any) {
	switch source {
	case PlanOfMonth:
		return `SELECT category_id, amount_cents FROM period_plan WHERE pot_id = ? AND month = ?`,
			[]any{potID, month}
	case PlanNotStored:
		return `SELECT category_id, amount_cents FROM period_plan WHERE 0`, nil
	default: // the plan of a one-time pot, or the template as a preview
		return `SELECT category_id, amount_cents FROM pot_line WHERE pot_id = ?`, []any{potID}
	}
}

// CategoryTotals returns the number and sum of bookings per category of a
// ledger for the given date range, across all pots.
func (s *Store) CategoryTotals(ctx context.Context, ledgerID, from, to string) ([]CategoryTotal, error) {
	rc, rargs := rangeClause(from, to)
	args := append([]any{ledgerID}, rargs...)
	args = append(args, ledgerID)

	rows, err := s.db.QueryContext(ctx, `
		SELECT c.id, c.name, c.is_income, COUNT(t.id), COALESCE(SUM(t.amount_cents), 0)
		FROM category c
		JOIN transactions t ON t.category_id = c.id AND t.ledger_id = c.ledger_id
		WHERE t.ledger_id = ? AND t.deleted_at IS NULL AND t.type IN ('expense', 'income')`+rc+`
		  AND c.ledger_id = ?
		GROUP BY c.id, c.name, c.is_income
		ORDER BY c.name`,
		args...,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []CategoryTotal
	for rows.Next() {
		var t CategoryTotal
		if err := rows.Scan(&t.CategoryID, &t.Name, &t.IsIncome, &t.Count, &t.TotalCents); err != nil {
			return nil, err
		}
		if !t.IsIncome {
			t.TotalCents = -t.TotalCents // expenses are stored negative
		}
		out = append(out, t)
	}
	return out, rows.Err()
}
