package ledger

import (
	"context"
	"errors"
	"testing"
	"time"
)

func safeToSpendOn(t *testing.T, s *Store, on string) SafeToSpend {
	t.Helper()
	got, err := s.SafeToSpend(context.Background(), "l1", on)
	if err != nil {
		t.Fatalf("safe to spend on %s: %v", on, err)
	}
	return got
}

func TestSafeToSpendDeductsPaymentsRatesAndBudget(t *testing.T) {
	_, s := setup(t)
	ctx := context.Background()

	// 2000 EUR salary, 12.50 EUR groceries out of a budget of 300 EUR.
	salary := baseExpense()
	salary.Type, salary.CategoryID, salary.AmountCents = TypeIncome, "c-salary", 200000
	if _, err := s.CreateTransaction(ctx, salary, time.Now()); err != nil {
		t.Fatal(err)
	}
	if _, err := s.CreateTransaction(ctx, baseExpense(), time.Now()); err != nil {
		t.Fatal(err)
	}
	if err := s.SetPlannedAmount(ctx, planChange("p-default", "c-food", 30000, ScopeThisMonth)); err != nil {
		t.Fatal(err)
	}
	// Rent (800 EUR on the 20th) is still open. The insurance reserve needs
	// 1200 EUR in four periods, so 300 EUR now; 100 EUR are saved already.
	mustCreateRecurring(t, s, rent())
	reserve := loadPayment(t, s, mustCreateRecurring(t, s, insurance())).ReservePotID
	moveToPot(t, s, "p-default", reserve, 10000, today)

	got := safeToSpendOn(t, s, today)
	want := SafeToSpend{
		AsOf: today, PeriodEnd: "2026-11-30", DaysLeft: 15,
		MainPotCents:   188750,                   // 2000 − 12.50 − 100
		RecurringCents: 80000, RecurringCount: 1, // the insurance is paid from its reserve
		ReserveRatesCents: 20000,
		BudgetLeftCents:   28750,
		AmountCents:       60000,
		PerDayCents:       4000,
	}
	if got != want {
		t.Errorf("got  %+v\nwant %+v", got, want)
	}
}

func TestSafeToSpendCountsOverduePaymentsOfTheMainPotOnly(t *testing.T) {
	_, s := setup(t)
	mustCreateRecurring(t, s, rent())
	// Paid from another pot, so it does not touch the main pot.
	other := rent()
	other.Name, other.PotID = "Gym", "p-holiday"
	mustCreateRecurring(t, s, other)

	// In December the rent of November is overdue and December's is due.
	got := safeToSpendOn(t, s, "2026-12-21")
	if got.RecurringCount != 2 || got.RecurringCents != 160000 {
		t.Errorf("recurring = %d (%d cents), want 2 (160000)", got.RecurringCount, got.RecurringCents)
	}
	if got.DaysLeft != 11 || got.PeriodEnd != "2026-12-31" {
		t.Errorf("days left %d until %s, want 11 until 2026-12-31", got.DaysLeft, got.PeriodEnd)
	}
	// Over-planned: the amount is negative and there is nothing per day.
	if got.AmountCents != -160000 || got.PerDayCents != 0 {
		t.Errorf("amount %d, per day %d; want -160000, 0", got.AmountCents, got.PerDayCents)
	}
}

func TestSafeToSpendIgnoresOverspentCategoriesAndRecurringBookings(t *testing.T) {
	_, s := setup(t)
	ctx := context.Background()
	rentID := mustCreateRecurring(t, s, rent())
	if err := s.SetPlannedAmount(ctx, planChange("p-default", "c-food", 1000, ScopeThisMonth)); err != nil {
		t.Fatal(err)
	}
	// 12.50 EUR spent of 10 EUR: the category counts 0, not −2.50.
	if _, err := s.CreateTransaction(ctx, baseExpense(), time.Now()); err != nil {
		t.Fatal(err)
	}
	// The booked rent is no longer open and does not use up the budget.
	if _, err := payOccurrence(s, rentID, "2026-11-20"); err != nil {
		t.Fatal(err)
	}

	got := safeToSpendOn(t, s, today)
	if got.RecurringCents != 0 || got.BudgetLeftCents != 0 {
		t.Errorf("recurring %d, budget left %d; want 0, 0", got.RecurringCents, got.BudgetLeftCents)
	}
	if got.MainPotCents != -2500 || got.AmountCents != -2500 {
		t.Errorf("main pot %d, amount %d; want -2500, -2500", got.MainPotCents, got.AmountCents)
	}
}

func TestSafeToSpendRejectsAnInvalidDate(t *testing.T) {
	_, s := setup(t)
	if _, err := s.SafeToSpend(context.Background(), "l1", "2026-13-01"); !errors.Is(err, ErrInvalidDate) {
		t.Errorf("err = %v, want ErrInvalidDate", err)
	}
}
