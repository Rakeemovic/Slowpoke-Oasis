package api

import (
	"net/http"

	"github.com/Rakeemovic/slowpoke-oasis/backend/internal/ledger"
)

// budgetCategoryResponse is one category of the budget: the freely planned
// part, and lines for recurring payments and reserve rates below it.
type budgetCategoryResponse struct {
	planLineResponse
	Recurring []recurringLineResponse `json:"recurring"`
	Reserves  []reserveLineResponse   `json:"reserves"`
}

type recurringLineResponse struct {
	PaymentID       string `json:"payment_id"`
	Name            string `json:"name"`
	DueOn           string `json:"due_on"`
	PlannedCents    int64  `json:"planned_cents"`
	ActualCents     int64  `json:"actual_cents"`
	DifferenceCents int64  `json:"difference_cents"`
	Skipped         bool   `json:"skipped"`
}

// reserveLineResponse has planned_cents null in past months, whose rates
// were never stored.
type reserveLineResponse struct {
	PaymentID    string `json:"payment_id"`
	Name         string `json:"name"`
	PotID        string `json:"pot_id"`
	DueOn        string `json:"due_on,omitempty"`
	PlannedCents *int64 `json:"planned_cents"`
	ActualCents  int64  `json:"actual_cents"`
	Preview      bool   `json:"preview"`
}

type reserveDueResponse struct {
	PaymentID    string `json:"payment_id"`
	Name         string `json:"name"`
	PotID        string `json:"pot_id"`
	DueOn        string `json:"due_on"`
	AmountCents  int64  `json:"amount_cents"`
	CoveredCents int64  `json:"covered_cents"`
	Booked       bool   `json:"booked"`
}

// GET /ledgers/{ledger}/budget?month=YYYY-MM&date=YYYY-MM-DD
// The budget of the main pot for one month; without month, the current month,
// which date (the app's local date) decides. plan_source works as in the pot
// summary. Only current_month can be changed.
func (s *Server) budget(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.requireLedgerAccess(w, r); !ok {
		return
	}
	today, ok := s.today(w, r)
	if !ok {
		return
	}
	b, err := s.store.Budget(r.Context(), ledger.BudgetQuery{
		LedgerID: r.PathValue("ledger"),
		Month:    r.URL.Query().Get("month"),
		Today:    today,
	})
	if err != nil {
		fail(w, err)
		return
	}

	categories := make([]budgetCategoryResponse, 0, len(b.Categories))
	for _, c := range b.Categories {
		categories = append(categories, budgetCategoryFrom(c))
	}
	dues := make([]reserveDueResponse, 0, len(b.ReserveDues))
	for _, d := range b.ReserveDues {
		dues = append(dues, reserveDueResponse(d))
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"pot_id":        b.PotID,
		"month":         b.Month,
		"current_month": b.Current,
		"plan_source":   b.PlanSource,
		"from":          b.From,
		"to":            b.To,
		"categories":    categories,
		"reserve_dues":  dues,
	})
}

func budgetCategoryFrom(c ledger.BudgetCategory) budgetCategoryResponse {
	out := budgetCategoryResponse{
		planLineResponse: planLineResponse(c.CategoryLine),
		Recurring:        make([]recurringLineResponse, 0, len(c.Recurring)),
		Reserves:         make([]reserveLineResponse, 0, len(c.Reserves)),
	}
	for _, l := range c.Recurring {
		out.Recurring = append(out.Recurring, recurringLineResponse(l))
	}
	for _, l := range c.Reserves {
		out.Reserves = append(out.Reserves, reserveLineResponse(l))
	}
	return out
}
