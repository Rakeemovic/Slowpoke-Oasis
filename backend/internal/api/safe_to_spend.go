package api

import "net/http"

// safeToSpendResponse carries the amount and its breakdown, so the app can
// show where the money goes (rule L8). Its fields match ledger.SafeToSpend.
type safeToSpendResponse struct {
	AsOf              string `json:"as_of"`
	PeriodEnd         string `json:"period_end"`
	DaysLeft          int    `json:"days_left"`
	MainPotCents      int64  `json:"main_pot_cents"`
	RecurringCents    int64  `json:"recurring_cents"`
	RecurringCount    int    `json:"recurring_count"`
	ReserveRatesCents int64  `json:"reserve_rates_cents"`
	SavingsCents      int64  `json:"savings_cents"`
	BudgetLeftCents   int64  `json:"budget_left_cents"`
	AmountCents       int64  `json:"amount_cents"`
	PerDayCents       int64  `json:"per_day_cents"`
}

// GET /ledgers/{ledger}/safe-to-spend?date=YYYY-MM-DD
// What the main pot can spend freely until the end of the current payday
// period; date is the app's local today.
func (s *Server) safeToSpend(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.requireLedgerAccess(w, r); !ok {
		return
	}
	today, ok := s.today(w, r)
	if !ok {
		return
	}
	got, err := s.store.SafeToSpend(r.Context(), r.PathValue("ledger"), today)
	if err != nil {
		fail(w, err)
		return
	}
	writeJSON(w, http.StatusOK, safeToSpendResponse(got))
}
