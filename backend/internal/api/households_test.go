package api

import (
	"encoding/json"
	"net/http"
	"reflect"
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

func TestOwnerTransfersOwnershipToMember(t *testing.T) {
	h := newTestHandler(t)
	bobID, bobToken := register(t, h, "bob")
	carolID, carolToken := register(t, h, "carol")
	daveID, daveToken := register(t, h, "dave")

	rec := doToken(t, h, http.MethodPost, "/households", bobToken, map[string]any{"name": "Flat"})
	if rec.Code != http.StatusCreated {
		t.Fatalf("create household: status %d, body %s", rec.Code, rec.Body)
	}
	householdID := decodeID(t, rec.Body.Bytes())
	rec = doToken(t, h, http.MethodPost, "/households/"+householdID+"/members", bobToken, map[string]any{"username": "carol"})
	if rec.Code != http.StatusNoContent {
		t.Fatalf("add carol: status %d, body %s", rec.Code, rec.Body)
	}
	owner := "/households/" + householdID + "/owner/"

	cases := []struct {
		name  string
		token string
		path  string
		want  int
	}{
		{"non-member cannot see the household", daveToken, owner + carolID, http.StatusNotFound},
		{"member cannot transfer", carolToken, owner + carolID, http.StatusForbidden},
		{"target must be a member", bobToken, owner + daveID, http.StatusNotFound},
		{"owner transfers to carol", bobToken, owner + carolID, http.StatusNoContent},
		{"former owner cannot transfer back", bobToken, owner + bobID, http.StatusForbidden},
	}
	for _, c := range cases {
		rec := doToken(t, h, http.MethodPut, c.path, c.token, nil)
		if rec.Code != c.want {
			t.Errorf("%s: status %d, want %d, body %s", c.name, rec.Code, c.want, rec.Body)
		}
	}

	// Carol now has the owner's rights, Bob stays in the household as a member.
	rec = doToken(t, h, http.MethodGet, "/households", bobToken, nil)
	if !strings.Contains(rec.Body.String(), `"role":"member"`) {
		t.Errorf("bob's households after transfer: %s", rec.Body)
	}
	rec = doToken(t, h, http.MethodPost, "/households/"+householdID+"/members", carolToken, map[string]any{"username": "dave"})
	if rec.Code != http.StatusNoContent {
		t.Errorf("new owner adds dave: status %d, want 204, body %s", rec.Code, rec.Body)
	}
}

func TestMembersSeeOnlyUsernameRoleAndIDOfEachOther(t *testing.T) {
	h := newTestHandler(t)
	bobID, bobToken := register(t, h, "bob")
	carolID, carolToken := register(t, h, "carol")
	_, daveToken := register(t, h, "dave")

	rec := doToken(t, h, http.MethodPost, "/households", bobToken, map[string]any{"name": "Flat"})
	if rec.Code != http.StatusCreated {
		t.Fatalf("create household: status %d, body %s", rec.Code, rec.Body)
	}
	members := "/households/" + decodeID(t, rec.Body.Bytes()) + "/members"
	rec = doToken(t, h, http.MethodPost, members, bobToken, map[string]any{"username": "carol"})
	if rec.Code != http.StatusNoContent {
		t.Fatalf("add carol: status %d, body %s", rec.Code, rec.Body)
	}

	rec = doToken(t, h, http.MethodGet, members, daveToken, nil)
	if rec.Code != http.StatusNotFound {
		t.Errorf("non-member lists members: status %d, want 404", rec.Code)
	}

	rec = doToken(t, h, http.MethodGet, members, carolToken, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("member lists members: status %d, body %s", rec.Code, rec.Body)
	}
	// Decoding into generic maps catches any extra field, such as an email.
	var got []map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	want := []map[string]any{
		{"user_id": bobID, "username": "bob", "role": "owner"},
		{"user_id": carolID, "username": "carol", "role": "member"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("members = %v, want %v", got, want)
	}
}
