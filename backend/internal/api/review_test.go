package api

import (
	"encoding/json"
	"net/http"
	"testing"
)

func TestWeeklyReviewOverHTTP(t *testing.T) {
	h := newTestHandler(t)
	_, token := register(t, h, "alice")
	me := decodeMe(t, doToken(t, h, http.MethodGet, "/me", token, nil).Body.Bytes())
	if me.ReviewWeekday != 1 {
		t.Errorf("review weekday = %d, want Monday by default", me.ReviewWeekday)
	}
	base := "/ledgers/" + *me.PersonalLedgerID + "/weekly-review"

	// 2026-11-26 is a Thursday.
	if rec := doToken(t, h, http.MethodPut, "/me/review-weekday", token, map[string]any{"weekday": 4}); rec.Code != http.StatusNoContent {
		t.Fatalf("set weekday: status %d, body %s", rec.Code, rec.Body)
	}
	if got := decodeMe(t, doToken(t, h, http.MethodGet, "/me", token, nil).Body.Bytes()).ReviewWeekday; got != 4 {
		t.Errorf("review weekday = %d, want 4", got)
	}

	booking := map[string]any{"type": "expense", "category_id": firstExpenseCategory(t, h, token, *me.PersonalLedgerID),
		"amount_cents": 1000, "booked_on": "2026-11-25"}
	if rec := doToken(t, h, http.MethodPost, "/ledgers/"+*me.PersonalLedgerID+"/transactions", token, booking); rec.Code != http.StatusCreated {
		t.Fatalf("book: status %d, body %s", rec.Code, rec.Body)
	}

	rec := doToken(t, h, http.MethodGet, base+"?date=2026-11-26", token, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("review: status %d, body %s", rec.Code, rec.Body)
	}
	var got struct {
		Due          bool   `json:"due"`
		CoveredFrom  string `json:"covered_from"`
		CoveredUntil string `json:"covered_until"`
		Lines        []any  `json:"lines"`
		Wants        []any  `json:"wants"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if !got.Due || got.CoveredFrom != "2026-11-19" || got.CoveredUntil != "2026-11-25" || len(got.Lines) != 4 || got.Wants == nil {
		t.Fatalf("review = %s", rec.Body)
	}

	if rec := doToken(t, h, http.MethodPost, base+"?date=2026-11-26", token, nil); rec.Code != http.StatusCreated {
		t.Fatalf("complete: status %d, body %s", rec.Code, rec.Body)
	}
	if rec := doToken(t, h, http.MethodPost, base+"?date=2026-11-26", token, nil); rec.Code != http.StatusConflict {
		t.Errorf("complete again: status %d, want 409", rec.Code)
	}
	rec = doToken(t, h, http.MethodGet, base+"?date=2026-11-26", token, nil)
	if body := rec.Body.String(); body != "{\"due\":false}\n" {
		t.Errorf("after completing: %s", body)
	}
}

func firstExpenseCategory(t *testing.T, h http.Handler, token, ledgerID string) string {
	t.Helper()
	rec := doToken(t, h, http.MethodGet, "/ledgers/"+ledgerID+"/categories", token, nil)
	var categories []struct {
		ID       string `json:"id"`
		IsIncome bool   `json:"is_income"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &categories); err != nil {
		t.Fatalf("categories: %v (%s)", err, rec.Body)
	}
	for _, c := range categories {
		if !c.IsIncome {
			return c.ID
		}
	}
	t.Fatal("no expense category")
	return ""
}

func TestSetReviewWeekdayRejectsOtherNumbers(t *testing.T) {
	h := newTestHandler(t)
	_, token := register(t, h, "alice")
	for _, day := range []int{0, 8} {
		rec := doToken(t, h, http.MethodPut, "/me/review-weekday", token, map[string]any{"weekday": day})
		if rec.Code != http.StatusBadRequest {
			t.Errorf("weekday %d: status %d, want 400", day, rec.Code)
		}
	}
}

func TestWeeklyReviewOfForeignLedgerIsNotFound(t *testing.T) {
	h := newTestHandler(t)
	_, aliceToken := register(t, h, "alice")
	_, bobToken := register(t, h, "bob")
	me := decodeMe(t, doToken(t, h, http.MethodGet, "/me", aliceToken, nil).Body.Bytes())
	for _, method := range []string{http.MethodGet, http.MethodPost} {
		rec := doToken(t, h, method, "/ledgers/"+*me.PersonalLedgerID+"/weekly-review?date=2026-11-26", bobToken, nil)
		if rec.Code != http.StatusNotFound {
			t.Errorf("%s: status %d, want 404 (body %s)", method, rec.Code, rec.Body)
		}
	}
}
