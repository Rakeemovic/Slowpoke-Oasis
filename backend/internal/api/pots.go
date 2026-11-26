package api

import (
	"net/http"

	"github.com/Rakeemovic/slowpoke-oasis/backend/internal/ledger"
)

// potResponse is the JSON representation of a pot.
type potResponse struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Kind        string `json:"kind"`
	IsDefault   bool   `json:"is_default"`
	Archived    bool   `json:"archived"`
	TargetCents int64  `json:"target_cents,omitempty"`
	TargetDate  string `json:"target_date,omitempty"`
	// MonthlyRateCents is the fixed rate per payday period (rule T6).
	MonthlyRateCents int64 `json:"monthly_rate_cents,omitempty"`
	// IsSeparateAccount: a real sub-account at the bank (rule T9).
	IsSeparateAccount bool `json:"is_separate_account"`
	// RoundUpMultiplier is 0 (off) or 1, 2, 3 or 5: the pot collects the
	// spare change of expenses from the main pot (rule T10).
	RoundUpMultiplier int    `json:"round_up_multiplier"`
	RoundUpFrom       string `json:"round_up_from,omitempty"`
	// ReserveFor is the recurring payment this pot is the reserve of (R10).
	ReserveFor   *string           `json:"reserve_for"`
	BalanceCents int64             `json:"balance_cents"`
	Planned      []plannedResponse `json:"planned"`
}

type plannedResponse struct {
	CategoryID  string `json:"category_id"`
	AmountCents int64  `json:"amount_cents"`
}

// POST /ledgers/{ledger}/pots?date=YYYY-MM-DD
// round_up_multiplier (1, 2, 3 or 5) makes the new pot collect round-ups from
// date (the app's local today) on, as in updatePot.
func (s *Server) createPot(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.requireLedgerAccess(w, r); !ok {
		return
	}
	var req struct {
		Name              string `json:"name"`
		TargetCents       int64  `json:"target_cents"`
		TargetDate        string `json:"target_date"`
		MonthlyRateCents  int64  `json:"monthly_rate_cents"`
		SeparateAccount   bool   `json:"is_separate_account"`
		RoundUpMultiplier int    `json:"round_up_multiplier"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	today, ok := s.today(w, r)
	if !ok {
		return
	}
	id, err := s.store.CreatePot(r.Context(), ledger.NewPot{
		LedgerID:          r.PathValue("ledger"),
		Name:              req.Name,
		TargetCents:       req.TargetCents,
		TargetDate:        req.TargetDate,
		MonthlyRateCents:  req.MonthlyRateCents,
		SeparateAccount:   req.SeparateAccount,
		RoundUpMultiplier: req.RoundUpMultiplier,
		Today:             today,
	})
	if err != nil {
		fail(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]string{"id": id})
}

// GET /ledgers/{ledger}/pots
func (s *Server) listPots(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.requireLedgerAccess(w, r); !ok {
		return
	}
	pots, err := s.store.ListPots(r.Context(), r.PathValue("ledger"))
	if err != nil {
		fail(w, err)
		return
	}

	out := make([]potResponse, 0, len(pots))
	for _, p := range pots {
		planned := make([]plannedResponse, 0, len(p.Planned))
		for _, l := range p.Planned {
			planned = append(planned, plannedResponse{CategoryID: l.CategoryID, AmountCents: l.AmountCents})
		}
		out = append(out, potResponse{
			ID:                p.ID,
			Name:              p.Name,
			Kind:              p.Kind,
			IsDefault:         p.IsDefault,
			Archived:          p.Archived,
			TargetCents:       p.TargetCents,
			TargetDate:        p.TargetDate,
			MonthlyRateCents:  p.MonthlyRateCents,
			IsSeparateAccount: p.IsSeparateAccount,
			RoundUpMultiplier: p.RoundUpMultiplier,
			RoundUpFrom:       p.RoundUpFrom,
			ReserveFor:        nullIfEmpty(p.ReserveFor),
			BalanceCents:      p.BalanceCents,
			Planned:           planned,
		})
	}
	writeJSON(w, http.StatusOK, out)
}

// PATCH /ledgers/{ledger}/pots/{pot}
// Only the fields present in the body are changed. Sending target_cents: 0
// removes the target and its date, monthly_rate_cents: 0 the fixed rate.
// round_up_multiplier (0, 1, 2, 3 or 5) switches the round-up of the pot; date
// (the app's local today) is the first day that is rounded up.
func (s *Server) updatePot(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.requireLedgerAccess(w, r); !ok {
		return
	}
	var req struct {
		Name              *string `json:"name"`
		TargetCents       *int64  `json:"target_cents"`
		TargetDate        *string `json:"target_date"`
		MonthlyRateCents  *int64  `json:"monthly_rate_cents"`
		SeparateAccount   *bool   `json:"is_separate_account"`
		RoundUpMultiplier *int    `json:"round_up_multiplier"`
		Archived          *bool   `json:"archived"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	today, ok := s.today(w, r)
	if !ok {
		return
	}
	err := s.store.UpdatePot(r.Context(), r.PathValue("ledger"), r.PathValue("pot"), ledger.PotUpdate{
		Name:              req.Name,
		TargetCents:       req.TargetCents,
		TargetDate:        req.TargetDate,
		MonthlyRateCents:  req.MonthlyRateCents,
		SeparateAccount:   req.SeparateAccount,
		RoundUpMultiplier: req.RoundUpMultiplier,
		Today:             today,
		Archived:          req.Archived,
	})
	if err != nil {
		fail(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// PUT /ledgers/{ledger}/pots/{pot}/lines/{category}?date=YYYY-MM-DD
// Monthly pots need "scope" in the body: "this_month" changes only the
// current month, "from_now_on" also the template for the months to come.
// date is the app's local date and decides which month is current.
func (s *Server) setPlannedAmount(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.requireLedgerAccess(w, r); !ok {
		return
	}
	var req struct {
		AmountCents int64            `json:"amount_cents"`
		Scope       ledger.PlanScope `json:"scope"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	change, ok := s.planChange(w, r, req.Scope)
	if !ok {
		return
	}
	change.AmountCents = req.AmountCents
	if err := s.store.SetPlannedAmount(r.Context(), change); err != nil {
		fail(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// DELETE /ledgers/{ledger}/pots/{pot}/lines/{category}?scope=...&date=YYYY-MM-DD
// The parameters work as for PUT.
func (s *Server) deletePlannedAmount(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.requireLedgerAccess(w, r); !ok {
		return
	}
	change, ok := s.planChange(w, r, ledger.PlanScope(r.URL.Query().Get("scope")))
	if !ok {
		return
	}
	if err := s.store.DeletePlannedAmount(r.Context(), change); err != nil {
		fail(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// planChange builds a change of a planned amount from the path and the
// date parameter. It writes the error response and returns false on failure.
func (s *Server) planChange(w http.ResponseWriter, r *http.Request, scope ledger.PlanScope) (ledger.PlanChange, bool) {
	today, ok := s.today(w, r)
	if !ok {
		return ledger.PlanChange{}, false
	}
	return ledger.PlanChange{
		LedgerID:   r.PathValue("ledger"),
		PotID:      r.PathValue("pot"),
		CategoryID: r.PathValue("category"),
		Scope:      scope,
		Today:      today,
	}, true
}
