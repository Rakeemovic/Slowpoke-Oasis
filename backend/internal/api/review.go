package api

import (
	"net/http"
	"time"
)

type weeklyReviewResponse struct {
	Due bool `json:"due"`
	// The remaining fields are left out when no reflection is due.
	CoveredFrom   string                 `json:"covered_from,omitempty"`
	CoveredUntil  string                 `json:"covered_until,omitempty"`
	PreviousFrom  string                 `json:"previous_from,omitempty"`
	PreviousUntil string                 `json:"previous_until,omitempty"`
	Lines         []purposeLineResponse  `json:"lines,omitempty"`
	Wants         *[]transactionResponse `json:"wants,omitempty"`
}

// GET /ledgers/{ledger}/weekly-review?date=YYYY-MM-DD
// The open reflection of the signed-in user about the ledger (rules W1, W2):
// their own spending by purpose and their own wants. date is the app's local date.
func (s *Server) weeklyReview(w http.ResponseWriter, r *http.Request) {
	userID, ok := s.requireLedgerAccess(w, r)
	if !ok {
		return
	}
	today, ok := s.today(w, r)
	if !ok {
		return
	}
	review, err := s.store.WeeklyReview(r.Context(), r.PathValue("ledger"), userID, today)
	if err != nil {
		fail(w, err)
		return
	}
	if !review.Due {
		writeJSON(w, http.StatusOK, weeklyReviewResponse{})
		return
	}
	// Wants are expenses, so none of them has a counterpart ledger.
	wants := make([]transactionResponse, 0, len(review.Wants))
	for _, t := range review.Wants {
		wants = append(wants, newTransactionResponse(t, nil))
	}
	writeJSON(w, http.StatusOK, weeklyReviewResponse{
		Due:           true,
		CoveredFrom:   review.Covered.From,
		CoveredUntil:  review.Covered.Until,
		PreviousFrom:  review.Previous.From,
		PreviousUntil: review.Previous.Until,
		Lines:         newPurposeLines(review.Lines),
		Wants:         &wants,
	})
}

// POST /ledgers/{ledger}/weekly-review?date=YYYY-MM-DD
// Completes the open reflection of the signed-in user (rule W3).
func (s *Server) completeWeeklyReview(w http.ResponseWriter, r *http.Request) {
	userID, ok := s.requireLedgerAccess(w, r)
	if !ok {
		return
	}
	today, ok := s.today(w, r)
	if !ok {
		return
	}
	covered, err := s.store.CompleteWeeklyReview(r.Context(), r.PathValue("ledger"), userID, today, time.Now())
	if err != nil {
		fail(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]string{
		"covered_from":  covered.From,
		"covered_until": covered.Until,
	})
}

// PUT /me/review-weekday
// Chooses the day of the weekly reflection for all ledgers (rule W1).
func (s *Server) setReviewWeekday(w http.ResponseWriter, r *http.Request) {
	userID, ok := s.requireUser(w, r)
	if !ok {
		return
	}
	var req struct {
		Weekday int `json:"weekday"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	if err := s.accounts.SetReviewWeekday(r.Context(), userID, req.Weekday); err != nil {
		fail(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
