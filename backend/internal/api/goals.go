package api

import "net/http"

// savingsGoalResponse is the rate of one savings goal in the current payday
// period (rule T8).
type savingsGoalResponse struct {
	PotID            string `json:"pot_id"`
	Name             string `json:"name"`
	TargetCents      int64  `json:"target_cents"`
	TargetDate       string `json:"target_date,omitempty"`
	MonthlyRateCents int64  `json:"monthly_rate_cents"`
	BalanceCents     int64  `json:"balance_cents"`
	Periods          int    `json:"periods"`
	SavedCents       int64  `json:"saved_cents"`
	RateCents        int64  `json:"rate_cents"`
	OpenCents        int64  `json:"open_cents"`
	Reached          bool   `json:"reached"`
	DatePassed       bool   `json:"date_passed"`
}

// GET /ledgers/{ledger}/savings-goals?date=YYYY-MM-DD
// The pots that save a rate (a target date or a fixed rate), with what they
// should receive in the current payday period; date is the app's local today.
func (s *Server) listSavingsGoals(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.requireLedgerAccess(w, r); !ok {
		return
	}
	today, ok := s.today(w, r)
	if !ok {
		return
	}
	goals, err := s.store.SavingsGoals(r.Context(), r.PathValue("ledger"), today)
	if err != nil {
		fail(w, err)
		return
	}
	out := make([]savingsGoalResponse, 0, len(goals))
	for _, g := range goals {
		out = append(out, savingsGoalResponse{
			PotID:            g.Pot.ID,
			Name:             g.Pot.Name,
			TargetCents:      g.Pot.TargetCents,
			TargetDate:       g.Pot.TargetDate,
			MonthlyRateCents: g.Pot.MonthlyRateCents,
			BalanceCents:     g.Pot.BalanceCents,
			Periods:          g.Periods,
			SavedCents:       g.SavedCents,
			RateCents:        g.RateCents,
			OpenCents:        g.OpenCents,
			Reached:          g.Reached,
			DatePassed:       g.DatePassed,
		})
	}
	writeJSON(w, http.StatusOK, out)
}
