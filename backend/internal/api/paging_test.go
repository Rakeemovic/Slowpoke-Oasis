package api

import (
	"encoding/json"
	"net/http"
	"testing"
)

func TestTransactionListPaging(t *testing.T) {
	h := newTestHandler(t)
	token := testTokens["u1"]
	for i := 0; i < 2; i++ {
		rec := doToken(t, h, http.MethodPost, "/ledgers/l1/transactions", token, map[string]any{
			"type": "expense", "category_id": "c-food", "amount_cents": 500, "booked_on": "2026-11-16",
		})
		if rec.Code != http.StatusCreated {
			t.Fatalf("create: status %d, body %s", rec.Code, rec.Body)
		}
	}

	count := func(path string) int {
		t.Helper()
		rec := doToken(t, h, http.MethodGet, path, token, nil)
		if rec.Code != http.StatusOK {
			t.Fatalf("%s: status %d, body %s", path, rec.Code, rec.Body)
		}
		var items []map[string]any
		if err := json.Unmarshal(rec.Body.Bytes(), &items); err != nil {
			t.Fatalf("decode %s: %v", path, err)
		}
		return len(items)
	}

	if got := count("/ledgers/l1/transactions?limit=1"); got != 1 {
		t.Errorf("limit 1: got %d items, want 1", got)
	}
	if got := count("/ledgers/l1/transactions?limit=1&offset=1"); got != 1 {
		t.Errorf("limit 1 offset 1: got %d items, want 1", got)
	}
	if got := count("/ledgers/l1/transactions?limit=1&offset=2"); got != 0 {
		t.Errorf("offset past the end: got %d items, want 0", got)
	}
	if got := count("/ledgers/l1/transactions"); got != 2 {
		t.Errorf("default page: got %d items, want 2", got)
	}
}

func TestTransactionListRejectsBadPaging(t *testing.T) {
	h := newTestHandler(t)
	token := testTokens["u1"]
	for _, query := range []string{"limit=0", "limit=501", "limit=abc", "offset=-1"} {
		rec := doToken(t, h, http.MethodGet, "/ledgers/l1/transactions?"+query, token, nil)
		if rec.Code != http.StatusBadRequest {
			t.Errorf("%s: status %d, want 400", query, rec.Code)
		}
	}
}
