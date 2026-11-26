package api

import (
	"net/http"
)

// purposeLineResponse is one purpose of the report. purpose is "need",
// "want", "saving" or "none" for expenses whose category has no purpose.
type purposeLineResponse struct {
	Purpose          string `json:"purpose"`
	Cents            int64  `json:"cents"`
	SpontaneousCents int64  `json:"spontaneous_cents"`
	TransferCents    int64  `json:"transfer_cents"`
	PreviousCents    int64  `json:"previous_cents"`
	// SharePercent is null when no income is booked in the period.
	SharePercent *int `json:"share_percent"`
}

// GET /ledgers/{ledger}/purpose-report?month=YYYY-MM&date=YYYY-MM-DD
// What the ledger spent in one payday period, by the purpose of the category
// (rule L10); without month, the current period, which date (the app's local
// date) decides.
func (s *Server) purposeReport(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.requireLedgerAccess(w, r); !ok {
		return
	}
	today, ok := s.today(w, r)
	if !ok {
		return
	}
	report, err := s.store.PurposeReport(r.Context(), r.PathValue("ledger"), r.URL.Query().Get("month"), today)
	if err != nil {
		fail(w, err)
		return
	}
	lines := make([]purposeLineResponse, 0, len(report.Lines))
	for _, l := range report.Lines {
		purpose := string(l.Purpose)
		if purpose == "" {
			purpose = "none"
		}
		lines = append(lines, purposeLineResponse{
			Purpose:          purpose,
			Cents:            l.Cents,
			SpontaneousCents: l.SpontaneousCents,
			TransferCents:    l.TransferCents,
			PreviousCents:    l.PreviousCents,
			SharePercent:     l.SharePercent,
		})
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"month":          report.Month,
		"current_month":  report.Current,
		"previous_month": report.PreviousMonth,
		"from":           report.From,
		"to":             report.To,
		"income_cents":   report.IncomeCents,
		"lines":          lines,
	})
}
