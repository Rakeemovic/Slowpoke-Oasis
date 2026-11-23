package api

import (
	"net/http"

	"github.com/Rakeemovic/slowpoke-oasis/backend/internal/ledger"
)

// recurringPaymentResponse is one recurring payment of a ledger.
type recurringPaymentResponse struct {
	ID             string  `json:"id"`
	Name           string  `json:"name"`
	AmountCents    int64   `json:"amount_cents"`
	CategoryID     string  `json:"category_id"`
	PotID          string  `json:"pot_id"`
	IntervalMonths int     `json:"interval_months"`
	DueDay         int     `json:"due_day"`
	FirstMonth     string  `json:"first_month"`
	LastMonth      *string `json:"last_month"`
	Archived       bool    `json:"archived"`
	// NextDueOn is the first due date from today on, booked or not; null when
	// the payment has ended.
	NextDueOn *string `json:"next_due_on"`
}

func newRecurringPaymentResponse(p ledger.RecurringPayment, today string) recurringPaymentResponse {
	return recurringPaymentResponse{
		ID:             p.ID,
		Name:           p.Name,
		AmountCents:    p.AmountCents,
		CategoryID:     p.CategoryID,
		PotID:          p.PotID,
		IntervalMonths: p.IntervalMonths,
		DueDay:         p.DueDay,
		FirstMonth:     p.FirstMonth,
		LastMonth:      nullIfEmpty(p.LastMonth),
		Archived:       p.Archived,
		NextDueOn:      nullIfEmpty(p.NextDueOn(today)),
	}
}

// nullIfEmpty turns an empty string into JSON null.
func nullIfEmpty(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

// today returns the app's local date from ?date=, or the server's date.
// If the parameter is invalid, it writes the response and returns false.
func (s *Server) today(w http.ResponseWriter, r *http.Request) (string, bool) {
	today, err := ledger.Today(r.URL.Query().Get("date"), s.now())
	if err != nil {
		fail(w, err)
		return "", false
	}
	return today, true
}

// GET /ledgers/{ledger}/recurring-payments?date=YYYY-MM-DD
// Archived payments are included.
func (s *Server) listRecurringPayments(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.requireLedgerAccess(w, r); !ok {
		return
	}
	today, ok := s.today(w, r)
	if !ok {
		return
	}
	items, err := s.store.ListRecurringPayments(r.Context(), r.PathValue("ledger"))
	if err != nil {
		fail(w, err)
		return
	}
	out := make([]recurringPaymentResponse, 0, len(items))
	for _, p := range items {
		out = append(out, newRecurringPaymentResponse(p, today))
	}
	writeJSON(w, http.StatusOK, out)
}

// POST /ledgers/{ledger}/recurring-payments?date=YYYY-MM-DD
// Every member may create recurring payments (rule R8). Occurrences due
// before date never become open (rule R5).
func (s *Server) createRecurringPayment(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.requireLedgerAccess(w, r); !ok {
		return
	}
	today, ok := s.today(w, r)
	if !ok {
		return
	}
	var req struct {
		Name           string `json:"name"`
		AmountCents    int64  `json:"amount_cents"`
		CategoryID     string `json:"category_id"`
		PotID          string `json:"pot_id"`
		IntervalMonths int    `json:"interval_months"`
		DueDay         int    `json:"due_day"`
		FirstMonth     string `json:"first_month"`
		LastMonth      string `json:"last_month"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	id, err := s.store.CreateRecurringPayment(r.Context(), ledger.NewRecurringPayment{
		LedgerID:    r.PathValue("ledger"),
		Name:        req.Name,
		AmountCents: req.AmountCents,
		CategoryID:  req.CategoryID,
		PotID:       req.PotID,
		Schedule: ledger.Schedule{
			IntervalMonths: req.IntervalMonths,
			DueDay:         req.DueDay,
			FirstMonth:     req.FirstMonth,
			LastMonth:      req.LastMonth,
		},
		Today: today,
	}, s.now())
	if err != nil {
		fail(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]string{"id": id})
}

// PATCH /ledgers/{ledger}/recurring-payments/{payment}?date=YYYY-MM-DD
// Only the owner (rule R8). Fields that are left out stay unchanged;
// "last_month": "" removes the end.
func (s *Server) updateRecurringPayment(w http.ResponseWriter, r *http.Request) {
	if !s.requireLedgerOwner(w, r) {
		return
	}
	today, ok := s.today(w, r)
	if !ok {
		return
	}
	var req struct {
		Name           *string `json:"name"`
		AmountCents    *int64  `json:"amount_cents"`
		CategoryID     *string `json:"category_id"`
		PotID          *string `json:"pot_id"`
		IntervalMonths *int    `json:"interval_months"`
		DueDay         *int    `json:"due_day"`
		FirstMonth     *string `json:"first_month"`
		LastMonth      *string `json:"last_month"`
		Archived       *bool   `json:"archived"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	err := s.store.UpdateRecurringPayment(r.Context(), r.PathValue("ledger"), r.PathValue("payment"),
		ledger.RecurringPaymentUpdate{
			Name:           req.Name,
			AmountCents:    req.AmountCents,
			CategoryID:     req.CategoryID,
			PotID:          req.PotID,
			IntervalMonths: req.IntervalMonths,
			DueDay:         req.DueDay,
			FirstMonth:     req.FirstMonth,
			LastMonth:      req.LastMonth,
			Archived:       req.Archived,
			Today:          today,
		})
	if err != nil {
		fail(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// occurrenceResponse is one open occurrence of a recurring payment.
type occurrenceResponse struct {
	RecurringPaymentID string `json:"recurring_payment_id"`
	Name               string `json:"name"`
	AmountCents        int64  `json:"amount_cents"`
	CategoryID         string `json:"category_id"`
	PotID              string `json:"pot_id"`
	DueOn              string `json:"due_on"`
	// Status is "overdue", "due" (today) or "upcoming".
	Status string `json:"status"`
}

// GET /ledgers/{ledger}/recurring-payments/due?date=YYYY-MM-DD
// Lists the open occurrences up to the end of the current payday period.
func (s *Server) listOpenOccurrences(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.requireLedgerAccess(w, r); !ok {
		return
	}
	today, ok := s.today(w, r)
	if !ok {
		return
	}
	items, err := s.store.OpenOccurrences(r.Context(), r.PathValue("ledger"), today)
	if err != nil {
		fail(w, err)
		return
	}
	out := make([]occurrenceResponse, 0, len(items))
	for _, o := range items {
		out = append(out, occurrenceResponse{
			RecurringPaymentID: o.Payment.ID,
			Name:               o.Payment.Name,
			AmountCents:        o.Payment.AmountCents,
			CategoryID:         o.Payment.CategoryID,
			PotID:              o.Payment.PotID,
			DueOn:              o.DueOn,
			Status:             string(o.Status),
		})
	}
	writeJSON(w, http.StatusOK, out)
}

// PUT /ledgers/{ledger}/recurring-payments/{payment}/skips/{due_on}
// Every member may skip an occurrence (rule R7). Skipping twice is fine.
func (s *Server) skipOccurrence(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.requireLedgerAccess(w, r); !ok {
		return
	}
	err := s.store.SkipOccurrence(r.Context(), r.PathValue("ledger"), r.PathValue("payment"), r.PathValue("due_on"))
	if err != nil {
		fail(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// DELETE /ledgers/{ledger}/recurring-payments/{payment}/skips/{due_on}
// Every member may undo a skip (rule R7).
func (s *Server) undoSkip(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.requireLedgerAccess(w, r); !ok {
		return
	}
	err := s.store.UndoSkip(r.Context(), r.PathValue("ledger"), r.PathValue("payment"), r.PathValue("due_on"))
	if err != nil {
		fail(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
