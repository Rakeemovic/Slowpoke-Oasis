package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Rakeemovic/slowpoke-oasis/backend/internal/ledger"
	"github.com/Rakeemovic/slowpoke-oasis/backend/internal/store"
)

// newTestHandler returns the routes backed by an in-memory database with one
// user, one ledger, a default pot, a second pot and one category.
func newTestHandler(t *testing.T) http.Handler {
	t.Helper()
	db, err := store.Open(":memory:")
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { db.Close() })

	seed := []string{
		`INSERT INTO user (id, name, created_at) VALUES ('u1', 'Alice', '2026-01-01T00:00:00Z')`,
		`INSERT INTO ledger (id, name, user_id, created_at) VALUES ('l1', 'Personal', 'u1', '2026-01-01T00:00:00Z')`,
		`INSERT INTO pot (id, ledger_id, name, kind, is_default) VALUES ('p-default', 'l1', 'Monthly', 'monthly', 1)`,
		`INSERT INTO pot (id, ledger_id, name, kind) VALUES ('p-holiday', 'l1', 'Holiday', 'one_time')`,
		`INSERT INTO category (id, ledger_id, name) VALUES ('c-food', 'l1', 'Groceries')`,
	}
	for _, q := range seed {
		if _, err := db.Exec(q); err != nil {
			t.Fatalf("seed %q: %v", q, err)
		}
	}
	return NewServer(ledger.New(db)).Routes()
}

// do sends a request through the handler. An empty user means no X-User-ID header.
func do(t *testing.T, h http.Handler, method, path, user string, body any) *httptest.ResponseRecorder {
	t.Helper()
	var buf bytes.Buffer
	if body != nil {
		if err := json.NewEncoder(&buf).Encode(body); err != nil {
			t.Fatalf("encode body: %v", err)
		}
	}
	req := httptest.NewRequest(method, path, &buf)
	req.Header.Set("Content-Type", "application/json")
	if user != "" {
		req.Header.Set("X-User-ID", user)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func expenseBody() map[string]any {
	return map[string]any{
		"type":         "expense",
		"category_id":  "c-food",
		"amount_cents": 1250,
		"booked_on":    "2026-11-16",
	}
}

func TestCreateAndListTransaction(t *testing.T) {
	h := newTestHandler(t)

	rec := do(t, h, http.MethodPost, "/ledgers/l1/transactions", "u1", expenseBody())
	if rec.Code != http.StatusCreated {
		t.Fatalf("create: status %d, body %s", rec.Code, rec.Body)
	}

	rec = do(t, h, http.MethodGet, "/ledgers/l1/transactions", "u1", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("list: status %d, body %s", rec.Code, rec.Body)
	}
	var items []map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &items); err != nil {
		t.Fatalf("decode list: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("got %d items, want 1", len(items))
	}
	if items[0]["amount_cents"] != float64(-1250) {
		t.Errorf("amount_cents = %v, want -1250", items[0]["amount_cents"])
	}
	if items[0]["pot_id"] != "p-default" {
		t.Errorf("pot_id = %v, want p-default", items[0]["pot_id"])
	}
}

func TestEmptyListReturnsJSONArray(t *testing.T) {
	h := newTestHandler(t)

	rec := do(t, h, http.MethodGet, "/ledgers/l1/transactions", "u1", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d", rec.Code)
	}
	if got := rec.Body.String(); got != "[]\n" {
		t.Errorf("body = %q, want an empty JSON array", got)
	}
}

func TestDeleteRemovesTransactionFromList(t *testing.T) {
	h := newTestHandler(t)

	rec := do(t, h, http.MethodPost, "/ledgers/l1/transactions", "u1", expenseBody())
	var created map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &created); err != nil {
		t.Fatalf("decode create: %v", err)
	}

	rec = do(t, h, http.MethodDelete, "/ledgers/l1/transactions/"+created["id"], "u1", nil)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("delete: status %d, body %s", rec.Code, rec.Body)
	}

	rec = do(t, h, http.MethodGet, "/ledgers/l1/transactions", "u1", nil)
	if got := rec.Body.String(); got != "[]\n" {
		t.Errorf("after delete, body = %q, want empty list", got)
	}
}

func TestErrorStatuses(t *testing.T) {
	h := newTestHandler(t)

	cases := []struct {
		name   string
		method string
		path   string
		user   string
		body   any
		want   int
	}{
		{
			name: "missing user header", method: http.MethodPost,
			path: "/ledgers/l1/transactions", user: "", body: expenseBody(),
			want: http.StatusUnauthorized,
		},
		{
			name: "unknown JSON field", method: http.MethodPost,
			path: "/ledgers/l1/transactions", user: "u1",
			body: map[string]any{"type": "expense", "unknown": 1},
			want: http.StatusBadRequest,
		},
		{
			name: "expense without category", method: http.MethodPost,
			path: "/ledgers/l1/transactions", user: "u1",
			body: map[string]any{"type": "expense", "amount_cents": 5, "booked_on": "2026-11-16"},
			want: http.StatusBadRequest,
		},
		{
			name: "category from another ledger or missing", method: http.MethodPost,
			path: "/ledgers/l1/transactions", user: "u1",
			body: map[string]any{"type": "expense", "category_id": "c-missing", "amount_cents": 5, "booked_on": "2026-11-16"},
			want: http.StatusUnprocessableEntity,
		},
		{
			name: "unknown ledger", method: http.MethodPost,
			path: "/ledgers/nope/transactions", user: "u1", body: expenseBody(),
			want: http.StatusNotFound,
		},
		{
			name: "delete unknown transaction", method: http.MethodDelete,
			path: "/ledgers/l1/transactions/does-not-exist", user: "u1",
			want: http.StatusNotFound,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rec := do(t, h, tc.method, tc.path, tc.user, tc.body)
			if rec.Code != tc.want {
				t.Errorf("status = %d, want %d (body %s)", rec.Code, tc.want, rec.Body)
			}
		})
	}
}
