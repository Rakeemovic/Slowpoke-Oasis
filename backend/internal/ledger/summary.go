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
// An empty From or To means the range is not limited on that side.
type PotSummary struct {
	PotID             string
	From              string
	To                string
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

// rangeClause adds an inclusive booked_on range to a query.
func rangeClause(from, to string) (string, []any) {
	clause := ""
	var args []any
	if from != "" {
		clause += ` AND booked_on >= ?`
		args = append(args, from)
	}
	if to != "" {
		clause += ` AND booked_on <= ?`
		args = append(args, to)
	}
	return clause, args
}

// PotSummary returns the planned and actual amounts of a pot per category, and
// the transfers into and out of the pot, for the given date range.
func (s *Store) PotSummary(ctx context.Context, ledgerID, potID, from, to string) (PotSummary, error) {
	out := PotSummary{PotID: potID, From: from, To: to}

	var one int
	if err := s.db.QueryRowContext(ctx,
		`SELECT 1 FROM pot WHERE id = ? AND ledger_id = ?`, potID, ledgerID,
	).Scan(&one); err != nil {
		return PotSummary{}, err // sql.ErrNoRows maps to 404
	}

	rc, rargs := rangeClause(from, to)

	lineArgs := append([]any{potID, ledgerID, potID}, rargs...)
	lineArgs = append(lineArgs, ledgerID)
	rows, err := s.db.QueryContext(ctx, `
		SELECT c.id, c.name, c.is_income, COALESCE(pl.amount_cents, 0), COALESCE(a.total, 0)
		FROM category c
		LEFT JOIN pot_line pl ON pl.pot_id = ? AND pl.category_id = c.id
		LEFT JOIN (
			SELECT category_id, SUM(amount_cents) AS total
			FROM transactions
			WHERE ledger_id = ? AND pot_id = ? AND deleted_at IS NULL
			  AND type IN ('expense', 'income')`+rc+`
			GROUP BY category_id
		) a ON a.category_id = c.id
		WHERE c.ledger_id = ? AND (pl.amount_cents IS NOT NULL OR a.total IS NOT NULL)
		ORDER BY c.name`,
		lineArgs...,
	)
	if err != nil {
		return PotSummary{}, err
	}
	defer rows.Close()

	for rows.Next() {
		var l CategoryLine
		var income int
		var net int64
		if err := rows.Scan(&l.CategoryID, &l.Name, &income, &l.PlannedCents, &net); err != nil {
			return PotSummary{}, err
		}
		l.IsIncome = income == 1
		if l.IsIncome {
			l.ActualCents = net
			l.DifferenceCents = net - l.PlannedCents
		} else {
			l.ActualCents = -net
			l.DifferenceCents = l.PlannedCents - l.ActualCents
		}
		out.Lines = append(out.Lines, l)
	}
	if err := rows.Err(); err != nil {
		return PotSummary{}, err
	}
	rows.Close()

	transferArgs := append([]any{potID, potID, ledgerID, potID, potID}, rargs...)
	var in, outgoing sql.NullInt64
	if err := s.db.QueryRowContext(ctx, `
		SELECT SUM(CASE WHEN counterpart_pot_id = ? THEN amount_cents END),
		       SUM(CASE WHEN pot_id = ? THEN amount_cents END)
		FROM transactions
		WHERE ledger_id = ? AND type = 'transfer' AND deleted_at IS NULL
		  AND (pot_id = ? OR counterpart_pot_id = ?)`+rc,
		transferArgs...,
	).Scan(&in, &outgoing); err != nil {
		return PotSummary{}, err
	}
	out.TransfersInCents = in.Int64
	out.TransfersOutCents = outgoing.Int64
	return out, nil
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
		var income int
		var sum int64
		if err := rows.Scan(&t.CategoryID, &t.Name, &income, &t.Count, &sum); err != nil {
			return nil, err
		}
		t.IsIncome = income == 1
		if !t.IsIncome {
			sum = -sum
		}
		t.TotalCents = sum
		out = append(out, t)
	}
	return out, rows.Err()
}
