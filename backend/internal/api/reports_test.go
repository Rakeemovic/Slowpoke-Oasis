package api

import (
	"net/http"
	"strings"
	"testing"
)

func TestReportStatusesAndPeriodValidation(t *testing.T) {
	h := newTestHandler(t)
	userID, token := register(t, h, "frank")

	rec := doToken(t, h, http.MethodPost, "/users/"+userID+"/ledgers", token,
		map[string]any{"name": "Home", "payday_day": 23})
	if rec.Code != http.StatusCreated {
		t.Fatalf("create ledger: status %d, body %s", rec.Code, rec.Body)
	}
	ledgerID := decodeID(t, rec.Body.Bytes())
	base := "/ledgers/" + ledgerID

	rec = doToken(t, h, http.MethodGet, base+"/pots/unknown/summary?month=2026-11", token, nil)
	if rec.Code != http.StatusNotFound {
		t.Errorf("summary of unknown pot: status %d, want 404", rec.Code)
	}
	rec = doToken(t, h, http.MethodGet, base+"/analytics?month=2026-11&from=2026-11-01", token, nil)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("month and from together: status %d, want 400", rec.Code)
	}
	rec = doToken(t, h, http.MethodGet, base+"/analytics?from=2026-13-01", token, nil)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("invalid date: status %d, want 400", rec.Code)
	}
	rec = doToken(t, h, http.MethodGet, base+"/analytics?month=2026-11", token, nil)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"income_cents"`) {
		t.Errorf("analytics for a month: status %d, body %s", rec.Code, rec.Body)
	}
	rec = doToken(t, h, http.MethodGet, base+"/analytics", token, nil)
	if rec.Code != http.StatusOK {
		t.Errorf("analytics without range: status %d, want 200", rec.Code)
	}
	rec = doToken(t, h, http.MethodPatch, base+"/transactions/unknown", token, map[string]any{"note": "x"})
	if rec.Code != http.StatusNotFound {
		t.Errorf("patch unknown transaction: status %d, want 404", rec.Code)
	}
}
