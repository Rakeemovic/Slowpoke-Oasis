package api

import (
	"errors"
	"log"
	"net/http"

	"github.com/Rakeemovic/slowpoke-oasis/backend/internal/auth"
	"github.com/Rakeemovic/slowpoke-oasis/backend/internal/ledger"
)

// householdResponse is one household of the logged-in user.
type householdResponse struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Role string `json:"role"`
}

// requireHouseholdMember checks that the logged-in user belongs to the household
// in the path. It writes 404 for other households, so their IDs are not revealed.
// It returns the user ID and the user's role in the household.
func (s *Server) requireHouseholdMember(w http.ResponseWriter, r *http.Request) (string, string, bool) {
	userID, ok := s.requireUser(w, r)
	if !ok {
		return "", "", false
	}
	role, err := s.store.HouseholdRole(r.Context(), userID, r.PathValue("household"))
	if errors.Is(err, ledger.ErrNotMember) {
		writeError(w, http.StatusNotFound, "not found")
		return "", "", false
	}
	if err != nil {
		s.writeStoreError(w, err)
		return "", "", false
	}
	return userID, role, true
}

// POST /households
func (s *Server) createHousehold(w http.ResponseWriter, r *http.Request) {
	userID, ok := s.requireUser(w, r)
	if !ok {
		return
	}
	var req struct {
		Name string `json:"name"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	householdID, ledgerID, err := s.store.CreateHousehold(r.Context(), userID, req.Name, s.now())
	if err != nil {
		s.writeStoreError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]string{"id": householdID, "ledger_id": ledgerID})
}

// GET /households
func (s *Server) listHouseholds(w http.ResponseWriter, r *http.Request) {
	userID, ok := s.requireUser(w, r)
	if !ok {
		return
	}
	items, err := s.store.ListHouseholds(r.Context(), userID)
	if err != nil {
		s.writeStoreError(w, err)
		return
	}
	out := make([]householdResponse, 0, len(items))
	for _, h := range items {
		out = append(out, householdResponse{ID: h.ID, Name: h.Name, Role: h.Role})
	}
	writeJSON(w, http.StatusOK, out)
}

// POST /households/{household}/members
// Only the owner can add members. The request names the user by username.
func (s *Server) addHouseholdMember(w http.ResponseWriter, r *http.Request) {
	_, role, ok := s.requireHouseholdMember(w, r)
	if !ok {
		return
	}
	if role != "owner" {
		writeError(w, http.StatusForbidden, "only the owner can add members")
		return
	}
	var req struct {
		Username string `json:"username"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	memberID, err := s.accounts.FindUserID(r.Context(), req.Username)
	if errors.Is(err, auth.ErrUserNotFound) {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	if err != nil {
		log.Printf("find user: %v", err)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	err = s.store.AddHouseholdMember(r.Context(), r.PathValue("household"), memberID)
	if errors.Is(err, ledger.ErrAlreadyMember) {
		writeError(w, http.StatusConflict, err.Error())
		return
	}
	if err != nil {
		s.writeStoreError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// DELETE /households/{household}/members/{user}
// Members can leave by removing themselves. Only the owner can remove others.
func (s *Server) removeHouseholdMember(w http.ResponseWriter, r *http.Request) {
	userID, role, ok := s.requireHouseholdMember(w, r)
	if !ok {
		return
	}
	target := r.PathValue("user")
	if target != userID && role != "owner" {
		writeError(w, http.StatusForbidden, "only the owner can remove other members")
		return
	}
	err := s.store.LeaveHousehold(r.Context(), r.PathValue("household"), target)
	if errors.Is(err, ledger.ErrMemberNotFound) {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	if err != nil {
		s.writeStoreError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// deletedTransactionResponse is a soft-deleted transaction that can be restored.
type deletedTransactionResponse struct {
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
	DeletedAt        string `json:"deleted_at"`
	DeletedBy        string `json:"deleted_by"`
	BookedByName     string `json:"booked_by_name"`
	DeletedByName    string `json:"deleted_by_name"`
}

// GET /ledgers/{ledger}/transactions/deleted
func (s *Server) listDeletedTransactions(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.requireLedgerAccess(w, r); !ok {
		return
	}
	items, err := s.store.ListDeletedTransactions(r.Context(), r.PathValue("ledger"), s.now())
	if err != nil {
		s.writeStoreError(w, err)
		return
	}
	out := make([]deletedTransactionResponse, 0, len(items))
	for _, d := range items {
		out = append(out, deletedTransactionResponse{
			ID:               d.ID,
			LedgerID:         d.LedgerID,
			UserID:           d.UserID,
			Type:             string(d.Type),
			CategoryID:       d.CategoryID,
			PotID:            d.PotID,
			CounterpartPotID: d.CounterpartPotID,
			AmountCents:      d.AmountCents,
			BookedOn:         d.BookedOn,
			Note:             d.Note,
			DeletedAt:        d.DeletedAt,
			DeletedBy:        d.DeletedBy,
			BookedByName:     d.BookedByName,
			DeletedByName:    d.DeletedByName,
		})
	}
	writeJSON(w, http.StatusOK, out)
}

// POST /ledgers/{ledger}/transactions/{id}/restore
func (s *Server) restoreTransaction(w http.ResponseWriter, r *http.Request) {
	userID, ok := s.requireLedgerAccess(w, r)
	if !ok {
		return
	}
	err := s.authorizeLegChange(r.Context(), userID, r.PathValue("ledger"), r.PathValue("id"), ledger.ActionDelete)
	if err == nil {
		err = s.store.RestoreTransaction(r.Context(), r.PathValue("ledger"), r.PathValue("id"), s.now())
	}
	if err != nil {
		s.writeStoreError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
