package api

import (
	"encoding/json"
	"net/http"
	"testing"
)

// The household owner must not reach into a departed member's personal ledger
// through a shared transfer (see authorizeLegChange).
func TestOwnerCannotDeleteALegThatLivesInAnotherPersonalLedger(t *testing.T) {
	h := newTestHandler(t)
	aliceToken := testTokens["u1"]
	bobID, bobToken := register(t, h, "bob")

	rec := doToken(t, h, http.MethodPost, "/users/"+bobID+"/ledgers", bobToken,
		map[string]any{"name": "Home", "payday_day": 1})
	bobLedger := decodeID(t, rec.Body.Bytes())

	rec = doToken(t, h, http.MethodPost, "/households", aliceToken, map[string]any{"name": "Haus"})
	if rec.Code != http.StatusCreated {
		t.Fatalf("create household: status %d, body %s", rec.Code, rec.Body)
	}
	householdID := decodeID(t, rec.Body.Bytes())
	householdLedger := decodeField(t, rec.Body.Bytes(), "ledger_id")

	rec = doToken(t, h, http.MethodPost, "/households/"+householdID+"/members", aliceToken,
		map[string]any{"username": "bob"})
	if rec.Code != http.StatusNoContent {
		t.Fatalf("add bob: status %d, body %s", rec.Code, rec.Body)
	}

	// Bob moves money from his personal ledger into the household while a member.
	rec = doToken(t, h, http.MethodPost, "/transfers", bobToken, map[string]any{
		"from_ledger_id": bobLedger, "to_ledger_id": householdLedger,
		"amount_cents": 1000, "booked_on": "2026-11-16",
	})
	if rec.Code != http.StatusCreated {
		t.Fatalf("transfer: status %d, body %s", rec.Code, rec.Body)
	}
	householdLegID := decodeField(t, rec.Body.Bytes(), "in_id")

	// Bob leaves; his household leg now belongs to the placeholder.
	rec = doToken(t, h, http.MethodDelete, "/households/"+householdID+"/members/"+bobID, bobToken, nil)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("bob leaves: status %d, body %s", rec.Code, rec.Body)
	}

	// Alice owns the household, but the other leg is in Bob's personal ledger.
	rec = doToken(t, h, http.MethodDelete, "/ledgers/"+householdLedger+"/transactions/"+householdLegID, aliceToken, nil)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("owner deletes leg: status %d, want 403, body %s", rec.Code, rec.Body)
	}

	// Bob's own leg is untouched.
	rec = doToken(t, h, http.MethodGet, "/ledgers/"+bobLedger+"/transactions", bobToken, nil)
	var items []map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &items); err != nil {
		t.Fatalf("decode list: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("bob's ledger has %d transactions, want 1", len(items))
	}
}

// A user who may change both legs can delete the whole transfer.
func TestMemberDeletesBothLegsOfOwnTransfer(t *testing.T) {
	h := newTestHandler(t)
	aliceToken := testTokens["u1"]
	bobID, bobToken := register(t, h, "bob")

	rec := doToken(t, h, http.MethodPost, "/users/"+bobID+"/ledgers", bobToken,
		map[string]any{"name": "Home", "payday_day": 1})
	bobLedger := decodeID(t, rec.Body.Bytes())

	rec = doToken(t, h, http.MethodPost, "/households", aliceToken, map[string]any{"name": "Haus"})
	householdID := decodeID(t, rec.Body.Bytes())
	householdLedger := decodeField(t, rec.Body.Bytes(), "ledger_id")
	rec = doToken(t, h, http.MethodPost, "/households/"+householdID+"/members", aliceToken,
		map[string]any{"username": "bob"})
	if rec.Code != http.StatusNoContent {
		t.Fatalf("add bob: status %d, body %s", rec.Code, rec.Body)
	}

	rec = doToken(t, h, http.MethodPost, "/transfers", bobToken, map[string]any{
		"from_ledger_id": bobLedger, "to_ledger_id": householdLedger,
		"amount_cents": 1000, "booked_on": "2026-11-16",
	})
	inID := decodeField(t, rec.Body.Bytes(), "in_id")

	rec = doToken(t, h, http.MethodDelete, "/ledgers/"+householdLedger+"/transactions/"+inID, bobToken, nil)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("bob deletes own transfer: status %d, want 204, body %s", rec.Code, rec.Body)
	}
	rec = doToken(t, h, http.MethodGet, "/ledgers/"+bobLedger+"/transactions", bobToken, nil)
	var items []map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &items); err != nil {
		t.Fatalf("decode list: %v", err)
	}
	if len(items) != 0 {
		t.Fatalf("bob's ledger still has %d transactions, want 0", len(items))
	}
}
