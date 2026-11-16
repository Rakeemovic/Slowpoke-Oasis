package api

import (
	"net/http"
	"testing"
)

func TestDeleteAccountNeedsCorrectPasswordAndRemovesEverything(t *testing.T) {
	h := newTestHandler(t)
	bobID, bobToken := register(t, h, "bob")

	rec := doToken(t, h, http.MethodPost, "/users/"+bobID+"/ledgers", bobToken,
		map[string]any{"name": "Home", "payday_day": 23})
	if rec.Code != http.StatusCreated {
		t.Fatalf("create ledger: status %d, body %s", rec.Code, rec.Body)
	}
	ledgerID := decodeID(t, rec.Body.Bytes())

	// Wrong password: nothing is deleted.
	rec = doToken(t, h, http.MethodDelete, "/auth/account", bobToken,
		map[string]any{"password": "wrong password here"})
	if rec.Code != http.StatusForbidden {
		t.Fatalf("wrong password: status %d, want 403, body %s", rec.Code, rec.Body)
	}
	rec = doToken(t, h, http.MethodGet, "/ledgers/"+ledgerID+"/pots", bobToken, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("ledger after failed deletion: status %d, want 200", rec.Code)
	}

	// Correct password: the account and its ledgers are removed.
	rec = doToken(t, h, http.MethodDelete, "/auth/account", bobToken,
		map[string]any{"password": "correct horse battery"})
	if rec.Code != http.StatusNoContent {
		t.Fatalf("delete account: status %d, want 204, body %s", rec.Code, rec.Body)
	}

	// The old token no longer works, and the ledger is gone.
	rec = doToken(t, h, http.MethodGet, "/ledgers/"+ledgerID+"/pots", bobToken, nil)
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("old token: status %d, want 401", rec.Code)
	}
	rec = doToken(t, h, http.MethodPost, "/auth/login", "", map[string]any{
		"username": "bob", "password": "correct horse battery",
	})
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("login after deletion: status %d, want 401", rec.Code)
	}
}

func TestDeleteAccountRequiresLogin(t *testing.T) {
	h := newTestHandler(t)
	rec := doToken(t, h, http.MethodDelete, "/auth/account", "",
		map[string]any{"password": "correct horse battery"})
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status %d, want 401", rec.Code)
	}
}
