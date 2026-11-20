package api

import (
	"context"
	"database/sql"
	"net/http"

	"github.com/Rakeemovic/slowpoke-oasis/backend/internal/ledger"
)

// requireUser returns the ID of the logged-in user, or writes the error and
// returns false: 401 without a valid login, 403 for a session cookie without
// the CSRF header (see session.go).
func (s *Server) requireUser(w http.ResponseWriter, r *http.Request) (string, bool) {
	token, fromCookie := sessionToken(r)
	if token == "" {
		writeError(w, http.StatusUnauthorized, "missing login token, please log in")
		return "", false
	}
	if fromCookie && r.Header.Get(csrfHeader) == "" {
		fail(w, ErrCSRFHeaderMissing)
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
// The answer carries the recovery code. It is shown only this once; the
// server keeps nothing but its hash.
func (s *Server) register(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Username    string `json:"username"`
		Password    string `json:"password"`
		DisplayName string `json:"display_name"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	id, code, err := s.accounts.Register(r.Context(), req.Username, req.Password, req.DisplayName, s.createStarterData)
	if err != nil {
		fail(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]string{"id": id, "recovery_code": code})
}

// POST /auth/login
// With "cookie": true (web app) the token is set as an HttpOnly cookie and the
// answer is 204 without a body, so JavaScript never sees it. Otherwise (native
// apps) the token is returned in the body.
func (s *Server) login(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Username string `json:"username"`
		Password string `json:"password"`
		Cookie   bool   `json:"cookie"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	token, err := s.accounts.Login(r.Context(), req.Username, req.Password)
	if err != nil {
		fail(w, err)
		return
	}
	if req.Cookie {
		s.setSessionCookie(w, token)
		w.WriteHeader(http.StatusNoContent)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"token": token})
}

// POST /auth/logout
func (s *Server) logout(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.requireUser(w, r); !ok {
		return
	}
	token, _ := sessionToken(r)
	if err := s.accounts.Logout(r.Context(), token); err != nil {
		fail(w, err)
		return
	}
	s.clearSessionCookieIfUsed(w, r)
	w.WriteHeader(http.StatusNoContent)
}

// POST /auth/recover
// Sets a new password with the recovery code. The code is used up and the
// answer carries a new one. All logins of the account end.
func (s *Server) recoverAccount(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Username     string `json:"username"`
		RecoveryCode string `json:"recovery_code"`
		NewPassword  string `json:"new_password"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	code, err := s.accounts.Recover(r.Context(), req.Username, req.RecoveryCode, req.NewPassword)
	if err != nil {
		fail(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"recovery_code": code})
}

// PUT /auth/password
// Requires the current password. Other logins of the account end; the one
// making the request stays valid.
func (s *Server) changePassword(w http.ResponseWriter, r *http.Request) {
	userID, ok := s.requireUser(w, r)
	if !ok {
		return
	}
	var req struct {
		CurrentPassword string `json:"current_password"`
		NewPassword     string `json:"new_password"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	token, _ := sessionToken(r)
	if err := s.accounts.ChangePassword(r.Context(), userID, token, req.CurrentPassword, req.NewPassword); err != nil {
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
	s.clearSessionCookieIfUsed(w, r)
	w.WriteHeader(http.StatusNoContent)
}

// createStarterData gives a new account its personal ledger (rule L6). It runs
// inside the registration transaction.
func (s *Server) createStarterData(ctx context.Context, tx *sql.Tx, userID string) error {
	return ledger.CreateStarterLedger(ctx, tx, userID, s.now())
}

// meResponse describes the signed-in user. PersonalLedgerID is null for
// accounts that were created before every account got a personal ledger.
type meResponse struct {
	UserID           string  `json:"user_id"`
	Username         string  `json:"username"`
	DisplayName      string  `json:"display_name"`
	PersonalLedgerID *string `json:"personal_ledger_id"`
}

// GET /me
func (s *Server) me(w http.ResponseWriter, r *http.Request) {
	userID, ok := s.requireUser(w, r)
	if !ok {
		return
	}
	profile, err := s.accounts.Profile(r.Context(), userID)
	if err != nil {
		fail(w, err)
		return
	}
	ledgerID, err := s.store.PersonalLedgerID(r.Context(), userID)
	if err != nil {
		fail(w, err)
		return
	}
	resp := meResponse{UserID: profile.UserID, Username: profile.Username, DisplayName: profile.DisplayName}
	if ledgerID != "" {
		resp.PersonalLedgerID = &ledgerID
	}
	writeJSON(w, http.StatusOK, resp)
}
