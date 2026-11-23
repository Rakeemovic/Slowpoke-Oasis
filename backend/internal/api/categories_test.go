package api

import (
	"encoding/json"
	"net/http"
	"testing"
)

// listCategoriesOf returns the categories of a ledger keyed by name.
func listCategoriesOf(t *testing.T, h http.Handler, token, ledgerID string) map[string]categoryResponse {
	t.Helper()
	rec := doToken(t, h, http.MethodGet, "/ledgers/"+ledgerID+"/categories", token, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("list categories: status %d, body %s", rec.Code, rec.Body)
	}
	var list []categoryResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &list); err != nil {
		t.Fatal(err)
	}
	out := make(map[string]categoryResponse, len(list))
	for _, c := range list {
		out[c.Name] = c
	}
	return out
}

func TestCategoryManagementInHousehold(t *testing.T) {
	h := newTestHandler(t)
	_, bobToken := register(t, h, "bob")
	_, carolToken := register(t, h, "carol")

	rec := doToken(t, h, http.MethodPost, "/households", bobToken, map[string]any{"name": "Flat"})
	if rec.Code != http.StatusCreated {
		t.Fatalf("create household: status %d, body %s", rec.Code, rec.Body)
	}
	householdID := decodeID(t, rec.Body.Bytes())
	ledgerID := decodeField(t, rec.Body.Bytes(), "ledger_id")
	rec = doToken(t, h, http.MethodPost, "/households/"+householdID+"/members", bobToken, map[string]any{"username": "carol"})
	if rec.Code != http.StatusNoContent {
		t.Fatalf("add carol: status %d, body %s", rec.Code, rec.Body)
	}
	categories := "/ledgers/" + ledgerID + "/categories"

	// Every member may create categories, with a purpose.
	rec = doToken(t, h, http.MethodPost, categories, carolToken, map[string]any{"name": "Garden", "purpose": "want"})
	if rec.Code != http.StatusCreated {
		t.Fatalf("member creates category: status %d, body %s", rec.Code, rec.Body)
	}
	garden := decodeID(t, rec.Body.Bytes())

	// Carol books on it, marked as spontaneous.
	body := map[string]any{"type": "expense", "category_id": garden, "amount_cents": 500,
		"booked_on": "2026-11-20", "is_spontaneous": true}
	rec = doToken(t, h, http.MethodPost, "/ledgers/"+ledgerID+"/transactions", carolToken, body)
	if rec.Code != http.StatusCreated {
		t.Fatalf("book: status %d, body %s", rec.Code, rec.Body)
	}
	txID := decodeID(t, rec.Body.Bytes())

	got := listCategoriesOf(t, h, carolToken, ledgerID)
	if c := got["Garden"]; c.Purpose == nil || *c.Purpose != "want" || c.TransactionCount != 1 {
		t.Errorf("Garden = %+v, want purpose want and 1 transaction", c)
	}
	if c := got["Groceries"]; c.Purpose == nil || *c.Purpose != "need" {
		t.Errorf("Groceries = %+v, want purpose need", c)
	}
	if !got["Other"].IsFallback {
		t.Error("Other is not marked as fallback")
	}

	// Only the owner may change or delete categories.
	gardenPath := categories + "/" + garden
	if rec = doToken(t, h, http.MethodPatch, gardenPath, carolToken, map[string]any{"name": "Yard"}); rec.Code != http.StatusForbidden {
		t.Errorf("member renames: status %d, want 403", rec.Code)
	}
	if rec = doToken(t, h, http.MethodDelete, gardenPath, carolToken, nil); rec.Code != http.StatusForbidden {
		t.Errorf("member deletes: status %d, want 403", rec.Code)
	}
	if rec = doToken(t, h, http.MethodPatch, gardenPath, bobToken, map[string]any{"name": "Yard", "purpose": ""}); rec.Code != http.StatusNoContent {
		t.Errorf("owner renames: status %d, body %s", rec.Code, rec.Body)
	}
	if rec = doToken(t, h, http.MethodPatch, gardenPath, bobToken, map[string]any{"purpose": "luxury"}); rec.Code != http.StatusBadRequest {
		t.Errorf("unknown purpose: status %d, want 400", rec.Code)
	}
	if rec = doToken(t, h, http.MethodPatch, categories+"/"+got["Other"].ID, bobToken, map[string]any{"archived": true}); rec.Code != http.StatusConflict {
		t.Errorf("archive fallback: status %d, want 409", rec.Code)
	}
	if rec = doToken(t, h, http.MethodPatch, categories+"/missing", bobToken, map[string]any{"name": "X"}); rec.Code != http.StatusNotFound {
		t.Errorf("unknown category: status %d, want 404", rec.Code)
	}

	// Only the author may change the spontaneous mark, not even the owner.
	txPath := "/ledgers/" + ledgerID + "/transactions/" + txID
	if rec = doToken(t, h, http.MethodPatch, txPath, bobToken, map[string]any{"is_spontaneous": false}); rec.Code != http.StatusForbidden {
		t.Errorf("owner unmarks: status %d, want 403", rec.Code)
	}
	if rec = doToken(t, h, http.MethodPatch, txPath, carolToken, map[string]any{"is_spontaneous": false}); rec.Code != http.StatusNoContent {
		t.Errorf("author unmarks: status %d, body %s", rec.Code, rec.Body)
	}

	// Deleting moves the booking to "Other".
	rec = doToken(t, h, http.MethodDelete, gardenPath, bobToken, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("owner deletes: status %d, body %s", rec.Code, rec.Body)
	}
	var moved struct {
		MovedTo           string `json:"moved_to"`
		MovedTransactions int    `json:"moved_transactions"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &moved); err != nil {
		t.Fatal(err)
	}
	if moved.MovedTo != got["Other"].ID || moved.MovedTransactions != 1 {
		t.Errorf("delete answered %+v, want 1 transaction moved to Other %q", moved, got["Other"].ID)
	}
	rec = doToken(t, h, http.MethodGet, txPath, carolToken, nil)
	var tx transactionResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &tx); err != nil {
		t.Fatal(err)
	}
	if tx.CategoryID != got["Other"].ID || tx.IsSpontaneous {
		t.Errorf("transaction = %+v, want category Other and not spontaneous", tx)
	}
}

func TestArchivedCategoryAnswersConflict(t *testing.T) {
	h := newTestHandler(t)
	_, token := register(t, h, "bob")
	rec := doToken(t, h, http.MethodGet, "/me", token, nil)
	ledgerID := decodeField(t, rec.Body.Bytes(), "personal_ledger_id")
	rent := listCategoriesOf(t, h, token, ledgerID)["Rent"]

	if rec = doToken(t, h, http.MethodPatch, "/ledgers/"+ledgerID+"/categories/"+rent.ID, token, map[string]any{"archived": true}); rec.Code != http.StatusNoContent {
		t.Fatalf("archive: status %d, body %s", rec.Code, rec.Body)
	}
	body := map[string]any{"type": "expense", "category_id": rent.ID, "amount_cents": 500, "booked_on": "2026-11-20"}
	if rec = doToken(t, h, http.MethodPost, "/ledgers/"+ledgerID+"/transactions", token, body); rec.Code != http.StatusConflict {
		t.Errorf("book on archived category: status %d, want 409", rec.Code)
	}
}
