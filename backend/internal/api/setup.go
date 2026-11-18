package api

import "net/http"

// POST /users/{user}/ledgers
func (s *Server) createPersonalLedger(w http.ResponseWriter, r *http.Request) {
	userID, ok := s.requireUser(w, r)
	if !ok {
		return
	}
	if r.PathValue("user") != userID {
		writeError(w, http.StatusForbidden, "you can only create ledgers for yourself")
		return
	}

	var req struct {
		Name      string `json:"name"`
		PaydayDay int    `json:"payday_day"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	id, err := s.store.CreatePersonalLedger(r.Context(), userID, req.Name, req.PaydayDay, s.now())
	if err != nil {
		fail(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]string{"id": id})
}

// POST /ledgers/{ledger}/categories
func (s *Server) createCategory(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.requireLedgerAccess(w, r); !ok {
		return
	}

	var req struct {
		Name     string `json:"name"`
		IsIncome bool   `json:"is_income"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	id, err := s.store.CreateCategory(r.Context(), r.PathValue("ledger"), req.Name, req.IsIncome)
	if err != nil {
		fail(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]string{"id": id})
}
