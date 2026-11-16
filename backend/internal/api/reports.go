package api

import (
	"net/http"

	"github.com/Rakeemovic/slowpoke-oasis/backend/internal/ledger"
)

// transactionPatchRequest is the JSON body for changing a transaction.
// Fields that are left out stay unchanged.
type transactionPatchRequest struct {
	Type             *string `json:"type"`
	CategoryID       *string `json:"category_id"`
	PotID            *string `json:"pot_id"`
	CounterpartPotID *string `json:"counterpart_pot_id"`
	AmountCents      *int64  `json:"amount_cents"`
	BookedOn         *string `json:"booked_on"`
	Note             *string `json:"note"`
}

// PATCH /ledgers/{ledger}/transactions/{id}
func (s *Server) updateTransaction(w http.ResponseWriter, r *http.Request) {
	userID, ok := s.requireLedgerAccess(w, r)
	if !ok {
		return
	}
	var req transactionPatchRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	if err := s.store.AuthorizeTransactionChange(r.Context(), userID, r.PathValue("ledger"), r.PathValue("id"), ledger.ActionEdit); err != nil {
		s.writeStoreError(w, err)
		return
	}
	patch := ledger.TransactionPatch{
		CategoryID:       req.CategoryID,
		PotID:            req.PotID,
		CounterpartPotID: req.CounterpartPotID,
		AmountCents:      req.AmountCents,
		BookedOn:         req.BookedOn,
		Note:             req.Note,
	}
	if req.Type != nil {
		t := ledger.TxType(*req.Type)
		patch.Type = &t
	}
	err := s.store.UpdateTransaction(r.Context(), r.PathValue("ledger"), r.PathValue("id"), patch, s.now())
	if err != nil {
		s.writeStoreError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

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
		s.writeStoreError(w, err)
		return
	}
	sum, err := s.store.PotSummary(r.Context(), ledgerID, r.PathValue("pot"), from, to)
	if err != nil {
		s.writeStoreError(w, err)
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
		s.writeStoreError(w, err)
		return
	}
	totals, err := s.store.CategoryTotals(r.Context(), ledgerID, from, to)
	if err != nil {
		s.writeStoreError(w, err)
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
