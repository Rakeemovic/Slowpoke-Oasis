package api

import (
	"encoding/json"
	"net/http"
	"testing"
)

// decodeMe parses a GET /me response.
func decodeMe(t *testing.T, body []byte) meResponse {
	t.Helper()
	var me meResponse
	if err := json.Unmarshal(body, &me); err != nil {
		t.Fatalf("decode %s: %v", body, err)
	}
	return me
}

func TestMeReturnsProfileAndPersonalLedger(t *testing.T) {
	h := newTestHandler(t)
	bobID, bobToken := register(t, h, "bob")

	rec := doToken(t, h, http.MethodGet, "/me", bobToken, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("me: status %d, body %s", rec.Code, rec.Body)
	}
	me := decodeMe(t, rec.Body.Bytes())
	if me.UserID != bobID || me.Username != "bob" || me.DisplayName != "Bob" {
		t.Fatalf("me = %+v, want bob's profile", me)
	}
	if me.PersonalLedgerID == nil {
		t.Fatal("personal_ledger_id is null, want the ledger created at registration")
	}

	// The email address never leaves the server.
	var raw map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &raw); err != nil {
		t.Fatal(err)
	}
	if _, ok := raw["email"]; ok {
		t.Fatalf("response contains an email: %s", rec.Body)
	}

	// The personal ledger is usable right away, including its default pot (L4).
	rec = doToken(t, h, http.MethodGet, "/ledgers/"+*me.PersonalLedgerID+"/pots", bobToken, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("list pots: status %d, body %s", rec.Code, rec.Body)
	}
}

func TestMeKeepsFirstLedgerWhenMoreAreCreated(t *testing.T) {
	h := newTestHandler(t)
	bobID, bobToken := register(t, h, "bob")
	first := decodeMe(t, doToken(t, h, http.MethodGet, "/me", bobToken, nil).Body.Bytes())

	rec := doToken(t, h, http.MethodPost, "/users/"+bobID+"/ledgers", bobToken,
		map[string]any{"name": "Side project"})
	if rec.Code != http.StatusCreated {
		t.Fatalf("create ledger: status %d, body %s", rec.Code, rec.Body)
	}

	again := decodeMe(t, doToken(t, h, http.MethodGet, "/me", bobToken, nil).Body.Bytes())
	if *again.PersonalLedgerID != *first.PersonalLedgerID {
		t.Fatalf("personal ledger changed from %s to %s", *first.PersonalLedgerID, *again.PersonalLedgerID)
	}
}

func TestMeNeedsLogin(t *testing.T) {
	h := newTestHandler(t)
	if rec := doToken(t, h, http.MethodGet, "/me", "", nil); rec.Code != http.StatusUnauthorized {
		t.Fatalf("status %d, want 401", rec.Code)
	}
}
