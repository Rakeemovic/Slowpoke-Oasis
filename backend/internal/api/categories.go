package api

import (
	"net/http"

	"github.com/Rakeemovic/slowpoke-oasis/backend/internal/ledger"
)

// categoryResponse is one category of a ledger.
type categoryResponse struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	IsIncome bool   `json:"is_income"`
	Archived bool   `json:"archived"`
	// Purpose is null when the category has none (rule K5).
	Purpose    *string `json:"purpose"`
	IsFallback bool    `json:"is_fallback"`
	// TransactionCount counts the bookings that are not deleted.
	TransactionCount int `json:"transaction_count"`
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
		out = append(out, categoryResponse{
			ID:               c.ID,
			Name:             c.Name,
			IsIncome:         c.IsIncome,
			Archived:         c.Archived,
			Purpose:          purposeOrNull(c.Purpose),
			IsFallback:       c.IsFallback,
			TransactionCount: c.TransactionCount,
		})
	}
	writeJSON(w, http.StatusOK, out)
}

// purposeOrNull turns the empty purpose into JSON null.
func purposeOrNull(p ledger.Purpose) *string {
	if p == "" {
		return nil
	}
	s := string(p)
	return &s
}

// POST /ledgers/{ledger}/categories
// Every member may create categories.
func (s *Server) createCategory(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.requireLedgerAccess(w, r); !ok {
		return
	}

	var req struct {
		Name     string `json:"name"`
		IsIncome bool   `json:"is_income"`
		Purpose  string `json:"purpose"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	id, err := s.store.CreateCategory(r.Context(), ledger.NewCategory{
		LedgerID: r.PathValue("ledger"),
		Name:     req.Name,
		IsIncome: req.IsIncome,
		Purpose:  ledger.Purpose(req.Purpose),
	})
	if err != nil {
		fail(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]string{"id": id})
}

// requireCategoryOwner checks ledger access and that the user may change the
// ledger's categories (rule K9). If not, it writes the response.
func (s *Server) requireCategoryOwner(w http.ResponseWriter, r *http.Request) bool {
	userID, ok := s.requireLedgerAccess(w, r)
	if !ok {
		return false
	}
	if err := s.store.AuthorizeCategoryChange(r.Context(), userID, r.PathValue("ledger")); err != nil {
		fail(w, err)
		return false
	}
	return true
}

// PATCH /ledgers/{ledger}/categories/{category}
// Fields that are left out stay unchanged; "purpose": "" removes the purpose.
func (s *Server) updateCategory(w http.ResponseWriter, r *http.Request) {
	if !s.requireCategoryOwner(w, r) {
		return
	}
	var req struct {
		Name     *string         `json:"name"`
		Purpose  *ledger.Purpose `json:"purpose"`
		Archived *bool           `json:"archived"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	err := s.store.UpdateCategory(r.Context(), r.PathValue("ledger"), r.PathValue("category"),
		ledger.CategoryUpdate{Name: req.Name, Purpose: req.Purpose, Archived: req.Archived})
	if err != nil {
		fail(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// DELETE /ledgers/{ledger}/categories/{category}?move_to=
// The transactions move to move_to, or without it to the fallback category.
// Answers with the category that took over the transactions.
func (s *Server) deleteCategory(w http.ResponseWriter, r *http.Request) {
	if !s.requireCategoryOwner(w, r) {
		return
	}
	res, err := s.store.DeleteCategory(r.Context(), r.PathValue("ledger"), r.PathValue("category"),
		r.URL.Query().Get("move_to"))
	if err != nil {
		fail(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"moved_to":           res.TargetID,
		"moved_transactions": res.MovedTransactions,
	})
}
