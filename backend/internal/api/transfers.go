package api

import (
	"net/http"

	"github.com/Rakeemovic/slowpoke-oasis/backend/internal/ledger"
)

// linkedTransferRequest is the JSON body for a transfer between two ledgers.
// An empty pot ID means the default pot of that ledger.
type linkedTransferRequest struct {
	FromLedgerID string `json:"from_ledger_id"`
	FromPotID    string `json:"from_pot_id"`
	ToLedgerID   string `json:"to_ledger_id"`
	ToPotID      string `json:"to_pot_id"`
	AmountCents  int64  `json:"amount_cents"`
	BookedOn     string `json:"booked_on"`
	Note         string `json:"note"`
}

// POST /transfers
// Both ledgers must be accessible to the user. A ledger the user cannot access
// answers 404, like every other ledger endpoint.
func (s *Server) createLinkedTransfer(w http.ResponseWriter, r *http.Request) {
	userID, ok := s.requireUser(w, r)
	if !ok {
		return
	}
	var req linkedTransferRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	for _, ledgerID := range []string{req.FromLedgerID, req.ToLedgerID} {
		allowed, err := s.store.CanAccessLedger(r.Context(), userID, ledgerID)
		if err != nil {
			s.writeStoreError(w, err)
			return
		}
		if !allowed {
			writeError(w, http.StatusNotFound, "not found")
			return
		}
	}

	outID, inID, err := s.store.CreateLinkedTransfer(r.Context(), ledger.LinkedTransfer{
		UserID:       userID,
		FromLedgerID: req.FromLedgerID,
		FromPotID:    req.FromPotID,
		ToLedgerID:   req.ToLedgerID,
		ToPotID:      req.ToPotID,
		AmountCents:  req.AmountCents,
		BookedOn:     req.BookedOn,
		Note:         req.Note,
	}, s.now())
	if err != nil {
		s.writeStoreError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]string{"out_id": outID, "in_id": inID})
}
