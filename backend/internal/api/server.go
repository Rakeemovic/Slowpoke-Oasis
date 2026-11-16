// Package api exposes the ledger functions over HTTP as JSON.
//
// Requests that act on a user's data carry a bearer token obtained from /auth/login.
package api

import (
	"database/sql"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/Rakeemovic/slowpoke-oasis/backend/internal/auth"
	"github.com/Rakeemovic/slowpoke-oasis/backend/internal/ledger"
)

const maxBodyBytes = 1 << 20 // 1 MiB

// Server holds the dependencies of the HTTP handlers.
type Server struct {
	store    *ledger.Store
	accounts *auth.Service
	now      func() time.Time
}

// NewServer returns a Server that uses the given stores.
func NewServer(store *ledger.Store, accounts *auth.Service) *Server {
	return &Server{store: store, accounts: accounts, now: time.Now}
}

// Routes returns the HTTP handler with all endpoints registered.
func (s *Server) Routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /ledgers/{ledger}/transactions", s.createTransaction)
	mux.HandleFunc("GET /ledgers/{ledger}/transactions", s.listTransactions)
	mux.HandleFunc("DELETE /ledgers/{ledger}/transactions/{id}", s.deleteTransaction)
	mux.HandleFunc("GET /ledgers/{ledger}/transactions/deleted", s.listDeletedTransactions)
	mux.HandleFunc("POST /ledgers/{ledger}/transactions/{id}/restore", s.restoreTransaction)
	mux.HandleFunc("POST /households", s.createHousehold)
	mux.HandleFunc("GET /households", s.listHouseholds)
	mux.HandleFunc("POST /households/{household}/members", s.addHouseholdMember)
	mux.HandleFunc("DELETE /households/{household}/members/{user}", s.removeHouseholdMember)
	mux.HandleFunc("POST /auth/register", s.register)
	mux.HandleFunc("POST /auth/login", s.login)
	mux.HandleFunc("POST /auth/logout", s.logout)
	mux.HandleFunc("DELETE /auth/account", s.deleteAccount)
	mux.HandleFunc("POST /ledgers/{ledger}/pots", s.createPot)
	mux.HandleFunc("GET /ledgers/{ledger}/pots", s.listPots)
	mux.HandleFunc("PATCH /ledgers/{ledger}/pots/{pot}", s.updatePot)
	mux.HandleFunc("PUT /ledgers/{ledger}/pots/{pot}/lines/{category}", s.setPlannedAmount)
	mux.HandleFunc("DELETE /ledgers/{ledger}/pots/{pot}/lines/{category}", s.deletePlannedAmount)
	mux.HandleFunc("POST /users/{user}/ledgers", s.createPersonalLedger)
	mux.HandleFunc("POST /ledgers/{ledger}/categories", s.createCategory)
	return mux
}

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

// POST /ledgers/{ledger}/transactions
func (s *Server) createTransaction(w http.ResponseWriter, r *http.Request) {
	userID, ok := s.requireLedgerAccess(w, r)
	if !ok {
		return
	}

	var req transactionRequest
	dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxBodyBytes))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
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
		s.writeStoreError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]string{"id": id})
}

// GET /ledgers/{ledger}/transactions?pot_id=&from=&to=
func (s *Server) listTransactions(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.requireLedgerAccess(w, r); !ok {
		return
	}
	q := r.URL.Query()
	items, err := s.store.ListTransactions(r.Context(), ledger.ListFilter{
		LedgerID: r.PathValue("ledger"),
		PotID:    q.Get("pot_id"),
		From:     q.Get("from"),
		To:       q.Get("to"),
	})
	if err != nil {
		s.writeStoreError(w, err)
		return
	}

	out := make([]transactionResponse, 0, len(items))
	for _, t := range items {
		out = append(out, transactionResponse{
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
		})
	}
	writeJSON(w, http.StatusOK, out)
}

// DELETE /ledgers/{ledger}/transactions/{id}
func (s *Server) deleteTransaction(w http.ResponseWriter, r *http.Request) {
	userID, ok := s.requireLedgerAccess(w, r)
	if !ok {
		return
	}
	err := s.store.DeleteTransaction(r.Context(), r.PathValue("ledger"), r.PathValue("id"), userID, s.now())
	if err != nil {
		s.writeStoreError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// validationErrors are caused by the client's input and map to 400.
var validationErrors = []error{
	ledger.ErrLedgerRequired,
	ledger.ErrInvalidType,
	ledger.ErrAmountNotPositive,
	ledger.ErrInvalidDate,
	ledger.ErrCategoryRequired,
	ledger.ErrCategoryForbidden,
	ledger.ErrCounterpartRequired,
	ledger.ErrCounterpartForbidden,
	ledger.ErrSamePot,
	ledger.ErrNoteTooLong,
	ledger.ErrInvalidName,
	ledger.ErrInvalidPayday,
	ledger.ErrInvalidTarget,
	ledger.ErrTargetDateNeedsTarget,
	ledger.ErrInvalidAmount,
	ledger.ErrCategoryKindMismatch,
}

// writeStoreError maps errors from the ledger package to HTTP status codes.
func (s *Server) writeStoreError(w http.ResponseWriter, err error) {
	for _, v := range validationErrors {
		if errors.Is(err, v) {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
	}

	switch {
	case errors.Is(err, ledger.ErrTransactionNotFound), errors.Is(err, ledger.ErrPotLineNotFound), errors.Is(err, sql.ErrNoRows):
		writeError(w, http.StatusNotFound, "not found")
	case errors.Is(err, ledger.ErrPotArchived), errors.Is(err, ledger.ErrDefaultPotProtected):
		writeError(w, http.StatusConflict, err.Error())
	case strings.Contains(err.Error(), "constraint failed"):
		// Database rules such as foreign keys and CHECK constraints.
		writeError(w, http.StatusUnprocessableEntity, "the request violates a data rule")
	default:
		log.Printf("internal error: %v", err)
		writeError(w, http.StatusInternalServerError, "internal error")
	}
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		log.Printf("write response: %v", err)
	}
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}
