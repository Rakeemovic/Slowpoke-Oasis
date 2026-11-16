package api

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"
)

// decodeField reads one string field from a JSON object response.
func decodeField(t *testing.T, body []byte, field string) string {
	t.Helper()
	var m map[string]string
	if err := json.Unmarshal(body, &m); err != nil {
		t.Fatalf("decode response %s: %v", body, err)
	}
	return m[field]
}

// decodeID is shorthand for decodeField(..., "id").
func decodeID(t *testing.T, body []byte) string {
	t.Helper()
	return decodeField(t, body, "id")
}

// register creates an account and returns its user ID and a login token.
func register(t *testing.T, h http.Handler, username string) (userID, token string) {
	t.Helper()
	rec := doToken(t, h, http.MethodPost, "/auth/register", "", map[string]any{
		"username": username, "email": username + "@example.com",
		"password": "correct horse battery", "display_name": "Bob",
	})
	if rec.Code != http.StatusCreated {
		t.Fatalf("register: status %d, body %s", rec.Code, rec.Body)
	}
	userID = decodeID(t, rec.Body.Bytes())

	rec = doToken(t, h, http.MethodPost, "/auth/login", "", map[string]any{
		"username": username, "password": "correct horse battery",
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("login: status %d, body %s", rec.Code, rec.Body)
	}
	return userID, decodeField(t, rec.Body.Bytes(), "token")
}

func TestOnboardingFlowWithLogin(t *testing.T) {
	h := newTestHandler(t)
	bobID, bobToken := register(t, h, "bob")

	rec := doToken(t, h, http.MethodPost, "/users/"+bobID+"/ledgers", bobToken,
		map[string]any{"name": "Home", "payday_day": 23})
	if rec.Code != http.StatusCreated {
		t.Fatalf("create ledger: status %d, body %s", rec.Code, rec.Body)
	}
	ledgerID := decodeID(t, rec.Body.Bytes())

	rec = doToken(t, h, http.MethodPost, "/ledgers/"+ledgerID+"/categories", bobToken,
		map[string]any{"name": "Groceries"})
	if rec.Code != http.StatusCreated {
		t.Fatalf("create category: status %d, body %s", rec.Code, rec.Body)
	}
	categoryID := decodeID(t, rec.Body.Bytes())

	rec = doToken(t, h, http.MethodPost, "/ledgers/"+ledgerID+"/transactions", bobToken,
		map[string]any{"type": "expense", "category_id": categoryID, "amount_cents": 500, "booked_on": "2026-11-16"})
	if rec.Code != http.StatusCreated {
		t.Fatalf("create transaction: status %d, body %s", rec.Code, rec.Body)
	}
}

func TestLoginFailures(t *testing.T) {
	h := newTestHandler(t)
	register(t, h, "bob")

	rec := doToken(t, h, http.MethodPost, "/auth/login", "", map[string]any{
		"username": "bob", "password": "wrong password here",
	})
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("wrong password: status %d, want 401", rec.Code)
	}

	rec = doToken(t, h, http.MethodPost, "/auth/register", "", map[string]any{
		"username": "BOB", "email": "other@example.com", "password": "another password",
	})
	if rec.Code != http.StatusConflict {
		t.Errorf("taken username: status %d, want 409", rec.Code)
	}
}

func TestLogoutEndsAccess(t *testing.T) {
	h := newTestHandler(t)
	_, token := register(t, h, "bob")

	rec := doToken(t, h, http.MethodPost, "/auth/logout", token, nil)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("logout: status %d", rec.Code)
	}
	rec = doToken(t, h, http.MethodGet, "/ledgers/l1/transactions", token, nil)
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("after logout: status %d, want 401", rec.Code)
	}
}

func TestUserCannotCreateLedgerForAnotherUser(t *testing.T) {
	h := newTestHandler(t)
	bobID, _ := register(t, h, "bob")

	// u1 is logged in but tries to create a ledger for bob.
	rec := do(t, h, http.MethodPost, "/users/"+bobID+"/ledgers", "u1", map[string]any{"name": "Sneaky"})
	if rec.Code != http.StatusForbidden {
		t.Errorf("status = %d, want 403", rec.Code)
	}
}

func TestSetupValidationStatuses(t *testing.T) {
	h := newTestHandler(t)

	rec := do(t, h, http.MethodPost, "/users/u1/ledgers", "u1", map[string]any{"name": "Home", "payday_day": 40})
	if rec.Code != http.StatusBadRequest {
		t.Errorf("payday 40: status = %d, want 400", rec.Code)
	}

	rec = do(t, h, http.MethodPost, "/ledgers/l1/categories", "u1", map[string]any{"name": "Groceries"})
	if rec.Code != http.StatusConflict {
		t.Errorf("duplicate category: status = %d, want 409", rec.Code)
	}
}

func TestForeignLedgersAreHidden(t *testing.T) {
	h := newTestHandler(t)
	_, bobToken := register(t, h, "bob")

	// u1 books a transaction in their own ledger.
	rec := do(t, h, http.MethodPost, "/ledgers/l1/transactions", "u1", expenseBody())
	if rec.Code != http.StatusCreated {
		t.Fatalf("u1 create: status %d, body %s", rec.Code, rec.Body)
	}
	txID := decodeID(t, rec.Body.Bytes())

	// Bob is logged in but is not a member of l1, so every request fails with 404.
	checks := []struct {
		name   string
		method string
		path   string
		body   any
	}{
		{"list transactions", http.MethodGet, "/ledgers/l1/transactions", nil},
		{"book a transaction", http.MethodPost, "/ledgers/l1/transactions", expenseBody()},
		{"delete a transaction", http.MethodDelete, "/ledgers/l1/transactions/" + txID, nil},
		{"list pots", http.MethodGet, "/ledgers/l1/pots", nil},
		{"create pot", http.MethodPost, "/ledgers/l1/pots", map[string]any{"name": "Spy"}},
		{"create category", http.MethodPost, "/ledgers/l1/categories", map[string]any{"name": "Spy"}},
	}
	for _, c := range checks {
		t.Run(c.name, func(t *testing.T) {
			rec := doToken(t, h, c.method, c.path, bobToken, c.body)
			if rec.Code != http.StatusNotFound {
				t.Errorf("status = %d, want 404 (body %s)", rec.Code, rec.Body)
			}
		})
	}

	// The transaction is still there for its owner.
	rec = do(t, h, http.MethodGet, "/ledgers/l1/transactions", "u1", nil)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), txID) {
		t.Errorf("owner lost access or data: status %d, body %s", rec.Code, rec.Body)
	}
}
