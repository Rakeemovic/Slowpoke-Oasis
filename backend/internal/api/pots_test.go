package api

import (
	"encoding/json"
	"net/http"
	"testing"
)

func TestPotsEndToEnd(t *testing.T) {
	h := newTestHandler(t)

	// Create a savings goal.
	rec := do(t, h, http.MethodPost, "/ledgers/l1/pots", "u1", map[string]any{
		"name": "Trip", "target_cents": 100000, "target_date": "2028-04-30",
	})
	if rec.Code != http.StatusCreated {
		t.Fatalf("create pot: status %d, body %s", rec.Code, rec.Body)
	}
	tripID := decodeID(t, rec.Body.Bytes())

	// Plan 50 EUR for groceries in the holiday pot.
	rec = do(t, h, http.MethodPut, "/ledgers/l1/pots/p-holiday/lines/c-food", "u1",
		map[string]any{"amount_cents": 5000})
	if rec.Code != http.StatusNoContent {
		t.Fatalf("set plan: status %d, body %s", rec.Code, rec.Body)
	}

	// The list contains the default pot, the seeded holiday pot and the new goal.
	rec = do(t, h, http.MethodGet, "/ledgers/l1/pots", "u1", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("list pots: status %d, body %s", rec.Code, rec.Body)
	}
	var pots []map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &pots); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(pots) != 3 {
		t.Fatalf("got %d pots, want 3", len(pots))
	}
	if pots[0]["is_default"] != true {
		t.Errorf("first pot should be the default pot, got %v", pots[0])
	}

	// Remove the target from the goal.
	rec = do(t, h, http.MethodPatch, "/ledgers/l1/pots/"+tripID, "u1", map[string]any{"target_cents": 0})
	if rec.Code != http.StatusNoContent {
		t.Fatalf("patch: status %d, body %s", rec.Code, rec.Body)
	}

	// Remove the plan again, then removing it a second time fails.
	rec = do(t, h, http.MethodDelete, "/ledgers/l1/pots/p-holiday/lines/c-food", "u1", nil)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("delete plan: status %d", rec.Code)
	}
	rec = do(t, h, http.MethodDelete, "/ledgers/l1/pots/p-holiday/lines/c-food", "u1", nil)
	if rec.Code != http.StatusNotFound {
		t.Errorf("second delete: status %d, want 404", rec.Code)
	}
}

func TestPotErrorStatuses(t *testing.T) {
	h := newTestHandler(t)

	cases := []struct {
		name   string
		method string
		path   string
		body   any
		want   int
	}{
		{"date without target", http.MethodPost, "/ledgers/l1/pots",
			map[string]any{"name": "X", "target_date": "2028-01-01"}, http.StatusBadRequest},
		{"negative target", http.MethodPost, "/ledgers/l1/pots",
			map[string]any{"name": "X", "target_cents": -5}, http.StatusBadRequest},
		{"archive default pot", http.MethodPatch, "/ledgers/l1/pots/p-default",
			map[string]any{"archived": true}, http.StatusConflict},
		{"negative planned amount", http.MethodPut, "/ledgers/l1/pots/p-holiday/lines/c-food",
			map[string]any{"amount_cents": -1}, http.StatusBadRequest},
		{"unknown pot", http.MethodPatch, "/ledgers/l1/pots/nope",
			map[string]any{"name": "X"}, http.StatusNotFound},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rec := do(t, h, tc.method, tc.path, "u1", tc.body)
			if rec.Code != tc.want {
				t.Errorf("status = %d, want %d (body %s)", rec.Code, tc.want, rec.Body)
			}
		})
	}
}
