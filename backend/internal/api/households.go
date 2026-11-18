package api

import (
	"net/http"

	"github.com/Rakeemovic/slowpoke-oasis/backend/internal/ledger"
)

// householdResponse is one household of the logged-in user.
type householdResponse struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Role string `json:"role"`
}

// memberResponse is one member of a household. Other members only see the
// username and the role; the user ID is needed to address the member in
// other requests.
type memberResponse struct {
	UserID   string `json:"user_id"`
	Username string `json:"username"`
	Role     string `json:"role"`
}

// requireHouseholdMember checks that the logged-in user belongs to the household
// in the path. It writes 404 for unknown households and for households of other
// people alike, so their IDs are not revealed. It returns the user ID and the
// user's role in the household.
func (s *Server) requireHouseholdMember(w http.ResponseWriter, r *http.Request) (string, string, bool) {
	userID, ok := s.requireUser(w, r)
	if !ok {
		return "", "", false
	}
	role, err := s.store.HouseholdRole(r.Context(), userID, r.PathValue("household"))
	if err != nil {
		fail(w, err)
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
		fail(w, err)
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
		fail(w, err)
		return
	}
	out := make([]householdResponse, 0, len(items))
	for _, h := range items {
		out = append(out, householdResponse{ID: h.ID, Name: h.Name, Role: h.Role})
	}
	writeJSON(w, http.StatusOK, out)
}

// GET /households/{household}/members
// Every member may see who else belongs to the household.
func (s *Server) listHouseholdMembers(w http.ResponseWriter, r *http.Request) {
	if _, _, ok := s.requireHouseholdMember(w, r); !ok {
		return
	}
	members, err := s.store.ListMembers(r.Context(), r.PathValue("household"))
	if err != nil {
		fail(w, err)
		return
	}
	out := make([]memberResponse, 0, len(members))
	for _, m := range members {
		out = append(out, memberResponse{UserID: m.UserID, Username: m.Username, Role: m.Role})
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
	if role != ledger.RoleOwner {
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
	if err == nil {
		err = s.store.AddHouseholdMember(r.Context(), r.PathValue("household"), memberID)
	}
	if err != nil {
		fail(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// PUT /households/{household}/owner/{user}
// The owner hands the role to another member and becomes a regular member.
func (s *Server) transferHouseholdOwnership(w http.ResponseWriter, r *http.Request) {
	userID, _, ok := s.requireHouseholdMember(w, r)
	if !ok {
		return
	}
	// The store checks the owner role itself, inside its transaction.
	err := s.store.TransferOwnership(r.Context(), r.PathValue("household"), userID, r.PathValue("user"))
	if err != nil {
		fail(w, err)
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
	if target != userID && role != ledger.RoleOwner {
		writeError(w, http.StatusForbidden, "only the owner can remove other members")
		return
	}
	if err := s.store.LeaveHousehold(r.Context(), r.PathValue("household"), target); err != nil {
		fail(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
