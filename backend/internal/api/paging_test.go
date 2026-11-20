package api

import (
	"encoding/json"
	"net/http"
	"testing"
)

// createExpenses books n expenses in ledger l1 and returns their ids.
func createExpenses(t *testing.T, h http.Handler, token string, n int) []string {
	t.Helper()
	ids := make([]string, 0, n)
	for i := 0; i < n; i++ {
		rec := doToken(t, h, http.MethodPost, "/ledgers/l1/transactions", token, map[string]any{
			"type": "expense", "category_id": "c-food", "amount_cents": 500, "booked_on": "2026-11-16",
		})
		if rec.Code != http.StatusCreated {
			t.Fatalf("create: status %d, body %s", rec.Code, rec.Body)
		}
		var created struct{ ID string }
		if err := json.Unmarshal(rec.Body.Bytes(), &created); err != nil {
			t.Fatalf("decode create: %v", err)
		}
		ids = append(ids, created.ID)
	}
	return ids
}

// countItems fetches a list endpoint and returns how many entries it sent.
func countItems(t *testing.T, h http.Handler, token, path string) int {
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

// expectTwoItemPaging checks limit and offset on a list that holds two entries.
func expectTwoItemPaging(t *testing.T, h http.Handler, token, path string) {
	t.Helper()
	cases := []struct {
		query string
		want  int
	}{
		{"?limit=1", 1},
		{"?limit=1&offset=1", 1},
		{"?limit=1&offset=2", 0},
		{"", 2},
	}
	for _, c := range cases {
		if got := countItems(t, h, token, path+c.query); got != c.want {
			t.Errorf("%s%s: got %d items, want %d", path, c.query, got, c.want)
		}
	}
}

func TestTransactionListPaging(t *testing.T) {
	h := newTestHandler(t)
	token := testTokens["u1"]
	createExpenses(t, h, token, 2)
	expectTwoItemPaging(t, h, token, "/ledgers/l1/transactions")
}

func TestDeletedTransactionListPaging(t *testing.T) {
	h := newTestHandler(t)
	token := testTokens["u1"]
	for _, id := range createExpenses(t, h, token, 2) {
		rec := doToken(t, h, http.MethodDelete, "/ledgers/l1/transactions/"+id, token, nil)
		if rec.Code != http.StatusNoContent {
			t.Fatalf("delete: status %d, body %s", rec.Code, rec.Body)
		}
	}
	expectTwoItemPaging(t, h, token, "/ledgers/l1/transactions/deleted")
}

func TestListsRejectBadPaging(t *testing.T) {
	h := newTestHandler(t)
	token := testTokens["u1"]
	for _, path := range []string{"/ledgers/l1/transactions", "/ledgers/l1/transactions/deleted"} {
		for _, query := range []string{"limit=0", "limit=501", "limit=abc", "offset=-1"} {
			rec := doToken(t, h, http.MethodGet, path+"?"+query, token, nil)
			if rec.Code != http.StatusBadRequest {
				t.Errorf("%s?%s: status %d, want 400", path, query, rec.Code)
			}
		}
	}
}
