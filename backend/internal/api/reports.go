package api

import "net/http"

// planLineResponse compares plan and actual for one category.
type planLineResponse struct {
	CategoryID      string `json:"category_id"`
	Name            string `json:"name"`
	IsIncome        bool   `json:"is_income"`
	PlannedCents    int64  `json:"planned_cents"`
	ActualCents     int64  `json:"actual_cents"`
	DifferenceCents int64  `json:"difference_cents"`
}

// GET /ledgers/{ledger}/pots/{pot}/summary?month=YYYY-MM or ?from=&to=
// With neither parameter, the summary covers all dates.
func (s *Server) potSummary(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.requireLedgerAccess(w, r); !ok {
		return
	}
	ledgerID := r.PathValue("ledger")
	q := r.URL.Query()
	from, to, err := s.store.ResolvePeriod(r.Context(), ledgerID, q.Get("month"), q.Get("from"), q.Get("to"))
	if err != nil {
		fail(w, err)
		return
	}
	sum, err := s.store.PotSummary(r.Context(), ledgerID, r.PathValue("pot"), from, to)
	if err != nil {
		fail(w, err)
		return
	}

	lines := make([]planLineResponse, 0, len(sum.Lines))
	for _, l := range sum.Lines {
		lines = append(lines, planLineResponse{
			CategoryID:      l.CategoryID,
			Name:            l.Name,
			IsIncome:        l.IsIncome,
			PlannedCents:    l.PlannedCents,
			ActualCents:     l.ActualCents,
			DifferenceCents: l.DifferenceCents,
		})
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"pot_id":              sum.PotID,
		"from":                sum.From,
		"to":                  sum.To,
		"lines":               lines,
		"transfers_in_cents":  sum.TransfersInCents,
		"transfers_out_cents": sum.TransfersOutCents,
	})
}

// categoryTotalResponse is the sum of bookings of one category.
type categoryTotalResponse struct {
	CategoryID string `json:"category_id"`
	Name       string `json:"name"`
	IsIncome   bool   `json:"is_income"`
	Count      int64  `json:"count"`
	TotalCents int64  `json:"total_cents"`
}

// GET /ledgers/{ledger}/analytics?month=YYYY-MM or ?from=&to=
// With neither parameter, the report covers all dates.
func (s *Server) analytics(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.requireLedgerAccess(w, r); !ok {
		return
	}
	ledgerID := r.PathValue("ledger")
	q := r.URL.Query()
	from, to, err := s.store.ResolvePeriod(r.Context(), ledgerID, q.Get("month"), q.Get("from"), q.Get("to"))
	if err != nil {
		fail(w, err)
		return
	}
	totals, err := s.store.CategoryTotals(r.Context(), ledgerID, from, to)
	if err != nil {
		fail(w, err)
		return
	}

	var income, expenses int64
	categories := make([]categoryTotalResponse, 0, len(totals))
	for _, t := range totals {
		if t.IsIncome {
			income += t.TotalCents
		} else {
			expenses += t.TotalCents
		}
		categories = append(categories, categoryTotalResponse{
			CategoryID: t.CategoryID,
			Name:       t.Name,
			IsIncome:   t.IsIncome,
			Count:      t.Count,
			TotalCents: t.TotalCents,
		})
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"from":          from,
		"to":            to,
		"income_cents":  income,
		"expense_cents": expenses,
		"net_cents":     income - expenses,
		"categories":    categories,
	})
}
