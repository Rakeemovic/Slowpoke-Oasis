package api

import (
	"net/http"
	"strings"
	"testing"
)

func TestHouseholdMembershipRules(t *testing.T) {
	h := newTestHandler(t)
	_, bobToken := register(t, h, "bob")
	carolID, carolToken := register(t, h, "carol")
	_, daveToken := register(t, h, "dave")

	rec := doToken(t, h, http.MethodPost, "/households", bobToken, map[string]any{"name": "Flat"})
	if rec.Code != http.StatusCreated {
		t.Fatalf("create household: status %d, body %s", rec.Code, rec.Body)
	}
	householdID := decodeID(t, rec.Body.Bytes())
	ledgerID := decodeField(t, rec.Body.Bytes(), "ledger_id")

	members := "/households/" + householdID + "/members"

	rec = doToken(t, h, http.MethodPost, members, bobToken, map[string]any{"username": "carol"})
	if rec.Code != http.StatusNoContent {
		t.Fatalf("owner adds carol: status %d, want 204, body %s", rec.Code, rec.Body)
	}
	rec = doToken(t, h, http.MethodPost, members, bobToken, map[string]any{"username": "carol"})
	if rec.Code != http.StatusConflict {
		t.Errorf("adding carol twice: status %d, want 409", rec.Code)
	}
	rec = doToken(t, h, http.MethodPost, members, bobToken, map[string]any{"username": "nobody"})
	if rec.Code != http.StatusNotFound {
		t.Errorf("unknown username: status %d, want 404", rec.Code)
	}
	rec = doToken(t, h, http.MethodPost, members, carolToken, map[string]any{"username": "dave"})
	if rec.Code != http.StatusForbidden {
		t.Errorf("member adds dave: status %d, want 403", rec.Code)
	}

	// Carol sees the household as a member; Dave, who is not a member, sees nothing.
	rec = doToken(t, h, http.MethodGet, "/households", carolToken, nil)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"role":"member"`) {
		t.Errorf("carol's households: status %d, body %s", rec.Code, rec.Body)
	}
	rec = doToken(t, h, http.MethodGet, "/ledgers/"+ledgerID+"/transactions", daveToken, nil)
	if rec.Code != http.StatusNotFound {
		t.Errorf("non-member reads household ledger: status %d, want 404", rec.Code)
	}
	rec = doToken(t, h, http.MethodDelete, members+"/"+carolID, daveToken, nil)
	if rec.Code != http.StatusNotFound {
		t.Errorf("non-member removes carol: status %d, want 404", rec.Code)
	}

	// Carol may read the shared ledger and leaves by removing herself.
	rec = doToken(t, h, http.MethodGet, "/ledgers/"+ledgerID+"/transactions/deleted", carolToken, nil)
	if rec.Code != http.StatusOK {
		t.Errorf("member reads deleted transactions: status %d, want 200", rec.Code)
	}
	rec = doToken(t, h, http.MethodDelete, members+"/"+carolID, carolToken, nil)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("carol leaves: status %d, want 204", rec.Code)
	}
	rec = doToken(t, h, http.MethodDelete, members+"/"+carolID, bobToken, nil)
	if rec.Code != http.StatusNotFound {
		t.Errorf("removing carol again: status %d, want 404", rec.Code)
	}
	rec = doToken(t, h, http.MethodGet, "/ledgers/"+ledgerID+"/transactions", carolToken, nil)
	if rec.Code != http.StatusNotFound {
		t.Errorf("former member reads household ledger: status %d, want 404", rec.Code)
	}

	// The owner stays in the household.
	rec = doToken(t, h, http.MethodGet, "/ledgers/"+ledgerID+"/transactions", bobToken, nil)
	if rec.Code != http.StatusOK {
		t.Errorf("owner reads household ledger: status %d, want 200", rec.Code)
	}
}

func TestRestoreUnknownTransactionIsNotFound(t *testing.T) {
	h := newTestHandler(t)
	_, token := register(t, h, "erin")
	rec := doToken(t, h, http.MethodPost, "/households", token, map[string]any{"name": "Flat"})
	ledgerID := decodeField(t, rec.Body.Bytes(), "ledger_id")

	rec = doToken(t, h, http.MethodPost, "/ledgers/"+ledgerID+"/transactions/unknown/restore", token, nil)
	if rec.Code != http.StatusNotFound {
		t.Errorf("status %d, want 404", rec.Code)
	}
}
