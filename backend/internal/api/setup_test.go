package api

import (
	"encoding/json"
	"net/http"
	"testing"
)

// decodeID reads the "id" field from a JSON response.
func decodeID(t *testing.T, body []byte) string {
	t.Helper()
	var m map[string]string
	if err := json.Unmarshal(body, &m); err != nil {
		t.Fatalf("decode response %s: %v", body, err)
	}
	return m["id"]
}

func TestOnboardingFlowWithoutSQL(t *testing.T) {
	h := newTestHandler(t)

	// 1. Create a user.
	rec := do(t, h, http.MethodPost, "/users", "", map[string]any{"name": "Bob"})
	if rec.Code != http.StatusCreated {
		t.Fatalf("create user: status %d, body %s", rec.Code, rec.Body)
	}
	userID := decodeID(t, rec.Body.Bytes())

	// 2. Create a personal ledger for that user.
	rec = do(t, h, http.MethodPost, "/users/"+userID+"/ledgers", userID,
		map[string]any{"name": "Home", "payday_day": 23})
	if rec.Code != http.StatusCreated {
		t.Fatalf("create ledger: status %d, body %s", rec.Code, rec.Body)
	}
	ledgerID := decodeID(t, rec.Body.Bytes())

	// 3. Create a category in that ledger.
	rec = do(t, h, http.MethodPost, "/ledgers/"+ledgerID+"/categories", userID,
		map[string]any{"name": "Groceries"})
	if rec.Code != http.StatusCreated {
		t.Fatalf("create category: status %d, body %s", rec.Code, rec.Body)
	}
	categoryID := decodeID(t, rec.Body.Bytes())

	// 4. Book a transaction with it.
	rec = do(t, h, http.MethodPost, "/ledgers/"+ledgerID+"/transactions", userID,
		map[string]any{"type": "expense", "category_id": categoryID, "amount_cents": 500, "booked_on": "2026-11-16"})
	if rec.Code != http.StatusCreated {
		t.Fatalf("create transaction: status %d, body %s", rec.Code, rec.Body)
	}
}

func TestUserCannotCreateLedgerForAnotherUser(t *testing.T) {
	h := newTestHandler(t)

	// u1 tries to create a ledger for someone else.
	rec := do(t, h, http.MethodPost, "/users/other/ledgers", "u1", map[string]any{"name": "Sneaky"})
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
	if rec.Code != http.StatusUnprocessableEntity {
		t.Errorf("duplicate category: status = %d, want 422", rec.Code)
	}
}
