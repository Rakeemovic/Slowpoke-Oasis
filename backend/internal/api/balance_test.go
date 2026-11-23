package api

import (
	"encoding/json"
	"net/http"
	"testing"
)

func TestLedgerBalanceCountsOnlyUpToTheGivenDay(t *testing.T) {
	h := newTestHandler(t)
	_, token := register(t, h, "bob")
	me := decodeMe(t, doToken(t, h, http.MethodGet, "/me", token, nil).Body.Bytes())
	ledgerID := *me.PersonalLedgerID

	rec := doToken(t, h, http.MethodPost, "/ledgers/"+ledgerID+"/categories", token,
		map[string]any{"name": "Bonus", "is_income": true})
	if rec.Code != http.StatusCreated {
		t.Fatalf("category: status %d, body %s", rec.Code, rec.Body)
	}
	salary := decodeID(t, rec.Body.Bytes())
	for _, day := range []string{"2026-11-01", "2026-12-01"} {
		rec = doToken(t, h, http.MethodPost, "/ledgers/"+ledgerID+"/transactions", token, map[string]any{
			"type": "income", "category_id": salary, "amount_cents": 1000, "booked_on": day,
		})
		if rec.Code != http.StatusCreated {
			t.Fatalf("income: status %d, body %s", rec.Code, rec.Body)
		}
	}

	rec = doToken(t, h, http.MethodGet, "/ledgers/"+ledgerID+"/balance?date=2026-11-19", token, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("balance: status %d, body %s", rec.Code, rec.Body)
	}
	var got balanceResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	want := balanceResponse{AsOf: "2026-11-19", AvailableCents: 1000, TotalCents: 1000}
	if got != want {
		t.Errorf("balance = %+v, want %+v", got, want)
	}

	if rec := doToken(t, h, http.MethodGet, "/ledgers/"+ledgerID+"/balance?date=tomorrow", token, nil); rec.Code != http.StatusBadRequest {
		t.Errorf("invalid date: status %d, want 400", rec.Code)
	}
}

func TestLedgerBalanceOfForeignLedgerIsNotFound(t *testing.T) {
	h := newTestHandler(t)
	_, aliceToken := register(t, h, "alice")
	_, bobToken := register(t, h, "bob")
	alice := decodeMe(t, doToken(t, h, http.MethodGet, "/me", aliceToken, nil).Body.Bytes())

	rec := doToken(t, h, http.MethodGet, "/ledgers/"+*alice.PersonalLedgerID+"/balance", bobToken, nil)
	if rec.Code != http.StatusNotFound {
		t.Errorf("status %d, want 404", rec.Code)
	}
}
