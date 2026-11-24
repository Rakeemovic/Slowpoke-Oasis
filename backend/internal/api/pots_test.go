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
		{"negative planned amount", http.MethodPut, "/ledgers/l1/pots/p-default/lines/c-food",
			map[string]any{"amount_cents": -1, "scope": "this_month"}, http.StatusBadRequest},
		{"budget in a one-time pot", http.MethodPut, "/ledgers/l1/pots/p-holiday/lines/c-food",
			map[string]any{"amount_cents": 100, "scope": "this_month"}, http.StatusBadRequest},
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

func TestMonthlyPlanOverHTTP(t *testing.T) {
	h := newTestHandler(t)
	line := "/ledgers/l1/pots/p-default/lines/c-food?date=2026-11-16"

	rec := do(t, h, http.MethodPut, line, "u1", map[string]any{"amount_cents": 5000})
	if rec.Code != http.StatusBadRequest {
		t.Errorf("monthly pot without scope: status %d, want 400", rec.Code)
	}
	rec = do(t, h, http.MethodPut, line, "u1", map[string]any{"amount_cents": 5000, "scope": "from_now_on"})
	if rec.Code != http.StatusNoContent {
		t.Fatalf("set plan: status %d, body %s", rec.Code, rec.Body)
	}

	rec = do(t, h, http.MethodGet, "/ledgers/l1/pots/p-default/summary?date=2026-11-16", "u1", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("summary: status %d, body %s", rec.Code, rec.Body)
	}
	var sum struct {
		Month      string `json:"month"`
		PlanSource string `json:"plan_source"`
		Lines      []struct {
			PlannedCents int64 `json:"planned_cents"`
		} `json:"lines"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &sum); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if sum.Month != "2026-11" || sum.PlanSource != "month" || len(sum.Lines) != 1 || sum.Lines[0].PlannedCents != 5000 {
		t.Errorf("summary = %+v, want month 2026-11 from the stored plan with 5000", sum)
	}

	rec = do(t, h, http.MethodGet, "/ledgers/l1/pots/p-default/summary?from=2026-11-01", "u1", nil)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("monthly pot with from: status %d, want 400", rec.Code)
	}
	rec = do(t, h, http.MethodDelete, line+"&scope=this_month", "u1", nil)
	if rec.Code != http.StatusNoContent {
		t.Errorf("remove for this month: status %d, body %s", rec.Code, rec.Body)
	}
	rec = do(t, h, http.MethodDelete, line+"&scope=this_month", "u1", nil)
	if rec.Code != http.StatusNotFound {
		t.Errorf("remove again: status %d, want 404", rec.Code)
	}
}
