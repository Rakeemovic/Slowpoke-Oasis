package api

import (
	"encoding/json"
	"net/http"
	"testing"
)

// openOccurrences returns the open occurrences of a ledger on a day.
func openOccurrences(t *testing.T, h http.Handler, token, ledgerID, date string) []occurrenceResponse {
	t.Helper()
	rec := doToken(t, h, http.MethodGet, "/ledgers/"+ledgerID+"/recurring-payments/due?date="+date, token, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("list due: status %d, body %s", rec.Code, rec.Body)
	}
	var list []occurrenceResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &list); err != nil {
		t.Fatal(err)
	}
	return list
}

func TestRecurringPaymentsInHousehold(t *testing.T) {
	h := newTestHandler(t)
	_, bobToken := register(t, h, "bob")
	_, carolToken := register(t, h, "carol")
	_, daveToken := register(t, h, "dave")

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
	rentCategory := listCategoriesOf(t, h, bobToken, ledgerID)["Rent"].ID
	payments := "/ledgers/" + ledgerID + "/recurring-payments"

	// Every member may create a recurring payment (rule R8).
	body := map[string]any{"name": "Rent", "amount_cents": 80000, "category_id": rentCategory,
		"interval_months": 1, "due_day": 20, "first_month": "2026-11"}
	rec = doToken(t, h, http.MethodPost, payments+"?date=2026-11-16", carolToken, body)
	if rec.Code != http.StatusCreated {
		t.Fatalf("member creates: status %d, body %s", rec.Code, rec.Body)
	}
	paymentID := decodeID(t, rec.Body.Bytes())

	body["interval_months"] = 2
	if rec = doToken(t, h, http.MethodPost, payments, carolToken, body); rec.Code != http.StatusBadRequest {
		t.Errorf("bad interval: status %d, want 400", rec.Code)
	}
	if rec = doToken(t, h, http.MethodGet, payments, daveToken, nil); rec.Code != http.StatusNotFound {
		t.Errorf("outsider lists: status %d, want 404", rec.Code)
	}

	// The list says when the payment is due next.
	rec = doToken(t, h, http.MethodGet, payments+"?date=2026-11-21", carolToken, nil)
	var list []recurringPaymentResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &list); err != nil {
		t.Fatal(err)
	}
	if len(list) != 1 || list[0].NextDueOn == nil || *list[0].NextDueOn != "2026-12-20" || list[0].LastMonth != nil {
		t.Fatalf("list = %s", rec.Body)
	}

	// Only the owner may change it.
	paymentPath := payments + "/" + paymentID
	if rec = doToken(t, h, http.MethodPatch, paymentPath, carolToken, map[string]any{"amount_cents": 1}); rec.Code != http.StatusForbidden {
		t.Errorf("member changes: status %d, want 403", rec.Code)
	}
	if rec = doToken(t, h, http.MethodPatch, paymentPath+"?date=2026-11-16", bobToken, map[string]any{"amount_cents": 85000}); rec.Code != http.StatusNoContent {
		t.Errorf("owner changes: status %d, body %s", rec.Code, rec.Body)
	}

	// The open occurrence is booked by a member and disappears.
	due := openOccurrences(t, h, carolToken, ledgerID, "2026-11-20")
	if len(due) != 1 || due[0].DueOn != "2026-11-20" || due[0].Status != "due" || due[0].AmountCents != 85000 {
		t.Fatalf("due = %+v", due)
	}
	booking := map[string]any{"type": "expense", "category_id": rentCategory, "amount_cents": 85000,
		"booked_on": "2026-11-20", "recurring_payment_id": paymentID, "recurring_due_on": "2026-11-20"}
	rec = doToken(t, h, http.MethodPost, "/ledgers/"+ledgerID+"/transactions", carolToken, booking)
	if rec.Code != http.StatusCreated {
		t.Fatalf("book: status %d, body %s", rec.Code, rec.Body)
	}
	txID := decodeID(t, rec.Body.Bytes())
	if got := openOccurrences(t, h, carolToken, ledgerID, "2026-11-20"); len(got) != 0 {
		t.Fatalf("still open: %+v", got)
	}
	if rec = doToken(t, h, http.MethodPost, "/ledgers/"+ledgerID+"/transactions", bobToken, booking); rec.Code != http.StatusConflict {
		t.Errorf("book twice: status %d, want 409", rec.Code)
	}
	rec = doToken(t, h, http.MethodGet, "/ledgers/"+ledgerID+"/transactions/"+txID, carolToken, nil)
	var tx transactionResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &tx); err != nil {
		t.Fatal(err)
	}
	if tx.RecurringPaymentID != paymentID || tx.RecurringDueOn != "2026-11-20" {
		t.Errorf("booking lacks the link: %s", rec.Body)
	}

	// Every member may skip an occurrence and undo the skip (rule R7).
	skip := paymentPath + "/skips/2026-12-20"
	if rec = doToken(t, h, http.MethodPut, skip, carolToken, nil); rec.Code != http.StatusNoContent {
		t.Fatalf("skip: status %d, body %s", rec.Code, rec.Body)
	}
	if got := openOccurrences(t, h, bobToken, ledgerID, "2026-12-21"); len(got) != 0 {
		t.Fatalf("skipped occurrence still open: %+v", got)
	}
	if rec = doToken(t, h, http.MethodDelete, skip, bobToken, nil); rec.Code != http.StatusNoContent {
		t.Fatalf("undo skip: status %d, body %s", rec.Code, rec.Body)
	}
	if rec = doToken(t, h, http.MethodDelete, skip, bobToken, nil); rec.Code != http.StatusNotFound {
		t.Errorf("undo twice: status %d, want 404", rec.Code)
	}
	if rec = doToken(t, h, http.MethodPut, paymentPath+"/skips/2026-12-19", carolToken, nil); rec.Code != http.StatusBadRequest {
		t.Errorf("skip a day that is not due: status %d, want 400", rec.Code)
	}
	if got := openOccurrences(t, h, bobToken, ledgerID, "2026-12-21"); len(got) != 1 || got[0].Status != "overdue" {
		t.Fatalf("after undo: %+v", got)
	}
}
