package ledger

import (
	"context"
	"errors"
	"testing"
)

// setGoal gives the holiday pot a goal; 0 and "" leave a part out.
func setGoal(t *testing.T, s *Store, target int64, date string, rate int64) {
	t.Helper()
	err := s.UpdatePot(context.Background(), "l1", "p-holiday", PotUpdate{
		TargetCents: &target, TargetDate: &date, MonthlyRateCents: &rate,
	})
	if err != nil {
		t.Fatalf("set goal: %v", err)
	}
}

func onlyGoal(t *testing.T, s *Store) SavingsGoal {
	t.Helper()
	goals, err := s.SavingsGoals(context.Background(), "l1", today)
	if err != nil {
		t.Fatalf("savings goals: %v", err)
	}
	if len(goals) != 1 {
		t.Fatalf("goals = %+v, want one", goals)
	}
	return goals[0]
}

func TestGoalWithDateSpreadsTheTargetOverThePeriods(t *testing.T) {
	_, s := setup(t)
	// 1200 EUR by February: November to February are four periods.
	setGoal(t, s, 120000, "2027-02-10", 0)
	moveToPot(t, s, "p-default", "p-holiday", 10000, today)

	g := onlyGoal(t, s)
	if g.TargetMonth != "2027-02" || g.Periods != 4 || g.RateCents != 30000 {
		t.Errorf("target month %s, periods %d, rate %d; want 2027-02, 4, 30000", g.TargetMonth, g.Periods, g.RateCents)
	}
	// Saving during the period lowers what is open, not the rate.
	if g.SavedCents != 10000 || g.OpenCents != 20000 || g.Reached || g.DatePassed {
		t.Errorf("goal = %+v, want 10000 saved, 20000 open", g)
	}
}

func TestPassedTargetDateMakesTheRestDueNow(t *testing.T) {
	_, s := setup(t)
	moveToPot(t, s, "p-default", "p-holiday", 10000, "2026-10-05")
	setGoal(t, s, 50000, "2026-10-31", 0)

	g := onlyGoal(t, s)
	if !g.DatePassed || g.Periods != 1 || g.RateCents != 40000 || g.OpenCents != 40000 {
		t.Errorf("goal = %+v, want date passed and the rest of 40000 due", g)
	}
}

func TestFixedRateEndsAtTheTarget(t *testing.T) {
	cases := []struct {
		name    string
		target  int64
		before  int64 // moved in October, before this period
		rate    int64
		reached bool
	}{
		{"without target", 0, 40000, 20000, false},
		{"target far away", 100000, 40000, 20000, false},
		{"only the rest", 50000, 40000, 10000, false},
		{"target reached", 40000, 40000, 0, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, s := setup(t)
			moveToPot(t, s, "p-default", "p-holiday", tc.before, "2026-10-05")
			setGoal(t, s, tc.target, "", 20000)

			g := onlyGoal(t, s)
			if g.RateCents != tc.rate || g.Reached != tc.reached || g.Periods != 0 {
				t.Errorf("rate %d, reached %v, periods %d; want %d, %v, 0", g.RateCents, g.Reached, g.Periods, tc.rate, tc.reached)
			}
		})
	}
}

func TestOnlyPotsWithARateAreSavingsGoals(t *testing.T) {
	_, s := setup(t)
	ctx := context.Background()
	// A target without a date or rate only shows progress.
	setGoal(t, s, 50000, "", 0)
	archivedID, err := s.CreatePot(ctx, NewPot{LedgerID: "l1", Name: "Old", MonthlyRateCents: 5000})
	if err != nil {
		t.Fatal(err)
	}
	archived := true
	if err := s.UpdatePot(ctx, "l1", archivedID, PotUpdate{Archived: &archived}); err != nil {
		t.Fatal(err)
	}

	goals, err := s.SavingsGoals(ctx, "l1", today)
	if err != nil {
		t.Fatal(err)
	}
	if len(goals) != 0 {
		t.Errorf("goals = %+v, want none", goals)
	}
}

func TestGoalRulesAreChecked(t *testing.T) {
	_, s := setup(t)
	ctx := context.Background()
	reserve := loadPayment(t, s, mustCreateRecurring(t, s, insurance())).ReservePotID
	rate, negative := int64(5000), int64(-1)
	date, target := "2027-01-01", int64(10000)

	cases := []struct {
		name string
		pot  string
		u    PotUpdate
		want error
	}{
		{"rate and date", "p-holiday", PotUpdate{TargetCents: &target, TargetDate: &date, MonthlyRateCents: &rate}, ErrRateWithTargetDate},
		{"negative rate", "p-holiday", PotUpdate{MonthlyRateCents: &negative}, ErrInvalidRate},
		{"main pot", "p-default", PotUpdate{MonthlyRateCents: &rate}, ErrGoalRateNotAllowed},
		{"reserve", reserve, PotUpdate{MonthlyRateCents: &rate}, ErrGoalRateNotAllowed},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if err := s.UpdatePot(ctx, "l1", tc.pot, tc.u); !errors.Is(err, tc.want) {
				t.Errorf("err = %v, want %v", err, tc.want)
			}
		})
	}
	if _, err := s.CreatePot(ctx, NewPot{LedgerID: "l1", Name: "Car", TargetCents: target, TargetDate: date, MonthlyRateCents: rate}); !errors.Is(err, ErrRateWithTargetDate) {
		t.Errorf("create with rate and date: err = %v, want ErrRateWithTargetDate", err)
	}
}

func TestBudgetShowsSavingsGoalsPerMonth(t *testing.T) {
	_, s := setup(t)
	moveToPot(t, s, "p-default", "p-holiday", 5000, "2026-10-05")
	setGoal(t, s, 125000, "2027-02-10", 0) // (1250 − 50) / 4 = 300 EUR
	moveToPot(t, s, "p-default", "p-holiday", 10000, today)

	cases := []struct {
		month   string
		planned int64 // -1: none stored
		actual  int64
		preview bool
		lines   int
	}{
		{"2026-10", -1, 5000, false, 1},
		{"2026-11", 30000, 10000, false, 1},
		{"2027-02", 30000, 0, true, 1},
		{"2027-03", 0, 0, false, 0}, // after the target month
	}
	for _, tc := range cases {
		lines := budgetFor(t, s, tc.month).Savings
		if len(lines) != tc.lines {
			t.Fatalf("%s: savings = %+v, want %d lines", tc.month, lines, tc.lines)
		}
		if tc.lines == 0 {
			continue
		}
		l := lines[0]
		planned := int64(-1)
		if l.PlannedCents != nil {
			planned = *l.PlannedCents
		}
		if l.PotID != "p-holiday" || planned != tc.planned || l.ActualCents != tc.actual || l.Preview != tc.preview {
			t.Errorf("%s: line = %+v (planned %d), want planned %d, actual %d, preview %v",
				tc.month, l, planned, tc.planned, tc.actual, tc.preview)
		}
	}
}

func TestBudgetPreviewEndsAFixedRateAtTheTarget(t *testing.T) {
	_, s := setup(t)
	setGoal(t, s, 50000, "", 20000) // 200 EUR a month up to 500 EUR

	for month, want := range map[string]int64{"2026-12": 20000, "2027-01": 10000, "2027-02": -1} {
		lines := budgetFor(t, s, month).Savings
		got := int64(-1)
		if len(lines) == 1 {
			got = *lines[0].PlannedCents
		}
		if got != want {
			t.Errorf("%s: preview rate %d, want %d (-1: no line)", month, got, want)
		}
	}
}

func TestSafeToSpendDeductsOpenSavings(t *testing.T) {
	_, s := setup(t)
	setGoal(t, s, 0, "", 20000)
	moveToPot(t, s, "p-default", "p-holiday", 5000, today)

	got := safeToSpendOn(t, s, today)
	if got.SavingsCents != 15000 || got.AmountCents != -5000-15000 {
		t.Errorf("savings %d, amount %d; want 15000, -20000", got.SavingsCents, got.AmountCents)
	}
}
