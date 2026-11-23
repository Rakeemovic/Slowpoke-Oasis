package ledger

import (
	"context"
	"errors"
	"testing"
)

// today is the app's date in the plan tests. The ledger from setup has its
// payday on the 1st, so the current month is 2026-11.
const today = "2026-11-16"

func planChange(potID, categoryID string, amountCents int64, scope PlanScope) PlanChange {
	return PlanChange{
		LedgerID: "l1", PotID: potID, CategoryID: categoryID,
		AmountCents: amountCents, Scope: scope, Today: today,
	}
}

func monthQuery(potID, month string) SummaryQuery {
	return SummaryQuery{LedgerID: "l1", PotID: potID, Month: month, Today: today}
}

// plannedFor returns the plan source and the planned amount of a category in
// the summary of a monthly pot (0 if the category has no line).
func plannedFor(t *testing.T, s *Store, potID, month, categoryID string) (PlanSource, int64) {
	t.Helper()
	sum, err := s.PotSummary(context.Background(), monthQuery(potID, month))
	if err != nil {
		t.Fatalf("summary of %s: %v", month, err)
	}
	for _, l := range sum.Lines {
		if l.CategoryID == categoryID {
			return sum.PlanSource, l.PlannedCents
		}
	}
	return sum.PlanSource, 0
}

func TestMonthOf(t *testing.T) {
	cases := []struct {
		date   string
		payday int
		want   string
	}{
		{"2026-11-16", 1, "2026-11"},
		{"2026-11-24", 25, "2026-10"}, // before the payday: still last month's period
		{"2026-11-25", 25, "2026-11"},
		{"2026-01-15", 20, "2025-12"}, // across the turn of the year
		{"2026-02-28", 31, "2026-02"}, // payday 31 falls back to the 28th
		{"2026-03-30", 31, "2026-02"},
		{"2026-03-31", 31, "2026-03"},
	}
	for _, tc := range cases {
		got, err := MonthOf(tc.date, tc.payday)
		if err != nil || got != tc.want {
			t.Errorf("MonthOf(%s, %d) = %q, %v; want %q", tc.date, tc.payday, got, err, tc.want)
		}
	}
	if _, err := MonthOf("2026-13-01", 1); !errors.Is(err, ErrInvalidDate) {
		t.Errorf("invalid date: err = %v, want ErrInvalidDate", err)
	}
}

func TestMonthlyPlanIsCreatedFromTemplateOnFirstAccess(t *testing.T) {
	db, s := setup(t)
	mustExec(t, db, `INSERT INTO pot_line (pot_id, ledger_id, category_id, amount_cents) VALUES ('p-default', 'l1', 'c-food', 30000)`)

	if src, planned := plannedFor(t, s, "p-default", "", "c-food"); src != PlanOfMonth || planned != 30000 {
		t.Fatalf("current month = %s/%d, want month/30000", src, planned)
	}

	// A later change of the template leaves the stored month alone, but shows
	// in the preview of the months to come.
	mustExec(t, db, `UPDATE pot_line SET amount_cents = 50000 WHERE pot_id = 'p-default'`)
	if src, planned := plannedFor(t, s, "p-default", "2026-11", "c-food"); src != PlanOfMonth || planned != 30000 {
		t.Errorf("current month after template change = %s/%d, want month/30000", src, planned)
	}
	if src, planned := plannedFor(t, s, "p-default", "2026-12", "c-food"); src != PlanPreview || planned != 50000 {
		t.Errorf("next month = %s/%d, want preview/50000", src, planned)
	}
	var stored int
	if err := db.QueryRow(`SELECT COUNT(*) FROM period_plan WHERE month = '2026-12'`).Scan(&stored); err != nil || stored != 0 {
		t.Errorf("preview stored %d lines, %v; want 0", stored, err)
	}

	// Past months are never filled in afterwards.
	if src, planned := plannedFor(t, s, "p-default", "2026-10", "c-food"); src != PlanNotStored || planned != 0 {
		t.Errorf("past month = %s/%d, want none/0", src, planned)
	}
}

func TestPastMonthKeepsItsPlan(t *testing.T) {
	_, s := setup(t)
	ctx := context.Background()

	october := planChange("p-default", "c-food", 12000, ScopeThisMonth)
	october.Today = "2026-10-05"
	if err := s.SetPlannedAmount(ctx, october); err != nil {
		t.Fatal(err)
	}
	if src, planned := plannedFor(t, s, "p-default", "2026-10", "c-food"); src != PlanOfMonth || planned != 12000 {
		t.Errorf("October seen in November = %s/%d, want month/12000", src, planned)
	}
}

func TestPlanScopes(t *testing.T) {
	db, s := setup(t)
	ctx := context.Background()

	if err := s.SetPlannedAmount(ctx, planChange("p-default", "c-food", 20000, ScopeThisMonth)); err != nil {
		t.Fatal(err)
	}
	if _, planned := plannedFor(t, s, "p-default", "2026-11", "c-food"); planned != 20000 {
		t.Errorf("this month = %d, want 20000", planned)
	}
	if _, planned := plannedFor(t, s, "p-default", "2026-12", "c-food"); planned != 0 {
		t.Errorf("next month after this_month = %d, want 0 (template unchanged)", planned)
	}

	if err := s.SetPlannedAmount(ctx, planChange("p-default", "c-food", 25000, ScopeFromNowOn)); err != nil {
		t.Fatal(err)
	}
	if _, planned := plannedFor(t, s, "p-default", "2026-11", "c-food"); planned != 25000 {
		t.Errorf("this month after from_now_on = %d, want 25000", planned)
	}
	if _, planned := plannedFor(t, s, "p-default", "2026-12", "c-food"); planned != 25000 {
		t.Errorf("next month after from_now_on = %d, want 25000", planned)
	}

	// Removing for this month only keeps the template, and the month is not
	// filled from the template again.
	if err := s.DeletePlannedAmount(ctx, planChange("p-default", "c-food", 0, ScopeThisMonth)); err != nil {
		t.Fatal(err)
	}
	if _, planned := plannedFor(t, s, "p-default", "2026-11", "c-food"); planned != 0 {
		t.Errorf("this month after removal = %d, want 0", planned)
	}
	if err := s.DeletePlannedAmount(ctx, planChange("p-default", "c-food", 0, ScopeThisMonth)); !errors.Is(err, ErrPotLineNotFound) {
		t.Errorf("second removal: err = %v, want ErrPotLineNotFound", err)
	}
	if err := s.DeletePlannedAmount(ctx, planChange("p-default", "c-food", 0, ScopeFromNowOn)); err != nil {
		t.Fatalf("remove template: %v", err)
	}
	var templates int
	if err := db.QueryRow(`SELECT COUNT(*) FROM pot_line`).Scan(&templates); err != nil || templates != 0 {
		t.Errorf("templates = %d, %v; want 0", templates, err)
	}
}

func TestPlanScopeMustFitThePot(t *testing.T) {
	_, s := setup(t)
	ctx := context.Background()

	if err := s.SetPlannedAmount(ctx, planChange("p-default", "c-food", 100, "")); !errors.Is(err, ErrInvalidPlanScope) {
		t.Errorf("monthly pot without scope: err = %v, want ErrInvalidPlanScope", err)
	}
	if err := s.SetPlannedAmount(ctx, planChange("p-holiday", "c-food", 100, ScopeThisMonth)); !errors.Is(err, ErrInvalidPlanScope) {
		t.Errorf("one-time pot with scope: err = %v, want ErrInvalidPlanScope", err)
	}
	if err := s.SetPlannedAmount(ctx, planChange("nope", "c-food", 100, "")); !errors.Is(err, ErrPotNotFound) {
		t.Errorf("unknown pot: err = %v, want ErrPotNotFound", err)
	}
	q := monthQuery("p-default", "")
	q.From = "2026-11-01"
	if _, err := s.PotSummary(ctx, q); !errors.Is(err, ErrMonthlyPotNeedsMonth) {
		t.Errorf("monthly pot with from: err = %v, want ErrMonthlyPotNeedsMonth", err)
	}
}

func TestCategoryIsPlannedInOneMonthlyPotOnly(t *testing.T) {
	db, s := setup(t)
	ctx := context.Background()
	mustExec(t, db, `INSERT INTO pot (id, ledger_id, name, kind) VALUES ('p-car', 'l1', 'Car', 'monthly')`)

	if err := s.SetPlannedAmount(ctx, planChange("p-default", "c-food", 100, ScopeFromNowOn)); err != nil {
		t.Fatal(err)
	}
	for _, scope := range []PlanScope{ScopeThisMonth, ScopeFromNowOn} {
		if err := s.SetPlannedAmount(ctx, planChange("p-car", "c-food", 100, scope)); !errors.Is(err, ErrCategoryPlannedElsewhere) {
			t.Errorf("%s in a second monthly pot: err = %v, want ErrCategoryPlannedElsewhere", scope, err)
		}
	}
	// One-time pots are not limited by rule P3.
	if err := s.SetPlannedAmount(ctx, planChange("p-holiday", "c-food", 100, "")); err != nil {
		t.Errorf("one-time pot: %v", err)
	}
}

func TestDeleteCategoryMovesMonthlyPlans(t *testing.T) {
	db, s := setup(t)
	ctx := context.Background()

	garden, err := s.CreateCategory(ctx, NewCategory{LedgerID: "l1", Name: "Garden"})
	if err != nil {
		t.Fatal(err)
	}
	if err := s.SetPlannedAmount(ctx, planChange("p-default", "c-food", 1000, ScopeThisMonth)); err != nil {
		t.Fatal(err)
	}
	if err := s.SetPlannedAmount(ctx, planChange("p-default", garden, 500, ScopeFromNowOn)); err != nil {
		t.Fatal(err)
	}
	if _, err := s.DeleteCategory(ctx, "l1", "c-food", garden); err != nil {
		t.Fatal(err)
	}

	var planned int64
	if err := db.QueryRow(`SELECT amount_cents FROM period_plan WHERE category_id = ? AND month = '2026-11'`, garden).Scan(&planned); err != nil || planned != 1500 {
		t.Errorf("Garden in November = %d, %v; want 1500", planned, err)
	}
}
