package api

import (
	"net/http"

	"github.com/Rakeemovic/slowpoke-oasis/backend/internal/ledger"
)

// transactionRequest is the JSON body for creating a transaction.
type transactionRequest struct {
	Type             string `json:"type"`
	CategoryID       string `json:"category_id"`
	PotID            string `json:"pot_id"`
	CounterpartPotID string `json:"counterpart_pot_id"`
	AmountCents      int64  `json:"amount_cents"`
	BookedOn         string `json:"booked_on"`
	Note             string `json:"note"`
}

// transactionPatchRequest is the JSON body for changing a transaction.
// Fields that are left out stay unchanged.
type transactionPatchRequest struct {
	Type             *ledger.TxType `json:"type"`
	CategoryID       *string        `json:"category_id"`
	PotID            *string        `json:"pot_id"`
	CounterpartPotID *string        `json:"counterpart_pot_id"`
	AmountCents      *int64         `json:"amount_cents"`
	BookedOn         *string        `json:"booked_on"`
	Note             *string        `json:"note"`
}

// transactionResponse is the JSON representation of a stored transaction.
type transactionResponse struct {
	ID               string `json:"id"`
	LedgerID         string `json:"ledger_id"`
	UserID           string `json:"user_id"`
	Type             string `json:"type"`
	CategoryID       string `json:"category_id,omitempty"`
	PotID            string `json:"pot_id"`
	CounterpartPotID string `json:"counterpart_pot_id,omitempty"`
	AmountCents      int64  `json:"amount_cents"`
	BookedOn         string `json:"booked_on"`
	Note             string `json:"note,omitempty"`
}

func newTransactionResponse(t ledger.Transaction) transactionResponse {
	return transactionResponse{
		ID:               t.ID,
		LedgerID:         t.LedgerID,
		UserID:           t.UserID,
		Type:             string(t.Type),
		CategoryID:       t.CategoryID,
		PotID:            t.PotID,
		CounterpartPotID: t.CounterpartPotID,
		AmountCents:      t.AmountCents,
		BookedOn:         t.BookedOn,
		Note:             t.Note,
	}
}

// deletedTransactionResponse is a soft-deleted transaction that can be
// restored: the transaction itself plus who deleted it and when.
type deletedTransactionResponse struct {
	transactionResponse
	DeletedAt     string `json:"deleted_at"`
	DeletedBy     string `json:"deleted_by"`
	BookedByName  string `json:"booked_by_name"`
	DeletedByName string `json:"deleted_by_name"`
}

// POST /ledgers/{ledger}/transactions
func (s *Server) createTransaction(w http.ResponseWriter, r *http.Request) {
	userID, ok := s.requireLedgerAccess(w, r)
	if !ok {
		return
	}
	var req transactionRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	id, err := s.store.CreateTransaction(r.Context(), ledger.NewTransaction{
		LedgerID:         r.PathValue("ledger"),
		UserID:           userID,
		Type:             ledger.TxType(req.Type),
		CategoryID:       req.CategoryID,
		PotID:            req.PotID,
		CounterpartPotID: req.CounterpartPotID,
		AmountCents:      req.AmountCents,
		BookedOn:         req.BookedOn,
		Note:             req.Note,
	}, s.now())
	if err != nil {
		fail(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]string{"id": id})
}

// GET /ledgers/{ledger}/transactions?pot_id=&from=&to=&limit=&offset=
func (s *Server) listTransactions(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.requireLedgerAccess(w, r); !ok {
		return
	}
	q := r.URL.Query()
	limit, offset, err := parsePaging(q)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	items, err := s.store.ListTransactions(r.Context(), ledger.ListFilter{
		LedgerID: r.PathValue("ledger"),
		PotID:    q.Get("pot_id"),
		From:     q.Get("from"),
		To:       q.Get("to"),
		Limit:    limit,
		Offset:   offset,
	})
	if err != nil {
		fail(w, err)
		return
	}

	out := make([]transactionResponse, 0, len(items))
	for _, t := range items {
		out = append(out, newTransactionResponse(t))
	}
	writeJSON(w, http.StatusOK, out)
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
	ledgerID, id := r.PathValue("ledger"), r.PathValue("id")

	err := s.store.AuthorizeTransactionChange(r.Context(), userID, ledgerID, id, ledger.ActionEdit)
	if err == nil {
		err = s.store.UpdateTransaction(r.Context(), ledgerID, id, ledger.TransactionPatch(req), s.now())
	}
	if err != nil {
		fail(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// DELETE /ledgers/{ledger}/transactions/{id}
func (s *Server) deleteTransaction(w http.ResponseWriter, r *http.Request) {
	userID, ok := s.requireLedgerAccess(w, r)
	if !ok {
		return
	}
	ledgerID, id := r.PathValue("ledger"), r.PathValue("id")

	err := s.authorizeLegChange(r.Context(), userID, ledgerID, id, ledger.ActionDelete)
	if err == nil {
		err = s.store.DeleteTransaction(r.Context(), ledgerID, id, userID, s.now())
	}
	if err != nil {
		fail(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// GET /ledgers/{ledger}/transactions/deleted
func (s *Server) listDeletedTransactions(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.requireLedgerAccess(w, r); !ok {
		return
	}
	items, err := s.store.ListDeletedTransactions(r.Context(), r.PathValue("ledger"), s.now())
	if err != nil {
		fail(w, err)
		return
	}
	out := make([]deletedTransactionResponse, 0, len(items))
	for _, d := range items {
		out = append(out, deletedTransactionResponse{
			transactionResponse: newTransactionResponse(d.Transaction),
			DeletedAt:           d.DeletedAt,
			DeletedBy:           d.DeletedBy,
			BookedByName:        d.BookedByName,
			DeletedByName:       d.DeletedByName,
		})
	}
	writeJSON(w, http.StatusOK, out)
}

// POST /ledgers/{ledger}/transactions/{id}/restore
// Restoring follows the same permission rule as deleting.
func (s *Server) restoreTransaction(w http.ResponseWriter, r *http.Request) {
	userID, ok := s.requireLedgerAccess(w, r)
	if !ok {
		return
	}
	ledgerID, id := r.PathValue("ledger"), r.PathValue("id")

	err := s.authorizeLegChange(r.Context(), userID, ledgerID, id, ledger.ActionDelete)
	if err == nil {
		err = s.store.RestoreTransaction(r.Context(), ledgerID, id, s.now())
	}
	if err != nil {
		fail(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
