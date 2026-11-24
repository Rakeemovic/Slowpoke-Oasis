package ledger

import (
	"context"
	"testing"
	"time"
)

func budgetFor(t *testing.T, s *Store, month string) Budget {
	t.Helper()
	b, err := s.Budget(context.Background(), BudgetQuery{LedgerID: "l1", Month: month, Today: today})
	if err != nil {
		t.Fatalf("budget of %q: %v", month, err)
	}
	return b
}

// groceries returns the category c-food of a budget.
func groceries(t *testing.T, b Budget) BudgetCategory {
	t.Helper()
	for _, c := range b.Categories {
		if c.CategoryID == "c-food" {
			return c
		}
	}
	t.Fatalf("budget %s has no groceries: %+v", b.Month, b.Categories)
	return BudgetCategory{}
}

func TestBudgetKeepsRecurringBookingsOutOfThePlan(t *testing.T) {
	_, s := setup(t)
	ctx := context.Background()
	rentID := mustCreateRecurring(t, s, rent())
	if err := s.SetPlannedAmount(ctx, planChange("p-default", "c-food", 30000, ScopeThisMonth)); err != nil {
		t.Fatal(err)
	}
	if _, err := s.CreateTransaction(ctx, baseExpense(), time.Now()); err != nil {
		t.Fatal(err)
	}
	if _, err := payOccurrence(s, rentID, "2026-11-20"); err != nil {
		t.Fatal(err)
	}

	b := budgetFor(t, s, "")
	if b.Month != "2026-11" || b.Current != "2026-11" || b.PotID != "p-default" {
		t.Errorf("month %s, current %s, pot %s", b.Month, b.Current, b.PotID)
	}
	c := groceries(t, b)
	if c.PlannedCents != 30000 || c.ActualCents != 1250 || c.DifferenceCents != 28750 {
		t.Errorf("plan line = %+v, want 30000 planned, 1250 actual", c.CategoryLine)
	}
	if len(c.Recurring) != 1 {
		t.Fatalf("recurring lines = %+v, want one", c.Recurring)
	}
	want := RecurringLine{
		PaymentID: rentID, Name: "Rent", DueOn: "2026-11-20",
		PlannedCents: 80000, ActualCents: 1250, DifferenceCents: 78750,
	}
	if c.Recurring[0] != want {
		t.Errorf("recurring line = %+v, want %+v", c.Recurring[0], want)
	}
}

func TestRecurringBookingCountsInTheMonthOfItsDueDate(t *testing.T) {
	_, s := setup(t)
	ctx := context.Background()
	rentID := mustCreateRecurring(t, s, rent())
	in := baseExpense()
	in.RecurringPaymentID, in.RecurringDueOn, in.BookedOn = rentID, "2026-11-20", "2026-12-02"
	if _, err := s.CreateTransaction(ctx, in, time.Now()); err != nil {
		t.Fatal(err)
	}

	if got := groceries(t, budgetFor(t, s, "2026-11")).Recurring; len(got) != 1 || got[0].ActualCents != 1250 {
		t.Errorf("November: %+v, want the booking", got)
	}
	dec := groceries(t, budgetFor(t, s, "2026-12"))
	if dec.ActualCents != 0 || len(dec.Recurring) != 1 || dec.Recurring[0].ActualCents != 0 {
		t.Errorf("December: %+v, want nothing booked", dec)
	}
	if dec.Recurring[0].DueOn != "2026-12-20" || dec.Recurring[0].PlannedCents != 80000 {
		t.Errorf("December preview = %+v, want rent due on 2026-12-20", dec.Recurring[0])
	}
}

func TestSkippedOccurrenceStaysVisibleWithoutPlan(t *testing.T) {
	_, s := setup(t)
	rentID := mustCreateRecurring(t, s, rent())
	if err := s.SkipOccurrence(context.Background(), "l1", rentID, "2026-11-20"); err != nil {
		t.Fatal(err)
	}
	got := groceries(t, budgetFor(t, s, "")).Recurring
	if len(got) != 1 || !got[0].Skipped || got[0].PlannedCents != 0 {
		t.Errorf("recurring lines = %+v, want one skipped line without plan", got)
	}
}

func TestRecurringOfOtherPotsIsNotInTheBudget(t *testing.T) {
	_, s := setup(t)
	in := rent()
	in.PotID = "p-holiday"
	mustCreateRecurring(t, s, in)
	for _, c := range budgetFor(t, s, "").Categories {
		if len(c.Recurring) > 0 {
			t.Errorf("category %s has recurring lines %+v", c.Name, c.Recurring)
		}
	}
}

func TestBudgetShowsReserveRates(t *testing.T) {
	_, s := setup(t)
	id := mustCreateRecurring(t, s, insurance())
	reserve := loadPayment(t, s, id).ReservePotID
	moveToPot(t, s, "p-default", reserve, 10000, "2026-11-16")
	moveToPot(t, s, "p-default", reserve, 5000, "2026-10-10")

	cases := []struct {
		month   string
		due     string
		planned int64 // -1: no plan known
		actual  int64
		preview bool
	}{
		// 1200 EUR minus the 50 EUR saved before this period, over four
		// periods (November to February).
		{"2026-11", "2027-02-15", 28750, 10000, false},
		{"2026-12", "2027-02-15", 28750, 0, true},
		// After February, the next cycle spreads the amount over 3 months.
		{"2027-03", "2027-05-15", 40000, 0, true},
		{"2026-10", "", -1, 5000, false},
	}
	for _, tc := range cases {
		lines := groceries(t, budgetFor(t, s, tc.month)).Reserves
		if len(lines) != 1 {
			t.Fatalf("%s: reserve lines = %+v, want one", tc.month, lines)
		}
		l := lines[0]
		planned := int64(-1)
		if l.PlannedCents != nil {
			planned = *l.PlannedCents
		}
		if l.DueOn != tc.due || planned != tc.planned || l.ActualCents != tc.actual || l.Preview != tc.preview || l.PotID != reserve {
			t.Errorf("%s: line = %+v (planned %d), want due %s, planned %d, actual %d, preview %v",
				tc.month, l, planned, tc.due, tc.planned, tc.actual, tc.preview)
		}
	}
}

func TestReserveDueIsShownInItsMonth(t *testing.T) {
	_, s := setup(t)
	id := mustCreateRecurring(t, s, insurance())
	reserve := loadPayment(t, s, id).ReservePotID
	moveToPot(t, s, "p-default", reserve, 10000, "2026-11-16")

	if dues := budgetFor(t, s, "2027-01").ReserveDues; len(dues) != 0 {
		t.Errorf("January: dues = %+v, want none", dues)
	}
	dues := budgetFor(t, s, "2027-02").ReserveDues
	want := ReserveDue{
		PaymentID: id, Name: "Car insurance", CategoryID: "c-food", PotID: reserve,
		DueOn: "2027-02-15", AmountCents: 120000, CoveredCents: 10000,
	}
	if len(dues) != 1 || dues[0] != want {
		t.Errorf("February: dues = %+v, want %+v", dues, want)
	}
	// The payment is paid from the reserve, so it is no recurring line.
	if c := groceries(t, budgetFor(t, s, "2027-02")); len(c.Recurring) != 0 {
		t.Errorf("February: recurring lines = %+v, want none", c.Recurring)
	}
}
