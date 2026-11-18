package ledger

import (
	"testing"
	"time"
)

func TestPotSummaryComparesPlanWithActual(t *testing.T) {
	db, s := setup(t)
	ctx := t.Context()
	mustExec(t, db, `INSERT INTO pot_line (pot_id, ledger_id, category_id, amount_cents) VALUES ('p-default', 'l1', 'c-food', 5000)`)
	createExpense(t, s, "p-default", "2026-11-10", 1000)
	createExpense(t, s, "p-default", "2026-12-01", 999) // outside the period

	sum, err := s.PotSummary(ctx, "l1", "p-default", "2026-11-01", "2026-11-30")
	if err != nil {
		t.Fatal(err)
	}
	if len(sum.Lines) != 1 {
		t.Fatalf("%d lines, want 1", len(sum.Lines))
	}
	l := sum.Lines[0]
	if l.PlannedCents != 5000 || l.ActualCents != 1000 || l.DifferenceCents != 4000 {
		t.Errorf("line = %+v, want planned 5000, actual 1000, difference 4000", l)
	}
}

func TestPotSummaryCountsTransfers(t *testing.T) {
	_, s := setup(t)
	ctx := t.Context()
	_, err := s.CreateTransaction(ctx, NewTransaction{
		LedgerID: "l1", UserID: "u1", Type: TypeTransfer,
		PotID: "p-default", CounterpartPotID: "p-holiday",
		AmountCents: 200, BookedOn: "2026-11-05",
	}, time.Now())
	if err != nil {
		t.Fatal(err)
	}

	holiday, err := s.PotSummary(ctx, "l1", "p-holiday", "", "")
	if err != nil {
		t.Fatal(err)
	}
	if holiday.TransfersInCents != 200 || holiday.TransfersOutCents != 0 {
		t.Errorf("holiday transfers in/out = %d/%d, want 200/0", holiday.TransfersInCents, holiday.TransfersOutCents)
	}
	defaultPot, err := s.PotSummary(ctx, "l1", "p-default", "", "")
	if err != nil {
		t.Fatal(err)
	}
	if defaultPot.TransfersOutCents != 200 {
		t.Errorf("default transfers out = %d, want 200", defaultPot.TransfersOutCents)
	}
}

func TestCategoryTotalsAreSignedByCategoryKind(t *testing.T) {
	_, s := setup(t)
	ctx := t.Context()
	createExpense(t, s, "p-default", "2026-11-10", 1000)
	_, err := s.CreateTransaction(ctx, NewTransaction{
		LedgerID: "l1", UserID: "u1", Type: TypeIncome,
		CategoryID: "c-salary", AmountCents: 3000, BookedOn: "2026-11-01",
	}, time.Now())
	if err != nil {
		t.Fatal(err)
	}

	totals, err := s.CategoryTotals(ctx, "l1", "", "")
	if err != nil {
		t.Fatal(err)
	}
	byID := map[string]CategoryTotal{}
	for _, c := range totals {
		byID[c.CategoryID] = c
	}
	if byID["c-food"].TotalCents != 1000 || byID["c-food"].Count != 1 {
		t.Errorf("c-food = %+v, want total 1000, count 1", byID["c-food"])
	}
	if byID["c-salary"].TotalCents != 3000 || !byID["c-salary"].IsIncome {
		t.Errorf("c-salary = %+v, want income total 3000", byID["c-salary"])
	}
}
