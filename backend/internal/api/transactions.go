package api

import (
	"context"
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
	IsSpontaneous    bool   `json:"is_spontaneous"`
	// RecurringPaymentID and RecurringDueOn book one occurrence of a
	// recurring payment (rule R6). They cannot be changed afterwards.
	RecurringPaymentID string `json:"recurring_payment_id"`
	RecurringDueOn     string `json:"recurring_due_on"`
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
	IsSpontaneous    *bool          `json:"is_spontaneous"`
	// CategoryScope also gives the new category to other bookings of the
	// same recurring payment (rule R9). Left out, only this booking changes.
	CategoryScope ledger.CategoryScope `json:"category_scope"`
}

// patch turns the request into the change for the ledger package.
func (req transactionPatchRequest) patch(editorID string) ledger.TransactionPatch {
	return ledger.TransactionPatch{
		Type:             req.Type,
		CategoryID:       req.CategoryID,
		PotID:            req.PotID,
		CounterpartPotID: req.CounterpartPotID,
		AmountCents:      req.AmountCents,
		BookedOn:         req.BookedOn,
		Note:             req.Note,
		IsSpontaneous:    req.IsSpontaneous,
		CategoryScope:    req.CategoryScope,
		EditorID:         editorID,
	}
}

// transactionResponse is the JSON representation of a stored transaction.
type transactionResponse struct {
	ID       string `json:"id"`
	LedgerID string `json:"ledger_id"`
	UserID   string `json:"user_id"`
	// Username is null for the former-member placeholder (rule X18).
	Username         *string `json:"username"`
	Type             string  `json:"type"`
	CategoryID       string  `json:"category_id,omitempty"`
	PotID            string  `json:"pot_id"`
	CounterpartPotID string  `json:"counterpart_pot_id,omitempty"`
	AmountCents      int64   `json:"amount_cents"`
	BookedOn         string  `json:"booked_on"`
	Note             string  `json:"note,omitempty"`
	IsSpontaneous    bool    `json:"is_spontaneous"`
	// The occurrence of a recurring payment this expense pays (rule R6).
	RecurringPaymentID string `json:"recurring_payment_id,omitempty"`
	RecurringDueOn     string `json:"recurring_due_on,omitempty"`
	// Counterpart is set for transfers between ledgers (rule X17).
	Counterpart *counterpartResponse `json:"counterpart,omitempty"`
}

// counterpartResponse names the other ledger of a transfer. ledger_id and
// name are left out when the viewer cannot access that ledger.
type counterpartResponse struct {
	Kind     string `json:"kind"`
	LedgerID string `json:"ledger_id,omitempty"`
	Name     string `json:"name,omitempty"`
}

// newTransactionResponse converts a transaction. counterparts comes from
// counterpartsOf and may lack the transaction.
func newTransactionResponse(t ledger.Transaction, counterparts map[string]ledger.Counterpart) transactionResponse {
	resp := transactionResponse{
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
		IsSpontaneous:    t.IsSpontaneous,
		Username:         usernameOrNull(t.Username),

		RecurringPaymentID: t.RecurringPaymentID,
		RecurringDueOn:     t.RecurringDueOn,
	}
	if c, ok := counterparts[t.ID]; ok {
		resp.Counterpart = &counterpartResponse{Kind: c.Kind, LedgerID: c.LedgerID, Name: c.Name}
	}
	return resp
}

// usernameOrNull turns the empty username of the former-member placeholder
// into JSON null (rule X18).
func usernameOrNull(username string) *string {
	if username == "" {
		return nil
	}
	return &username
}

// counterpartsOf looks up the other ledger of every transfer between ledgers
// in items, as seen by viewerID, with a single query.
func (s *Server) counterpartsOf(ctx context.Context, viewerID string, items []ledger.Transaction) (map[string]ledger.Counterpart, error) {
	var ids []string
	for _, t := range items {
		if t.Type == ledger.TypeTransferOut || t.Type == ledger.TypeTransferIn {
			ids = append(ids, t.ID)
		}
	}
	return s.store.Counterparts(ctx, viewerID, ids)
}

// listedTransactionResponse is a transaction in the normal list, plus what
// the viewer may do with it (rule X19).
type listedTransactionResponse struct {
	transactionResponse
	CanEdit   bool `json:"can_edit"`
	CanDelete bool `json:"can_delete"`
}

// deletedTransactionResponse is a soft-deleted transaction that can be
// restored: the transaction itself plus who deleted it and when.
type deletedTransactionResponse struct {
	transactionResponse
	DeletedAt         string  `json:"deleted_at"`
	DeletedBy         string  `json:"deleted_by"`
	DeletedByUsername *string `json:"deleted_by_username"`
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
		IsSpontaneous:    req.IsSpontaneous,

		RecurringPaymentID: req.RecurringPaymentID,
		RecurringDueOn:     req.RecurringDueOn,
	}, s.now())
	if err != nil {
		fail(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]string{"id": id})
}

// GET /ledgers/{ledger}/transactions?pot_id=&from=&to=&limit=&offset=
func (s *Server) listTransactions(w http.ResponseWriter, r *http.Request) {
	userID, ok := s.requireLedgerAccess(w, r)
	if !ok {
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

	out, err := s.listedResponses(r.Context(), userID, items)
	if err != nil {
		fail(w, err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}

// GET /ledgers/{ledger}/transactions/{id}
// Answers with the same fields as one entry of the list.
func (s *Server) getTransaction(w http.ResponseWriter, r *http.Request) {
	userID, ok := s.requireLedgerAccess(w, r)
	if !ok {
		return
	}
	t, err := s.store.GetTransaction(r.Context(), r.PathValue("ledger"), r.PathValue("id"))
	if err != nil {
		fail(w, err)
		return
	}
	out, err := s.listedResponses(r.Context(), userID, []ledger.Transaction{t})
	if err != nil {
		fail(w, err)
		return
	}
	writeJSON(w, http.StatusOK, out[0])
}

// listedResponses converts transactions for viewerID: each with its
// counterpart (rule X17) and what the viewer may do with it (rule X19).
func (s *Server) listedResponses(ctx context.Context, viewerID string, items []ledger.Transaction) ([]listedTransactionResponse, error) {
	counterparts, err := s.counterpartsOf(ctx, viewerID, items)
	if err != nil {
		return nil, err
	}
	permissions, err := s.store.Permissions(ctx, viewerID, items)
	if err != nil {
		return nil, err
	}
	out := make([]listedTransactionResponse, 0, len(items))
	for _, t := range items {
		p := permissions[t.ID]
		out = append(out, listedTransactionResponse{
			transactionResponse: newTransactionResponse(t, counterparts),
			CanEdit:             p.CanEdit,
			CanDelete:           p.CanDelete,
		})
	}
	return out, nil
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
	if err == nil && req.IsSpontaneous != nil {
		// Only the author decides whether a purchase was spontaneous (rule X20).
		err = s.store.AuthorizeTransactionChange(r.Context(), userID, ledgerID, id, ledger.ActionMarkSpontaneous)
	}
	if err == nil {
		err = s.store.UpdateTransaction(r.Context(), ledgerID, id, req.patch(userID), s.now())
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

// GET /ledgers/{ledger}/transactions/deleted?limit=&offset=
func (s *Server) listDeletedTransactions(w http.ResponseWriter, r *http.Request) {
	userID, ok := s.requireLedgerAccess(w, r)
	if !ok {
		return
	}
	limit, offset, err := parsePaging(r.URL.Query())
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	items, err := s.store.ListDeletedTransactions(r.Context(), r.PathValue("ledger"), s.now(), limit, offset)
	if err != nil {
		fail(w, err)
		return
	}
	plain := make([]ledger.Transaction, 0, len(items))
	for _, d := range items {
		plain = append(plain, d.Transaction)
	}
	counterparts, err := s.counterpartsOf(r.Context(), userID, plain)
	if err != nil {
		fail(w, err)
		return
	}
	out := make([]deletedTransactionResponse, 0, len(items))
	for _, d := range items {
		out = append(out, deletedTransactionResponse{
			transactionResponse: newTransactionResponse(d.Transaction, counterparts),
			DeletedAt:           d.DeletedAt,
			DeletedBy:           d.DeletedBy,
			DeletedByUsername:   usernameOrNull(d.DeletedByUsername),
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
