package api

import (
	"errors"
	"log"
	"net/http"
	"strings"

	"github.com/Rakeemovic/slowpoke-oasis/backend/internal/auth"
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
	if errors.Is(err, auth.ErrUnauthenticated) {
		writeError(w, http.StatusUnauthorized, err.Error())
		return "", false
	}
	if err != nil {
		log.Printf("token lookup: %v", err)
		writeError(w, http.StatusInternalServerError, "internal error")
		return "", false
	}
	return userID, true
}

// POST /auth/register
func (s *Server) register(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Username    string `json:"username"`
		Password    string `json:"password"`
		DisplayName string `json:"display_name"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	id, err := s.accounts.Register(r.Context(), req.Username, req.Password, req.DisplayName)
	if err != nil {
		writeAuthError(w, err)
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
		writeAuthError(w, err)
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
		log.Printf("logout: %v", err)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// writeAuthError maps errors from the auth package to HTTP status codes.
func writeAuthError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, auth.ErrInvalidUsername), errors.Is(err, auth.ErrInvalidPassword):
		writeError(w, http.StatusBadRequest, err.Error())
	case errors.Is(err, auth.ErrUsernameTaken):
		writeError(w, http.StatusConflict, err.Error())
	case errors.Is(err, auth.ErrInvalidCredentials):
		writeError(w, http.StatusUnauthorized, err.Error())
	default:
		log.Printf("auth: %v", err)
		writeError(w, http.StatusInternalServerError, "internal error")
	}
}
