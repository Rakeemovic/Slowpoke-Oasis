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

// categoryResponse is one category of a ledger.
type categoryResponse struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	IsIncome bool   `json:"is_income"`
	Archived bool   `json:"archived"`
}

// GET /ledgers/{ledger}/categories
func (s *Server) listCategories(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.requireLedgerAccess(w, r); !ok {
		return
	}
	items, err := s.store.ListCategories(r.Context(), r.PathValue("ledger"))
	if err != nil {
		fail(w, err)
		return
	}
	out := make([]categoryResponse, 0, len(items))
	for _, c := range items {
		out = append(out, categoryResponse{ID: c.ID, Name: c.Name, IsIncome: c.IsIncome, Archived: c.Archived})
	}
	writeJSON(w, http.StatusOK, out)
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
