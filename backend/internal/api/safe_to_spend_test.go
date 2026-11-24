package api

import (
	"encoding/json"
	"net/http"
	"testing"
)

func TestSafeToSpendOverHTTP(t *testing.T) {
	h := newTestHandler(t)
	rec := do(t, h, http.MethodPost, "/ledgers/l1/recurring-payments?date=2026-11-16", "u1", map[string]any{
		"name": "Rent", "amount_cents": 80000, "category_id": "c-food",
		"interval_months": 1, "due_day": 20, "first_month": "2026-11",
	})
	if rec.Code != http.StatusCreated {
		t.Fatalf("create payment: status %d, body %s", rec.Code, rec.Body)
	}

	rec = do(t, h, http.MethodGet, "/ledgers/l1/safe-to-spend?date=2026-11-16", "u1", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("safe to spend: status %d, body %s", rec.Code, rec.Body)
	}
	var got map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	want := map[string]any{
		"as_of": "2026-11-16", "period_end": "2026-11-30", "days_left": 15.0,
		"main_pot_cents": 0.0, "recurring_cents": 80000.0, "recurring_count": 1.0,
		"reserve_rates_cents": 0.0, "budget_left_cents": 0.0,
		"amount_cents": -80000.0, "per_day_cents": 0.0,
	}
	for key, value := range want {
		if got[key] != value {
			t.Errorf("%s = %v, want %v", key, got[key], value)
		}
	}
}

func TestSafeToSpendErrorStatuses(t *testing.T) {
	h := newTestHandler(t)
	cases := []struct {
		name string
		path string
		user string
		want int
	}{
		{"invalid date", "/ledgers/l1/safe-to-spend?date=16.11.2026", "u1", http.StatusBadRequest},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if rec := do(t, h, http.MethodGet, tc.path, tc.user, nil); rec.Code != tc.want {
				t.Errorf("status = %d, want %d (body %s)", rec.Code, tc.want, rec.Body)
			}
		})
	}
}

func TestSafeToSpendOfForeignLedgerIsNotFound(t *testing.T) {
	h := newTestHandler(t)
	_, aliceToken := register(t, h, "alice")
	_, bobToken := register(t, h, "bob")
	alice := decodeMe(t, doToken(t, h, http.MethodGet, "/me", aliceToken, nil).Body.Bytes())

	rec := doToken(t, h, http.MethodGet, "/ledgers/"+*alice.PersonalLedgerID+"/safe-to-spend", bobToken, nil)
	if rec.Code != http.StatusNotFound {
		t.Errorf("status %d, want 404", rec.Code)
	}
}
