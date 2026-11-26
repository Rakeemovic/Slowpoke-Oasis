package api

import (
	"encoding/json"
	"net/http"
	"testing"
)

func TestPurposeReportOverHTTP(t *testing.T) {
	h := newTestHandler(t)
	rec := do(t, h, http.MethodPost, "/ledgers/l1/categories", "u1", map[string]any{"name": "Cinema", "purpose": "want"})
	if rec.Code != http.StatusCreated {
		t.Fatalf("create category: status %d, body %s", rec.Code, rec.Body)
	}
	cinema := decodeID(t, rec.Body.Bytes())
	rec = do(t, h, http.MethodPost, "/ledgers/l1/categories", "u1", map[string]any{"name": "Salary", "is_income": true})
	if rec.Code != http.StatusCreated {
		t.Fatalf("create income category: status %d, body %s", rec.Code, rec.Body)
	}
	salary := decodeID(t, rec.Body.Bytes())
	for _, in := range []map[string]any{
		{"type": "income", "category_id": salary, "amount_cents": 20000, "booked_on": "2026-11-01"},
		{"type": "expense", "category_id": cinema, "amount_cents": 3000, "booked_on": "2026-11-05", "is_spontaneous": true},
		{"type": "expense", "category_id": "c-food", "amount_cents": 1000, "booked_on": "2026-10-31"},
	} {
		if rec := do(t, h, http.MethodPost, "/ledgers/l1/transactions", "u1", in); rec.Code != http.StatusCreated {
			t.Fatalf("book %v: status %d, body %s", in, rec.Code, rec.Body)
		}
	}

	rec = do(t, h, http.MethodGet, "/ledgers/l1/purpose-report?date=2026-11-16", "u1", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("report: status %d, body %s", rec.Code, rec.Body)
	}
	var got struct {
		Month         string `json:"month"`
		CurrentMonth  string `json:"current_month"`
		PreviousMonth string `json:"previous_month"`
		From          string `json:"from"`
		To            string `json:"to"`
		IncomeCents   int64  `json:"income_cents"`
		Lines         []struct {
			Purpose          string `json:"purpose"`
			Cents            int64  `json:"cents"`
			SpontaneousCents int64  `json:"spontaneous_cents"`
			PreviousCents    int64  `json:"previous_cents"`
			SharePercent     *int   `json:"share_percent"`
		} `json:"lines"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if got.Month != "2026-11" || got.CurrentMonth != "2026-11" || got.PreviousMonth != "2026-10" ||
		got.From != "2026-11-01" || got.To != "2026-11-30" || got.IncomeCents != 20000 {
		t.Fatalf("report = %+v", got)
	}
	order := []string{"need", "want", "saving", "none"}
	if len(got.Lines) != len(order) {
		t.Fatalf("lines = %+v, want %v in this order", got.Lines, order)
	}
	for i, p := range order {
		if got.Lines[i].Purpose != p {
			t.Errorf("line %d is %q, want %q", i, got.Lines[i].Purpose, p)
		}
	}
	want, none := got.Lines[1], got.Lines[3]
	if want.Cents != 3000 || want.SpontaneousCents != 3000 || want.SharePercent == nil || *want.SharePercent != 15 {
		t.Errorf("want = %+v, want 3000, all spontaneous, 15 %% of the income", want)
	}
	if none.Cents != 0 || none.PreviousCents != 1000 {
		t.Errorf("none = %+v, want 0 now and the 1000 of October before", none)
	}

	if rec := do(t, h, http.MethodGet, "/ledgers/l1/purpose-report?month=2026-13", "u1", nil); rec.Code != http.StatusBadRequest {
		t.Errorf("bad month: status %d, want 400", rec.Code)
	}
}

func TestPurposeReportOfForeignLedgerIsNotFound(t *testing.T) {
	h := newTestHandler(t)
	_, aliceToken := register(t, h, "alice")
	_, bobToken := register(t, h, "bob")
	me := decodeMe(t, doToken(t, h, http.MethodGet, "/me", aliceToken, nil).Body.Bytes())

	rec := doToken(t, h, http.MethodGet, "/ledgers/"+*me.PersonalLedgerID+"/purpose-report?date=2026-11-16", bobToken, nil)
	if rec.Code != http.StatusNotFound {
		t.Errorf("status %d, want 404 (body %s)", rec.Code, rec.Body)
	}
}
