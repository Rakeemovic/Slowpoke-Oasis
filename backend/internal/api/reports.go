package api

import (
	"net/http"

	"github.com/Rakeemovic/slowpoke-oasis/backend/internal/ledger"
)

// planLineResponse compares plan and actual for one category.
type planLineResponse struct {
	CategoryID      string `json:"category_id"`
	Name            string `json:"name"`
	IsIncome        bool   `json:"is_income"`
	PlannedCents    int64  `json:"planned_cents"`
	ActualCents     int64  `json:"actual_cents"`
	DifferenceCents int64  `json:"difference_cents"`
}

// GET /ledgers/{ledger}/pots/{pot}/summary?month=YYYY-MM&date=YYYY-MM-DD
// Monthly pots are reported by month; without month, the current month is
// used, which date (the app's local date) decides. One-time pots also take
// ?from=&to=, and with no parameter the summary covers all dates.
// plan_source says where the planned amounts come from: "pot" (one-time
// pot), "month" (stored plan of the month), "preview" (future month, from the
// template) or "none" (past month without a plan).
func (s *Server) potSummary(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.requireLedgerAccess(w, r); !ok {
		return
	}
	q := r.URL.Query()
	today, ok := s.today(w, r)
	if !ok {
		return
	}
	sum, err := s.store.PotSummary(r.Context(), ledger.SummaryQuery{
		LedgerID: r.PathValue("ledger"),
		PotID:    r.PathValue("pot"),
		Month:    q.Get("month"),
		From:     q.Get("from"),
		To:       q.Get("to"),
		Today:    today,
	})
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
		"month":               sum.Month,
		"plan_source":         sum.PlanSource,
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

// balanceResponse is the balance of a ledger on one day (rule L7).
type balanceResponse struct {
	AsOf           string `json:"as_of"`
	AvailableCents int64  `json:"available_cents"`
	TotalCents     int64  `json:"total_cents"`
}

// GET /ledgers/{ledger}/balance?date=YYYY-MM-DD
// The app sends its own "today", because the server may be in another time
// zone. Without date, the server's today is used.
func (s *Server) ledgerBalance(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.requireLedgerAccess(w, r); !ok {
		return
	}
	b, err := s.store.LedgerBalance(r.Context(), r.PathValue("ledger"), r.URL.Query().Get("date"), s.now())
	if err != nil {
		fail(w, err)
		return
	}
	writeJSON(w, http.StatusOK, balanceResponse{AsOf: b.AsOf, AvailableCents: b.AvailableCents, TotalCents: b.TotalCents})
}
