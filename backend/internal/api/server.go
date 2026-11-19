// Package api exposes the ledger functions over HTTP as JSON.
//
// Requests that act on a user's data carry a bearer token obtained from /auth/login.
package api

import (
	"encoding/json"
	"log"
	"net/http"
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

	// Accounts and logins
	mux.HandleFunc("POST /auth/register", s.register)
	mux.HandleFunc("POST /auth/login", s.login)
	mux.HandleFunc("POST /auth/logout", s.logout)
	mux.HandleFunc("POST /auth/password-reset", s.requestPasswordReset)
	mux.HandleFunc("POST /auth/password-reset/confirm", s.confirmPasswordReset)
	mux.HandleFunc("DELETE /auth/account", s.deleteAccount)
	mux.HandleFunc("GET /me", s.me)

	// Households
	mux.HandleFunc("POST /households", s.createHousehold)
	mux.HandleFunc("GET /households", s.listHouseholds)
	mux.HandleFunc("GET /households/{household}/members", s.listHouseholdMembers)
	mux.HandleFunc("POST /households/{household}/members", s.addHouseholdMember)
	mux.HandleFunc("DELETE /households/{household}/members/{user}", s.removeHouseholdMember)
	mux.HandleFunc("PUT /households/{household}/owner/{user}", s.transferHouseholdOwnership)

	// Ledgers and categories
	mux.HandleFunc("POST /users/{user}/ledgers", s.createPersonalLedger)
	mux.HandleFunc("POST /ledgers/{ledger}/categories", s.createCategory)
	mux.HandleFunc("GET /ledgers/{ledger}/categories", s.listCategories)

	// Pots and planned amounts
	mux.HandleFunc("POST /ledgers/{ledger}/pots", s.createPot)
	mux.HandleFunc("GET /ledgers/{ledger}/pots", s.listPots)
	mux.HandleFunc("PATCH /ledgers/{ledger}/pots/{pot}", s.updatePot)
	mux.HandleFunc("PUT /ledgers/{ledger}/pots/{pot}/lines/{category}", s.setPlannedAmount)
	mux.HandleFunc("DELETE /ledgers/{ledger}/pots/{pot}/lines/{category}", s.deletePlannedAmount)

	// Transactions
	mux.HandleFunc("POST /ledgers/{ledger}/transactions", s.createTransaction)
	mux.HandleFunc("GET /ledgers/{ledger}/transactions", s.listTransactions)
	mux.HandleFunc("PATCH /ledgers/{ledger}/transactions/{id}", s.updateTransaction)
	mux.HandleFunc("DELETE /ledgers/{ledger}/transactions/{id}", s.deleteTransaction)
	mux.HandleFunc("GET /ledgers/{ledger}/transactions/deleted", s.listDeletedTransactions)
	mux.HandleFunc("POST /ledgers/{ledger}/transactions/{id}/restore", s.restoreTransaction)
	mux.HandleFunc("POST /transfers", s.createLinkedTransfer)

	// Reports
	mux.HandleFunc("GET /ledgers/{ledger}/pots/{pot}/summary", s.potSummary)
	mux.HandleFunc("GET /ledgers/{ledger}/analytics", s.analytics)

	return mux
}

// decodeJSON reads a JSON body with a size limit and rejects unknown fields.
// It writes 400 and returns false if the body is invalid.
func decodeJSON(w http.ResponseWriter, r *http.Request, v any) bool {
	dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxBodyBytes))
	dec.DisallowUnknownFields()
	if err := dec.Decode(v); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return false
	}
	return true
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
