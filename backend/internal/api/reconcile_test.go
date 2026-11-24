package api

import (
	"encoding/json"
	"net/http"
	"testing"
)

func TestOnlyTheOwnerReconciles(t *testing.T) {
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
	path := "/ledgers/" + ledgerID + "/reconciliations?date=2026-11-20"

	if rec := doToken(t, h, http.MethodPost, path, carolToken, map[string]any{"actual_cents": 100, "correct": true}); rec.Code != http.StatusForbidden {
		t.Errorf("member: status %d, want 403", rec.Code)
	}
	if rec := doToken(t, h, http.MethodPost, path, bobToken, map[string]any{}); rec.Code != http.StatusBadRequest {
		t.Errorf("no balance: status %d, want 400", rec.Code)
	}

	rec = doToken(t, h, http.MethodPost, path, bobToken, map[string]any{"actual_cents": 100, "correct": true})
	if rec.Code != http.StatusOK {
		t.Fatalf("owner: status %d, body %s", rec.Code, rec.Body)
	}
	var got reconcileResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if got.ExpectedCents != 0 || got.DifferenceCents != 100 || got.CorrectionID == "" {
		t.Errorf("reconcile = %+v", got)
	}

	// Every member sees the correction and the last reconciliation, but
	// cannot edit the correction.
	for _, tx := range listTransactionsAs(t, h, carolToken, ledgerID) {
		if tx.ID == got.CorrectionID && (tx.Type != "correction" || tx.CanEdit) {
			t.Errorf("correction as listed = %+v", tx)
		}
	}
	rec = doToken(t, h, http.MethodGet, "/ledgers/"+ledgerID+"/balance?date=2026-11-20", carolToken, nil)
	var b balanceResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &b); err != nil {
		t.Fatal(err)
	}
	if b.Reconciled == nil || *b.Reconciled != (reconciliationResponse{ReconciledOn: "2026-11-20", ActualCents: 100}) {
		t.Errorf("reconciled = %+v", b.Reconciled)
	}
	if rec := doToken(t, h, http.MethodPost, "/ledgers/"+ledgerID+"/transactions", bobToken,
		map[string]any{"type": "correction", "amount_cents": 100, "booked_on": "2026-11-20"}); rec.Code != http.StatusBadRequest {
		t.Errorf("direct correction: status %d, want 400", rec.Code)
	}
}
