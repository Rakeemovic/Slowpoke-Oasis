package api

import (
	"encoding/json"
	"net/http"
	"reflect"
	"testing"

	"github.com/Rakeemovic/slowpoke-oasis/backend/internal/ledger"
)

// flags is what a viewer may do with one transaction (rule X19).
type flags struct{ edit, delete bool }

// flagsByAuthorAndType lists the flags of every transaction in a ledger as
// seen by token, keyed by "author/type" so the test can name each booking.
func flagsByAuthorAndType(t *testing.T, h http.Handler, token, ledgerID string, names map[string]string) map[string]flags {
	t.Helper()
	out := map[string]flags{}
	for _, tx := range listTransactionsAs(t, h, token, ledgerID) {
		out[names[tx.UserID]+"/"+tx.Type] = flags{tx.CanEdit, tx.CanDelete}
	}
	return out
}

// Bob owns a household, Alice is a member. Each books an expense, and Alice
// moves money in from her personal ledger. The flags must match the rules of
// AuthorizeTransactionChange, including both legs of a transfer.
func TestTransactionListFlagsWhatTheViewerMayChange(t *testing.T) {
	h := newTestHandler(t)
	bobID, bobToken := register(t, h, "bob")
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
	categoryID := decodeID(t, rec.Body.Bytes())
	for _, token := range []string{bobToken, aliceToken} {
		rec = doToken(t, h, http.MethodPost, "/ledgers/"+ledgerID+"/transactions", token, map[string]any{
			"type": "expense", "category_id": categoryID, "amount_cents": 500, "booked_on": "2026-11-19",
		})
		if rec.Code != http.StatusCreated {
			t.Fatalf("create transaction: status %d, body %s", rec.Code, rec.Body)
		}
	}
	rec = doToken(t, h, http.MethodGet, "/me", aliceToken, nil)
	alicePersonal := decodeField(t, rec.Body.Bytes(), "personal_ledger_id")
	rec = doToken(t, h, http.MethodPost, "/transfers", aliceToken, map[string]any{
		"from_ledger_id": alicePersonal, "to_ledger_id": ledgerID,
		"amount_cents": 1000, "booked_on": "2026-11-19",
	})
	if rec.Code != http.StatusCreated {
		t.Fatalf("transfer: status %d, body %s", rec.Code, rec.Body)
	}

	names := map[string]string{bobID: "bob", aliceID: "alice", ledger.FormerMemberID: "former"}
	check := func(viewer, token string, want map[string]flags) {
		t.Helper()
		got := flagsByAuthorAndType(t, h, token, ledgerID, names)
		if len(got) != len(want) {
			t.Fatalf("%s sees %v, want %v", viewer, got, want)
		}
		for key, w := range want {
			if got[key] != w {
				t.Errorf("%s on %s: got %+v, want %+v", viewer, key, got[key], w)
			}
		}
	}

	check("owner bob", bobToken, map[string]flags{
		"bob/expense":   {edit: true, delete: true},
		"alice/expense": {edit: true, delete: false}, // owner edits, only the author deletes
		// A transfer is never edited, and its other leg is Alice's.
		"alice/transfer_in": {edit: false, delete: false},
	})
	check("member alice", aliceToken, map[string]flags{
		"bob/expense":       {edit: false, delete: false},
		"alice/expense":     {edit: true, delete: true},
		"alice/transfer_in": {edit: false, delete: true}, // she owns both legs
	})

	rec = doToken(t, h, http.MethodDelete, members+"/"+aliceID, aliceToken, nil)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("alice leaves: status %d, body %s", rec.Code, rec.Body)
	}
	check("owner bob after alice left", bobToken, map[string]flags{
		"bob/expense":    {edit: true, delete: true},
		"former/expense": {edit: true, delete: true}, // orphaned booking (rule X12)
		// The other leg still lives in Alice's personal ledger.
		"former/transfer_in": {edit: false, delete: false},
	})
}

// One transaction can be read on its own, with the same fields as in the
// list. It is gone once deleted, and another user's ledger answers 404.
func TestGetSingleTransaction(t *testing.T) {
	h := newTestHandler(t)
	_, bobToken := register(t, h, "bob")
	_, eveToken := register(t, h, "eve")

	rec := doToken(t, h, http.MethodGet, "/me", bobToken, nil)
	ledgerID := decodeField(t, rec.Body.Bytes(), "personal_ledger_id")
	rec = doToken(t, h, http.MethodPost, "/ledgers/"+ledgerID+"/categories", bobToken, map[string]any{"name": "Food"})
	categoryID := decodeID(t, rec.Body.Bytes())
	rec = doToken(t, h, http.MethodPost, "/ledgers/"+ledgerID+"/transactions", bobToken, map[string]any{
		"type": "expense", "category_id": categoryID, "amount_cents": 250, "booked_on": "2026-11-19", "note": "Bread",
	})
	if rec.Code != http.StatusCreated {
		t.Fatalf("create transaction: status %d, body %s", rec.Code, rec.Body)
	}
	path := "/ledgers/" + ledgerID + "/transactions/" + decodeID(t, rec.Body.Bytes())

	rec = doToken(t, h, http.MethodGet, path, bobToken, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("get: status %d, body %s", rec.Code, rec.Body)
	}
	var got listedTransactionResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if want := listTransactionsAs(t, h, bobToken, ledgerID)[0]; !reflect.DeepEqual(got, want) {
		t.Errorf("get = %+v, list entry = %+v", got, want)
	}

	if rec = doToken(t, h, http.MethodGet, path, eveToken, nil); rec.Code != http.StatusNotFound {
		t.Errorf("other user: status %d, want 404", rec.Code)
	}
	doToken(t, h, http.MethodDelete, path, bobToken, nil)
	if rec = doToken(t, h, http.MethodGet, path, bobToken, nil); rec.Code != http.StatusNotFound {
		t.Errorf("after delete: status %d, want 404", rec.Code)
	}
}
