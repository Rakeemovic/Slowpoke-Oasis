package api

import (
	"net/http"
	"strings"

	"github.com/Rakeemovic/slowpoke-oasis/backend/internal/ledger"
)

// bearerToken returns the token from "Authorization: Bearer <token>", or "".
func bearerToken(r *http.Request) string {
	const prefix = "Bearer "
	h := r.Header.Get("Authorization")
	if !strings.HasPrefix(h, prefix) {
		return ""
	}
	return strings.TrimSpace(h[len(prefix):])
}

// requireUser returns the ID of the logged-in user, or writes 401 and returns false.
func (s *Server) requireUser(w http.ResponseWriter, r *http.Request) (string, bool) {
	token := bearerToken(r)
	if token == "" {
		writeError(w, http.StatusUnauthorized, "missing bearer token, please log in")
		return "", false
	}
	userID, err := s.accounts.UserForToken(r.Context(), token)
	if err != nil {
		fail(w, err)
		return "", false
	}
	return userID, true
}

// requireLedgerAccess checks that the logged-in user may use the ledger in the
// path. It writes 401 without a valid login and 404 when the ledger is unknown
// or belongs to someone else.
func (s *Server) requireLedgerAccess(w http.ResponseWriter, r *http.Request) (string, bool) {
	userID, ok := s.requireUser(w, r)
	if !ok {
		return "", false
	}
	if !s.canAccessLedger(w, r, userID, r.PathValue("ledger")) {
		return "", false
	}
	return userID, true
}

// canAccessLedger reports whether the user may use a ledger. If not, it writes
// the response: 404 for unknown ledgers and for ledgers of other people alike,
// so that foreign IDs are not revealed.
func (s *Server) canAccessLedger(w http.ResponseWriter, r *http.Request, userID, ledgerID string) bool {
	allowed, err := s.store.CanAccessLedger(r.Context(), userID, ledgerID)
	if err != nil {
		fail(w, err)
		return false
	}
	if !allowed {
		fail(w, ledger.ErrLedgerNotFound)
		return false
	}
	return true
}

// POST /auth/register
func (s *Server) register(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Username    string `json:"username"`
		Email       string `json:"email"`
		Password    string `json:"password"`
		DisplayName string `json:"display_name"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	id, err := s.accounts.Register(r.Context(), req.Username, req.Email, req.Password, req.DisplayName)
	if err != nil {
		fail(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]string{"id": id})
}

// POST /auth/login
func (s *Server) login(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	token, err := s.accounts.Login(r.Context(), req.Username, req.Password)
	if err != nil {
		fail(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"token": token})
}

// POST /auth/logout
func (s *Server) logout(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.requireUser(w, r); !ok {
		return
	}
	if err := s.accounts.Logout(r.Context(), bearerToken(r)); err != nil {
		fail(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// POST /auth/password-reset
// Answers 202 for every well-formed address, so the answer does not reveal
// which addresses are registered. The code is sent by email.
func (s *Server) requestPasswordReset(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Email string `json:"email"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	if err := s.accounts.RequestPasswordReset(r.Context(), req.Email); err != nil {
		fail(w, err)
		return
	}
	w.WriteHeader(http.StatusAccepted)
}

// POST /auth/password-reset/confirm
// Sets a new password with the code from the email. All logins end.
func (s *Server) confirmPasswordReset(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Token       string `json:"token"`
		NewPassword string `json:"new_password"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	if err := s.accounts.ConfirmPasswordReset(r.Context(), req.Token, req.NewPassword); err != nil {
		fail(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// DELETE /auth/account
// Requires the current password. Removes the user with all personal ledgers
// and household memberships immediately; sessions are removed with the user.
func (s *Server) deleteAccount(w http.ResponseWriter, r *http.Request) {
	userID, ok := s.requireUser(w, r)
	if !ok {
		return
	}
	var req struct {
		Password string `json:"password"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	if err := s.accounts.DeleteAccount(r.Context(), userID, req.Password, ledger.RemoveUserData); err != nil {
		fail(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
