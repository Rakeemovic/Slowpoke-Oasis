package api

import (
	"encoding/json"
	"net/http"
	"testing"
)

func TestBudgetOverHTTP(t *testing.T) {
	h := newTestHandler(t)
	rec := do(t, h, http.MethodPost, "/ledgers/l1/recurring-payments?date=2026-11-16", "u1", map[string]any{
		"name": "Rent", "amount_cents": 80000, "category_id": "c-food",
		"interval_months": 1, "due_day": 20, "first_month": "2026-11",
	})
	if rec.Code != http.StatusCreated {
		t.Fatalf("create payment: status %d, body %s", rec.Code, rec.Body)
	}
	rec = do(t, h, http.MethodPut, "/ledgers/l1/pots/p-default/lines/c-food?date=2026-11-16", "u1",
		map[string]any{"amount_cents": 5000, "scope": "this_month"})
	if rec.Code != http.StatusNoContent {
		t.Fatalf("set plan: status %d, body %s", rec.Code, rec.Body)
	}

	rec = do(t, h, http.MethodGet, "/ledgers/l1/budget?month=2026-11&date=2026-11-16", "u1", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("budget: status %d, body %s", rec.Code, rec.Body)
	}
	var b struct {
		PotID        string `json:"pot_id"`
		Month        string `json:"month"`
		CurrentMonth string `json:"current_month"`
		PlanSource   string `json:"plan_source"`
		Categories   []struct {
			CategoryID   string `json:"category_id"`
			PlannedCents int64  `json:"planned_cents"`
			Recurring    []struct {
				DueOn        string `json:"due_on"`
				PlannedCents int64  `json:"planned_cents"`
				Skipped      bool   `json:"skipped"`
			} `json:"recurring"`
			Reserves []any `json:"reserves"`
		} `json:"categories"`
		ReserveDues []any `json:"reserve_dues"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &b); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if b.PotID != "p-default" || b.Month != "2026-11" || b.CurrentMonth != "2026-11" || b.PlanSource != "month" {
		t.Errorf("header = %+v", b)
	}
	if len(b.Categories) != 1 || b.Categories[0].PlannedCents != 5000 || b.Categories[0].Reserves == nil {
		t.Fatalf("categories = %+v, want groceries with plan and empty reserves", b.Categories)
	}
	if r := b.Categories[0].Recurring; len(r) != 1 || r[0].DueOn != "2026-11-20" || r[0].PlannedCents != 80000 || r[0].Skipped {
		t.Errorf("recurring = %+v, want rent due on 2026-11-20", r)
	}
	if b.ReserveDues == nil {
		t.Error("reserve_dues is null, want []")
	}
}

func TestBudgetErrorStatuses(t *testing.T) {
	h := newTestHandler(t)
	cases := []struct {
		name string
		path string
		user string
		want int
	}{
		{"invalid month", "/ledgers/l1/budget?month=2026-13&date=2026-11-16", "u1", http.StatusBadRequest},
		{"invalid date", "/ledgers/l1/budget?date=16.11.2026", "u1", http.StatusBadRequest},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if rec := do(t, h, http.MethodGet, tc.path, tc.user, nil); rec.Code != tc.want {
				t.Errorf("status = %d, want %d (body %s)", rec.Code, tc.want, rec.Body)
			}
		})
	}
}

func TestBudgetOfForeignLedgerIsNotFound(t *testing.T) {
	h := newTestHandler(t)
	_, aliceToken := register(t, h, "alice")
	_, bobToken := register(t, h, "bob")
	alice := decodeMe(t, doToken(t, h, http.MethodGet, "/me", aliceToken, nil).Body.Bytes())

	rec := doToken(t, h, http.MethodGet, "/ledgers/"+*alice.PersonalLedgerID+"/budget?date=2026-11-16", bobToken, nil)
	if rec.Code != http.StatusNotFound {
		t.Errorf("status %d, want 404", rec.Code)
	}
}
