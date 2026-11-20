package api

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/Rakeemovic/slowpoke-oasis/backend/internal/ledger"
)

// Bob and Alice each book in their household, then Alice leaves. The list names
// Bob by username and shows Alice's booking as the former-member placeholder.
func TestTransactionListNamesAuthor(t *testing.T) {
	h := newTestHandler(t)
	_, bobToken := register(t, h, "bob")
	aliceID, aliceToken := register(t, h, "alice")

	rec := doToken(t, h, http.MethodPost, "/households", bobToken, map[string]any{"name": "Flat"})
	if rec.Code != http.StatusCreated {
		t.Fatalf("create household: status %d, body %s", rec.Code, rec.Body)
	}
	members := "/households/" + decodeID(t, rec.Body.Bytes()) + "/members"
	ledgerID := decodeField(t, rec.Body.Bytes(), "ledger_id")
	rec = doToken(t, h, http.MethodPost, members, bobToken, map[string]any{"username": "alice"})
	if rec.Code != http.StatusNoContent {
		t.Fatalf("add alice: status %d, body %s", rec.Code, rec.Body)
	}
	rec = doToken(t, h, http.MethodPost, "/ledgers/"+ledgerID+"/categories", bobToken, map[string]any{"name": "Groceries"})
	if rec.Code != http.StatusCreated {
		t.Fatalf("create category: status %d, body %s", rec.Code, rec.Body)
	}
	categoryID := decodeID(t, rec.Body.Bytes())

	for _, token := range []string{bobToken, aliceToken} {
		rec = doToken(t, h, http.MethodPost, "/ledgers/"+ledgerID+"/transactions", token, map[string]any{
			"type": "expense", "category_id": categoryID, "amount_cents": 500, "booked_on": "2026-11-19",
		})
		if rec.Code != http.StatusCreated {
			t.Fatalf("create transaction: status %d, body %s", rec.Code, rec.Body)
		}
	}
	rec = doToken(t, h, http.MethodDelete, members+"/"+aliceID, aliceToken, nil)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("alice leaves: status %d, body %s", rec.Code, rec.Body)
	}

	got := map[string]*string{}
	formerTxID := ""
	for _, tx := range listTransactionsAs(t, h, bobToken, ledgerID) {
		got[tx.UserID] = tx.Username
		if tx.UserID == ledger.FormerMemberID {
			formerTxID = tx.ID
		}
	}
	if len(got) != 2 {
		t.Fatalf("want bookings by two authors, got %v", got)
	}
	for userID, username := range got {
		switch {
		case userID == ledger.FormerMemberID && username != nil:
			t.Errorf("former member has username %q, want null", *username)
		case userID != ledger.FormerMemberID && (username == nil || *username != "bob"):
			t.Errorf("bob's booking has username %v, want bob", username)
		}
	}

	// The owner may delete bookings of departed members (rule X12).
	rec = doToken(t, h, http.MethodDelete, "/ledgers/"+ledgerID+"/transactions/"+formerTxID, bobToken, nil)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("delete: status %d, body %s", rec.Code, rec.Body)
	}
	rec = doToken(t, h, http.MethodGet, "/ledgers/"+ledgerID+"/transactions/deleted", bobToken, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("list deleted: status %d, body %s", rec.Code, rec.Body)
	}
	var deleted []deletedTransactionResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &deleted); err != nil {
		t.Fatal(err)
	}
	if len(deleted) != 1 {
		t.Fatalf("want one deleted transaction, got %d", len(deleted))
	}
	if d := deleted[0]; d.Username != nil || d.DeletedByUsername == nil || *d.DeletedByUsername != "bob" {
		t.Errorf("deleted: username %v, deleted_by_username %v, want null and bob", d.Username, d.DeletedByUsername)
	}
}
