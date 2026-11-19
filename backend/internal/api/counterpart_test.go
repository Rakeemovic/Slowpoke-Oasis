package api

import (
	"encoding/json"
	"net/http"
	"testing"
)

// listTransactionsAs returns the transactions of a ledger as seen by token.
func listTransactionsAs(t *testing.T, h http.Handler, token, ledgerID string) []transactionResponse {
	t.Helper()
	rec := doToken(t, h, http.MethodGet, "/ledgers/"+ledgerID+"/transactions", token, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("list: status %d, body %s", rec.Code, rec.Body)
	}
	var out []transactionResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	return out
}

// onlyCounterpart returns the counterpart of the single transaction in txs.
func onlyCounterpart(t *testing.T, txs []transactionResponse) counterpartResponse {
	t.Helper()
	if len(txs) != 1 || txs[0].Counterpart == nil {
		t.Fatalf("want one transaction with a counterpart, got %+v", txs)
	}
	return *txs[0].Counterpart
}

// Bob moves money from his personal ledger into the household. Bob sees both
// ledgers by name; Alice, a member, only learns it came from a personal ledger.
func TestTransferShowsCounterpartOnlyWhereAccessible(t *testing.T) {
	h := newTestHandler(t)
	_, bobToken := register(t, h, "bob")
	_, aliceToken := register(t, h, "alice")
	bob := decodeMe(t, doToken(t, h, http.MethodGet, "/me", bobToken, nil).Body.Bytes())
	personalID := *bob.PersonalLedgerID

	rec := doToken(t, h, http.MethodPost, "/households", bobToken, map[string]any{"name": "Flat"})
	if rec.Code != http.StatusCreated {
		t.Fatalf("create household: status %d, body %s", rec.Code, rec.Body)
	}
	householdID := decodeID(t, rec.Body.Bytes())
	householdLedger := decodeField(t, rec.Body.Bytes(), "ledger_id")
	rec = doToken(t, h, http.MethodPost, "/households/"+householdID+"/members", bobToken, map[string]any{"username": "alice"})
	if rec.Code != http.StatusNoContent {
		t.Fatalf("add alice: status %d, body %s", rec.Code, rec.Body)
	}

	rec = doToken(t, h, http.MethodPost, "/transfers", bobToken, map[string]any{
		"from_ledger_id": personalID, "to_ledger_id": householdLedger,
		"amount_cents": 30000, "booked_on": "2026-11-19",
	})
	if rec.Code != http.StatusCreated {
		t.Fatalf("transfer: status %d, body %s", rec.Code, rec.Body)
	}

	tests := []struct {
		name     string
		token    string
		ledgerID string
		want     counterpartResponse
	}{
		{"bob in personal", bobToken, personalID, counterpartResponse{Kind: "household", LedgerID: householdLedger, Name: "Flat"}},
		{"bob in household", bobToken, householdLedger, counterpartResponse{Kind: "personal", LedgerID: personalID, Name: "Personal"}},
		{"alice in household", aliceToken, householdLedger, counterpartResponse{Kind: "personal"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := onlyCounterpart(t, listTransactionsAs(t, h, tt.token, tt.ledgerID)); got != tt.want {
				t.Errorf("counterpart = %+v, want %+v", got, tt.want)
			}
		})
	}
}
