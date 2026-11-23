package api

import (
	"encoding/json"
	"net/http"
	"testing"
)

func TestReservesOverHTTP(t *testing.T) {
	h := newTestHandler(t)
	_, token := register(t, h, "bob")
	rec := doToken(t, h, http.MethodGet, "/me", token, nil)
	ledgerID := decodeField(t, rec.Body.Bytes(), "personal_ledger_id")
	insurance := listCategoriesOf(t, h, token, ledgerID)["Insurance"].ID
	payments := "/ledgers/" + ledgerID + "/recurring-payments"

	// A payment due every three months gets a reserve and books from it (R10).
	rec = doToken(t, h, http.MethodPost, payments+"?date=2026-11-16", token, map[string]any{
		"name": "Car insurance", "amount_cents": 120000, "category_id": insurance,
		"interval_months": 3, "due_day": 15, "first_month": "2027-02"})
	if rec.Code != http.StatusCreated {
		t.Fatalf("create: status %d, body %s", rec.Code, rec.Body)
	}
	paymentID := decodeID(t, rec.Body.Bytes())

	rec = doToken(t, h, http.MethodGet, payments+"?date=2026-11-16", token, nil)
	var list []recurringPaymentResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &list); err != nil {
		t.Fatal(err)
	}
	if len(list) != 1 || list[0].ReservePotID == nil || *list[0].ReservePotID != list[0].PotID {
		t.Fatalf("list = %s", rec.Body)
	}
	reserve := list[0].PotID

	rec = doToken(t, h, http.MethodGet, "/ledgers/"+ledgerID+"/pots", token, nil)
	var pots []potResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &pots); err != nil {
		t.Fatal(err)
	}
	found := false
	for _, p := range pots {
		if p.ID == reserve {
			found = p.ReserveFor != nil && *p.ReserveFor == paymentID
		}
	}
	if !found {
		t.Fatalf("pots = %s, want the reserve linked to the payment", rec.Body)
	}

	// The rate of this period (R14): four periods until February.
	rec = doToken(t, h, http.MethodGet, payments+"/reserves?date=2026-11-16", token, nil)
	var rates []reserveRateResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &rates); err != nil {
		t.Fatal(err)
	}
	if len(rates) != 1 || rates[0].PotID != reserve || rates[0].RateCents != 30000 || rates[0].DueOn != "2027-02-15" {
		t.Fatalf("rates = %s", rec.Body)
	}

	// An expense the reserve cannot cover is refused (R12) unless topped up (R13).
	transactions := "/ledgers/" + ledgerID + "/transactions"
	expense := map[string]any{"type": "expense", "category_id": insurance, "pot_id": reserve,
		"amount_cents": 5000, "booked_on": "2026-11-16"}
	if rec = doToken(t, h, http.MethodPost, transactions, token, expense); rec.Code != http.StatusConflict {
		t.Fatalf("expense from empty reserve: status %d, want 409", rec.Code)
	}
	expense["top_up"] = true
	rec = doToken(t, h, http.MethodPost, transactions, token, expense)
	if rec.Code != http.StatusCreated {
		t.Fatalf("top up: status %d, body %s", rec.Code, rec.Body)
	}
	var created struct {
		ToppedUpCents int64 `json:"topped_up_cents"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &created); err != nil || created.ToppedUpCents != 5000 {
		t.Fatalf("top up answer = %s", rec.Body)
	}

	// The reserve of an active payment cannot be archived.
	rec = doToken(t, h, http.MethodPatch, "/ledgers/"+ledgerID+"/pots/"+reserve, token, map[string]any{"archived": true})
	if rec.Code != http.StatusConflict {
		t.Fatalf("archive reserve: status %d, want 409", rec.Code)
	}
}
