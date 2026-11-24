package api

import (
	"encoding/json"
	"net/http"
	"testing"
)

func TestSavingsGoalsOverHTTP(t *testing.T) {
	h := newTestHandler(t)
	rec := do(t, h, http.MethodPost, "/ledgers/l1/pots", "u1", map[string]any{
		"name": "Bike", "target_cents": 50000, "monthly_rate_cents": 20000,
	})
	if rec.Code != http.StatusCreated {
		t.Fatalf("create pot: status %d, body %s", rec.Code, rec.Body)
	}
	var created struct{ ID string }
	if err := json.Unmarshal(rec.Body.Bytes(), &created); err != nil {
		t.Fatal(err)
	}

	rec = do(t, h, http.MethodGet, "/ledgers/l1/savings-goals?date=2026-11-16", "u1", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("savings goals: status %d, body %s", rec.Code, rec.Body)
	}
	var goals []savingsGoalResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &goals); err != nil {
		t.Fatal(err)
	}
	want := savingsGoalResponse{
		PotID: created.ID, Name: "Bike", TargetCents: 50000, MonthlyRateCents: 20000,
		RateCents: 20000, OpenCents: 20000,
	}
	if len(goals) != 1 || goals[0] != want {
		t.Errorf("goals = %+v, want [%+v]", goals, want)
	}

	// The pot list carries the rate, the budget and safe to spend the goal.
	rec = do(t, h, http.MethodGet, "/ledgers/l1/pots", "u1", nil)
	var pots []potResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &pots); err != nil {
		t.Fatal(err)
	}
	for _, p := range pots {
		if p.ID == created.ID && p.MonthlyRateCents != 20000 {
			t.Errorf("pot = %+v, want monthly_rate_cents 20000", p)
		}
	}
	var budget struct {
		Savings []savingsLineResponse `json:"savings"`
	}
	rec = do(t, h, http.MethodGet, "/ledgers/l1/budget?date=2026-11-16", "u1", nil)
	if err := json.Unmarshal(rec.Body.Bytes(), &budget); err != nil {
		t.Fatal(err)
	}
	if len(budget.Savings) != 1 || *budget.Savings[0].PlannedCents != 20000 {
		t.Errorf("budget savings = %+v, want the rate of 20000", budget.Savings)
	}
	var sts safeToSpendResponse
	rec = do(t, h, http.MethodGet, "/ledgers/l1/safe-to-spend?date=2026-11-16", "u1", nil)
	if err := json.Unmarshal(rec.Body.Bytes(), &sts); err != nil {
		t.Fatal(err)
	}
	if sts.SavingsCents != 20000 {
		t.Errorf("safe to spend savings = %d, want 20000", sts.SavingsCents)
	}
}

func TestSavingsGoalErrorStatuses(t *testing.T) {
	h := newTestHandler(t)
	cases := []struct {
		name string
		path string
		body map[string]any
		want int
	}{
		{"rate and date", "/ledgers/l1/pots/p-holiday",
			map[string]any{"target_cents": 1000, "target_date": "2027-01-01", "monthly_rate_cents": 500}, http.StatusBadRequest},
		{"negative rate", "/ledgers/l1/pots/p-holiday", map[string]any{"monthly_rate_cents": -1}, http.StatusBadRequest},
		{"rate on the main pot", "/ledgers/l1/pots/p-default", map[string]any{"monthly_rate_cents": 500}, http.StatusConflict},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if rec := do(t, h, http.MethodPatch, tc.path, "u1", tc.body); rec.Code != tc.want {
				t.Errorf("status = %d, want %d (body %s)", rec.Code, tc.want, rec.Body)
			}
		})
	}
	if rec := do(t, h, http.MethodGet, "/ledgers/l1/savings-goals?date=16.11.2026", "u1", nil); rec.Code != http.StatusBadRequest {
		t.Errorf("invalid date: status %d, want 400", rec.Code)
	}
}
