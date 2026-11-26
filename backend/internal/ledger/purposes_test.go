package ledger

import (
	"context"
	"errors"
	"testing"
	"time"
)

// purposeCategory creates an expense category with a purpose in ledger l1.
func purposeCategory(t *testing.T, s *Store, name string, purpose Purpose) string {
	t.Helper()
	id, err := s.CreateCategory(context.Background(), NewCategory{LedgerID: "l1", Name: name, Purpose: purpose})
	if err != nil {
		t.Fatalf("create category %s: %v", name, err)
	}
	return id
}

// spend books an expense of a category in the main pot.
func spend(t *testing.T, s *Store, categoryID, day string, cents int64, spontaneous bool) string {
	t.Helper()
	in := baseExpense()
	in.CategoryID, in.BookedOn, in.AmountCents, in.IsSpontaneous = categoryID, day, cents, spontaneous
	id, err := s.CreateTransaction(context.Background(), in, time.Now())
	if err != nil {
		t.Fatalf("spend: %v", err)
	}
	return id
}

// earn books an income in the main pot.
func earn(t *testing.T, s *Store, day string, cents int64) {
	t.Helper()
	in := baseExpense()
	in.Type, in.CategoryID, in.BookedOn, in.AmountCents = TypeIncome, "c-salary", day, cents
	if _, err := s.CreateTransaction(context.Background(), in, time.Now()); err != nil {
		t.Fatalf("earn: %v", err)
	}
}

// purposeLine returns the line of a purpose of a report.
func purposeLine(t *testing.T, r PurposeReport, p Purpose) PurposeLine {
	t.Helper()
	for _, l := range r.Lines {
		if l.Purpose == p {
			return l
		}
	}
	t.Fatalf("no line for purpose %q", p)
	return PurposeLine{}
}

func reportOf(t *testing.T, s *Store, month, today string) PurposeReport {
	t.Helper()
	r, err := s.PurposeReport(context.Background(), "l1", month, today)
	if err != nil {
		t.Fatalf("report: %v", err)
	}
	return r
}

func TestPurposeReportSplitsExpensesByPurpose(t *testing.T) {
	_, s := setup(t)
	need := purposeCategory(t, s, "Rent", PurposeNeed)
	want := purposeCategory(t, s, "Cinema", PurposeWant)
	saving := purposeCategory(t, s, "Pension", PurposeSaving)
	earn(t, s, "2026-11-01", 100000)
	spend(t, s, need, "2026-11-03", 10000, false)
	spend(t, s, want, "2026-11-05", 5000, true)
	spend(t, s, want, "2026-11-06", 3000, false)
	spend(t, s, saving, "2026-11-07", 3000, false)
	spend(t, s, "c-food", "2026-11-08", 1000, false) // Groceries has no purpose here

	r := reportOf(t, s, "2026-11", "2026-11-20")
	if r.From != "2026-11-01" || r.To != "2026-11-30" || r.IncomeCents != 100000 || r.Month != "2026-11" || r.Current != "2026-11" {
		t.Fatalf("report = %+v", r)
	}
	want4 := []struct {
		purpose     Purpose
		cents       int64
		spontaneous int64
		share       int
	}{
		{PurposeNeed, 10000, 0, 10},
		{PurposeWant, 8000, 5000, 8},
		{PurposeSaving, 3000, 0, 3},
		{"", 1000, 0, 1},
	}
	if len(r.Lines) != len(want4) {
		t.Fatalf("lines = %+v", r.Lines)
	}
	for i, w := range want4 {
		l := r.Lines[i]
		if l.Purpose != w.purpose || l.Cents != w.cents || l.SpontaneousCents != w.spontaneous ||
			l.SharePercent == nil || *l.SharePercent != w.share {
			t.Errorf("line %d = %+v (share %v), want %+v", i, l, l.SharePercent, w)
		}
	}
}

func TestPurposeReportWithoutIncomeHasNoShares(t *testing.T) {
	_, s := setup(t)
	spend(t, s, purposeCategory(t, s, "Rent", PurposeNeed), "2026-11-03", 10000, false)
	for _, l := range reportOf(t, s, "", "2026-11-20").Lines {
		if l.SharePercent != nil {
			t.Errorf("%q has a share without income", l.Purpose)
		}
	}
}

func TestPurposeReportComparesWithThePeriodBefore(t *testing.T) {
	_, s := setup(t)
	need := purposeCategory(t, s, "Rent", PurposeNeed)
	spend(t, s, need, "2026-10-31", 9000, false) // last day of the period before
	spend(t, s, need, "2026-11-01", 10000, false)
	spend(t, s, need, "2026-12-01", 7000, false) // the period after

	r := reportOf(t, s, "", "2026-11-20")
	if l := purposeLine(t, r, PurposeNeed); l.Cents != 10000 || l.PreviousCents != 9000 {
		t.Errorf("need = %+v, want 10000 now and 9000 before", l)
	}
	if r.PreviousMonth != "2026-10" {
		t.Errorf("previous month = %q", r.PreviousMonth)
	}
	// Asking for the earlier month shows it with its own predecessor.
	if l := purposeLine(t, reportOf(t, s, "2026-10", "2026-11-20"), PurposeNeed); l.Cents != 9000 || l.PreviousCents != 0 {
		t.Errorf("October need = %+v", l)
	}
}

func TestPurposeReportUsesTheDateOfTheAppAndRejectsBadInput(t *testing.T) {
	_, s := setup(t)
	if got := reportOf(t, s, "", "2026-11-20").Month; got != "2026-11" {
		t.Errorf("month = %q, want the current one", got)
	}
	if _, err := s.PurposeReport(context.Background(), "l1", "2026-13", "2026-11-20"); !errors.Is(err, ErrInvalidMonth) {
		t.Errorf("bad month: err = %v, want ErrInvalidMonth", err)
	}
	if _, err := s.PurposeReport(context.Background(), "l1", "", "20.11.2026"); !errors.Is(err, ErrInvalidDate) {
		t.Errorf("bad date: err = %v, want ErrInvalidDate", err)
	}
}

func TestPurposeReportIgnoresDeletedExpenses(t *testing.T) {
	_, s := setup(t)
	need := purposeCategory(t, s, "Rent", PurposeNeed)
	id := spend(t, s, need, "2026-11-03", 10000, false)
	if err := s.DeleteTransaction(context.Background(), "l1", id, "u1", time.Now()); err != nil {
		t.Fatal(err)
	}
	if l := purposeLine(t, reportOf(t, s, "", "2026-11-20"), PurposeNeed); l.Cents != 0 {
		t.Errorf("need = %d after deleting, want 0", l.Cents)
	}
}

func TestMoneyMovedIntoSavingsPotsCountsAsSaving(t *testing.T) {
	_, s := setup(t)
	saving := purposeCategory(t, s, "Pension", PurposeSaving)
	spend(t, s, saving, "2026-11-02", 2000, false)
	moveToPot(t, s, "p-default", "p-holiday", 5000, "2026-11-10")
	moveToPot(t, s, "p-holiday", "p-default", 1500, "2026-11-12") // taken back out

	l := purposeLine(t, reportOf(t, s, "", "2026-11-20"), PurposeSaving)
	if l.TransferCents != 3500 || l.Cents != 5500 {
		t.Errorf("saving = %+v, want 3500 moved and 5500 in all", l)
	}
	// Only the line of savings carries transfers.
	for _, o := range reportOf(t, s, "", "2026-11-20").Lines {
		if o.Purpose != PurposeSaving && o.TransferCents != 0 {
			t.Errorf("%q has transfers", o.Purpose)
		}
	}
}

func TestWithdrawalsNeverMakeSavingsNegative(t *testing.T) {
	_, s := setup(t)
	moveToPot(t, s, "p-default", "p-holiday", 1000, "2026-10-10")
	moveToPot(t, s, "p-holiday", "p-default", 1000, "2026-11-10")

	r := reportOf(t, s, "", "2026-11-20")
	if l := purposeLine(t, r, PurposeSaving); l.Cents != 0 || l.TransferCents != 0 || l.PreviousCents != 1000 {
		t.Errorf("saving = %+v, want 0 now (not -1000) and 1000 before", l)
	}
}

func TestMovingMoneyBetweenSavingsPotsIsNoSaving(t *testing.T) {
	_, s := setup(t)
	bike, err := s.CreatePot(context.Background(), NewPot{LedgerID: "l1", Name: "Bike"})
	if err != nil {
		t.Fatal(err)
	}
	moveToPot(t, s, "p-holiday", bike, 4000, "2026-11-10")
	if l := purposeLine(t, reportOf(t, s, "", "2026-11-20"), PurposeSaving); l.Cents != 0 {
		t.Errorf("saving = %d, want 0 for a move between savings pots", l.Cents)
	}
}

func TestMoneyForReservesIsNoSaving(t *testing.T) {
	_, s := setup(t)
	reserve := loadPayment(t, s, mustCreateRecurring(t, s, insurance())).ReservePotID
	moveToPot(t, s, "p-default", reserve, 10000, "2026-11-10")

	if l := purposeLine(t, reportOf(t, s, "", "2026-11-20"), PurposeSaving); l.Cents != 0 {
		t.Errorf("saving = %d, want 0 for money moved into a reserve", l.Cents)
	}
}

func TestExpensesFromAnyPotCountOnTheirBookingDay(t *testing.T) {
	_, s := setup(t)
	want := purposeCategory(t, s, "Travel", PurposeWant)
	in := baseExpense()
	in.CategoryID, in.PotID, in.BookedOn, in.AmountCents = want, "p-holiday", "2026-11-09", 4000
	if _, err := s.CreateTransaction(context.Background(), in, time.Now()); err != nil {
		t.Fatal(err)
	}
	if l := purposeLine(t, reportOf(t, s, "", "2026-11-20"), PurposeWant); l.Cents != 4000 {
		t.Errorf("want = %d, want the expense from the Holiday pot", l.Cents)
	}
}
