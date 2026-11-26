package api

import (
	"encoding/json"
	"net/http"
	"testing"
)

func TestRoundUpOverHTTP(t *testing.T) {
	h := newTestHandler(t)
	rec := do(t, h, http.MethodPost, "/ledgers/l1/pots", "u1", map[string]any{"name": "Bike"})
	if rec.Code != http.StatusCreated {
		t.Fatalf("create pot: status %d, body %s", rec.Code, rec.Body)
	}
	bike := decodeID(t, rec.Body.Bytes())

	rec = do(t, h, http.MethodPatch, "/ledgers/l1/pots/"+bike+"?date=2026-11-01", "u1", map[string]any{"round_up_multiplier": 3})
	if rec.Code != http.StatusNoContent {
		t.Fatalf("switch on: status %d, body %s", rec.Code, rec.Body)
	}
	var pots []potResponse
	rec = do(t, h, http.MethodGet, "/ledgers/l1/pots", "u1", nil)
	if err := json.Unmarshal(rec.Body.Bytes(), &pots); err != nil {
		t.Fatal(err)
	}
	for _, p := range pots {
		if p.ID == bike && (p.RoundUpMultiplier != 3 || p.RoundUpFrom != "2026-11-01") {
			t.Errorf("pot = %+v, want x3 from 2026-11-01", p)
		}
	}

	// 9,75 € rounds up by 25 cents, times 3.
	rec = do(t, h, http.MethodPost, "/ledgers/l1/transactions", "u1", map[string]any{
		"type": "expense", "category_id": "c-food", "amount_cents": 975, "booked_on": "2026-11-16",
	})
	if rec.Code != http.StatusCreated {
		t.Fatalf("expense: status %d, body %s", rec.Code, rec.Body)
	}
	expense := decodeID(t, rec.Body.Bytes())
	var list []transactionResponse
	rec = do(t, h, http.MethodGet, "/ledgers/l1/transactions", "u1", nil)
	if err := json.Unmarshal(rec.Body.Bytes(), &list); err != nil {
		t.Fatal(err)
	}
	var roundUp *transactionResponse
	for i := range list {
		if list[i].RoundUpOf == expense {
			roundUp = &list[i]
		}
	}
	if roundUp == nil || roundUp.Type != "transfer" || roundUp.AmountCents != 75 || roundUp.CounterpartPotID != bike {
		t.Fatalf("list = %+v, want a transfer of 75 into Bike that rounds up the expense", list)
	}

	// It belongs to the expense.
	if rec := do(t, h, http.MethodDelete, "/ledgers/l1/transactions/"+roundUp.ID, "u1", nil); rec.Code != http.StatusConflict {
		t.Errorf("delete the round-up: status %d, want 409 (body %s)", rec.Code, rec.Body)
	}
	if rec := do(t, h, http.MethodPatch, "/ledgers/l1/transactions/"+roundUp.ID, "u1", map[string]any{"amount_cents": 10}); rec.Code != http.StatusConflict {
		t.Errorf("change the round-up: status %d, want 409 (body %s)", rec.Code, rec.Body)
	}
	if rec := do(t, h, http.MethodDelete, "/ledgers/l1/transactions/"+expense, "u1", nil); rec.Code != http.StatusNoContent {
		t.Fatalf("delete the expense: status %d, body %s", rec.Code, rec.Body)
	}
	rec = do(t, h, http.MethodGet, "/ledgers/l1/transactions", "u1", nil)
	if err := json.Unmarshal(rec.Body.Bytes(), &list); err != nil || len(list) != 0 {
		t.Errorf("after deleting the expense: %+v (%v), want an empty list", list, err)
	}
}

func TestRoundUpErrorStatuses(t *testing.T) {
	h := newTestHandler(t)
	cases := []struct {
		name string
		path string
		body map[string]any
		want int
	}{
		{"unknown multiplier", "/ledgers/l1/pots/p-holiday?date=2026-11-01", map[string]any{"round_up_multiplier": 4}, http.StatusBadRequest},
		{"invalid date", "/ledgers/l1/pots/p-holiday?date=1.11.2026", map[string]any{"round_up_multiplier": 1}, http.StatusBadRequest},
		{"main pot", "/ledgers/l1/pots/p-default?date=2026-11-01", map[string]any{"round_up_multiplier": 1}, http.StatusConflict},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if rec := do(t, h, http.MethodPatch, tc.path, "u1", tc.body); rec.Code != tc.want {
				t.Errorf("status = %d, want %d (body %s)", rec.Code, tc.want, rec.Body)
			}
		})
	}
}

func TestNewPotWithRoundUpAndTheAmountOnTheExpense(t *testing.T) {
	h := newTestHandler(t)
	rec := do(t, h, http.MethodPost, "/ledgers/l1/pots?date=2026-11-01", "u1", map[string]any{"name": "Bike", "round_up_multiplier": 3})
	if rec.Code != http.StatusCreated {
		t.Fatalf("create pot: status %d, body %s", rec.Code, rec.Body)
	}
	bike := decodeID(t, rec.Body.Bytes())

	var pots []potResponse
	rec = do(t, h, http.MethodGet, "/ledgers/l1/pots", "u1", nil)
	if err := json.Unmarshal(rec.Body.Bytes(), &pots); err != nil {
		t.Fatal(err)
	}
	for _, p := range pots {
		if p.ID == bike && (p.RoundUpMultiplier != 3 || p.RoundUpFrom != "2026-11-01") {
			t.Errorf("pot = %+v, want x3 from 2026-11-01", p)
		}
	}

	// 9,75 € rounds up by 25 cents, times 3; the expense names the amount.
	rec = do(t, h, http.MethodPost, "/ledgers/l1/transactions", "u1", map[string]any{
		"type": "expense", "category_id": "c-food", "amount_cents": 975, "booked_on": "2026-11-16",
	})
	expense := decodeID(t, rec.Body.Bytes())
	rec = do(t, h, http.MethodGet, "/ledgers/l1/transactions/"+expense, "u1", nil)
	var got transactionResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if got.RoundUpCents != 75 {
		t.Errorf("round_up_cents = %d, want 75 (body %s)", got.RoundUpCents, rec.Body)
	}

	// An invalid multiplier creates nothing.
	rec = do(t, h, http.MethodPost, "/ledgers/l1/pots?date=2026-11-01", "u1", map[string]any{"name": "Bad", "round_up_multiplier": 4})
	if rec.Code != http.StatusBadRequest {
		t.Errorf("multiplier 4: status %d, want 400 (body %s)", rec.Code, rec.Body)
	}
}
