package api

import (
	"net/http"

	"github.com/Rakeemovic/slowpoke-oasis/backend/internal/ledger"
)

// reconciliationResponse is the last reconciliation of an account, or null.
type reconciliationResponse struct {
	ReconciledOn string `json:"reconciled_on"`
	ActualCents  int64  `json:"actual_cents"`
}

func newReconciliationResponse(r *ledger.Reconciliation) *reconciliationResponse {
	if r == nil {
		return nil
	}
	return &reconciliationResponse{ReconciledOn: r.ReconciledOn, ActualCents: r.ActualCents}
}

// reconcileResponse compares the real balance with the ledger; its fields
// match ledger.ReconcileResult.
type reconcileResponse struct {
	PotID           string `json:"pot_id"`
	ExpectedCents   int64  `json:"expected_cents"`
	ActualCents     int64  `json:"actual_cents"`
	DifferenceCents int64  `json:"difference_cents"`
	CorrectionID    string `json:"correction_id,omitempty"`
}

// POST /ledgers/{ledger}/reconciliations?date=YYYY-MM-DD
// Body: {"actual_cents": N, "pot_id": "…" (optional), "correct": true}.
// Compares the real balance of the bank account (no pot_id) or of a separate
// account with the ledger on date, the app's local today, and keeps it as the
// last reconciliation. With correct, the difference is booked as a
// correction. Only the owner of the ledger may do this (rules L9, X22).
func (s *Server) reconcile(w http.ResponseWriter, r *http.Request) {
	userID, ok := s.requireLedgerOwner(w, r)
	if !ok {
		return
	}
	var req struct {
		PotID       string `json:"pot_id"`
		ActualCents *int64 `json:"actual_cents"`
		Correct     bool   `json:"correct"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	if req.ActualCents == nil {
		fail(w, ledger.ErrInvalidBalance)
		return
	}
	today, ok := s.today(w, r)
	if !ok {
		return
	}
	got, err := s.store.Reconcile(r.Context(), ledger.ReconcileInput{
		LedgerID:    r.PathValue("ledger"),
		UserID:      userID,
		PotID:       req.PotID,
		ActualCents: *req.ActualCents,
		Today:       today,
		Correct:     req.Correct,
	}, s.now())
	if err != nil {
		fail(w, err)
		return
	}
	writeJSON(w, http.StatusOK, reconcileResponse(got))
}
