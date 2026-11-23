package ledger

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

// balances returns the balance of each pot in a ledger, keyed by pot ID.
func balances(t *testing.T, s *Store, ledgerID string) map[string]int64 {
	t.Helper()
	pots, err := s.ListPots(context.Background(), ledgerID)
	if err != nil {
		t.Fatalf("list pots: %v", err)
	}
	out := make(map[string]int64, len(pots))
	for _, p := range pots {
		out[p.ID] = p.BalanceCents
	}
	return out
}

func TestPotBalanceFollowsIncomeExpensesAndTransfers(t *testing.T) {
	_, s := setup(t)
	ctx := context.Background()

	// Salary of 500 EUR into the default pot.
	in := baseExpense()
	in.Type = TypeIncome
	in.CategoryID = "c-salary"
	in.AmountCents = 50000
	if _, err := s.CreateTransaction(ctx, in, time.Now()); err != nil {
		t.Fatalf("income: %v", err)
	}

	// 200 EUR moved from the default pot into the holiday pot.
	transfer := NewTransaction{
		LedgerID: "l1", UserID: "u1", Type: TypeTransfer,
		PotID: "p-default", CounterpartPotID: "p-holiday",
		AmountCents: 20000, BookedOn: "2026-11-16",
	}
	if _, err := s.CreateTransaction(ctx, transfer, time.Now()); err != nil {
		t.Fatalf("transfer: %v", err)
	}

	// 30 EUR spent from the holiday pot.
	holidayExpense := createExpense(t, s, "p-holiday", "2026-11-16", 3000)

	got := balances(t, s, "l1")
	if got["p-default"] != 30000 {
		t.Errorf("default balance = %d, want 30000", got["p-default"])
	}
	if got["p-holiday"] != 17000 {
		t.Errorf("holiday balance = %d, want 17000", got["p-holiday"])
	}

	// A soft-deleted expense no longer counts.
	if err := s.DeleteTransaction(ctx, "l1", holidayExpense, "u1", time.Now()); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if got := balances(t, s, "l1")["p-holiday"]; got != 20000 {
		t.Errorf("holiday balance after delete = %d, want 20000", got)
	}
}

func TestCreatePotValidatesTargets(t *testing.T) {
	db, s := setup(t)
	ctx := context.Background()

	id, err := s.CreatePot(ctx, NewPot{LedgerID: "l1", Name: "Driving licence", TargetCents: 150000, TargetDate: "2028-06-30"})
	if err != nil {
		t.Fatalf("valid pot: %v", err)
	}
	var kind string
	if err := db.QueryRow(`SELECT kind FROM pot WHERE id = ?`, id).Scan(&kind); err != nil || kind != "one_time" {
		t.Errorf("kind = %q (err %v), want one_time", kind, err)
	}

	if _, err := s.CreatePot(ctx, NewPot{LedgerID: "l1", Name: "No target", TargetDate: "2028-06-30"}); !errors.Is(err, ErrTargetDateNeedsTarget) {
		t.Errorf("date without target: err = %v, want ErrTargetDateNeedsTarget", err)
	}
	if _, err := s.CreatePot(ctx, NewPot{LedgerID: "l1", Name: "Negative", TargetCents: -1}); !errors.Is(err, ErrInvalidTarget) {
		t.Errorf("negative target: err = %v, want ErrInvalidTarget", err)
	}
	if _, err := s.CreatePot(ctx, NewPot{LedgerID: "l1", Name: "holiday"}); err == nil {
		t.Error("duplicate pot name (case-insensitive) should be rejected")
	}
}

func TestDefaultPotCannotBeArchived(t *testing.T) {
	_, s := setup(t)
	archive := true
	err := s.UpdatePot(context.Background(), "l1", "p-default", PotUpdate{Archived: &archive})
	if !errors.Is(err, ErrDefaultPotProtected) {
		t.Errorf("err = %v, want ErrDefaultPotProtected", err)
	}
}

func TestArchivedPotRejectsNewTransactions(t *testing.T) {
	_, s := setup(t)
	ctx := context.Background()

	archive := true
	if err := s.UpdatePot(ctx, "l1", "p-holiday", PotUpdate{Archived: &archive}); err != nil {
		t.Fatalf("archive: %v", err)
	}

	in := baseExpense()
	in.PotID = "p-holiday"
	if _, err := s.CreateTransaction(ctx, in, time.Now()); !errors.Is(err, ErrPotArchived) {
		t.Errorf("err = %v, want ErrPotArchived", err)
	}
}

func TestClearingTheTargetAlsoClearsTheDate(t *testing.T) {
	_, s := setup(t)
	ctx := context.Background()

	id, err := s.CreatePot(ctx, NewPot{LedgerID: "l1", Name: "Trip", TargetCents: 100000, TargetDate: "2028-04-30"})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	zero := int64(0)
	if err := s.UpdatePot(ctx, "l1", id, PotUpdate{TargetCents: &zero}); err != nil {
		t.Fatalf("clear target: %v", err)
	}

	pots, err := s.ListPots(ctx, "l1")
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	for _, p := range pots {
		if p.ID == id && (p.TargetCents != 0 || p.TargetDate != "") {
			t.Errorf("after clearing: target %d, date %q; want both empty", p.TargetCents, p.TargetDate)
		}
	}
}

func TestPlannedAmountsCanBeSetReplacedAndRemoved(t *testing.T) {
	_, s := setup(t)
	ctx := context.Background()

	if err := s.SetPlannedAmount(ctx, planChange("p-holiday", "c-food", 30000, "")); err != nil {
		t.Fatalf("set: %v", err)
	}
	if err := s.SetPlannedAmount(ctx, planChange("p-holiday", "c-food", 40000, "")); err != nil {
		t.Fatalf("replace: %v", err)
	}

	pots, err := s.ListPots(ctx, "l1")
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	var planned []PotLine
	for _, p := range pots {
		if p.ID == "p-holiday" {
			planned = p.Planned
		}
	}
	if len(planned) != 1 || planned[0].AmountCents != 40000 {
		t.Fatalf("planned = %+v, want one line with 40000", planned)
	}

	if err := s.DeletePlannedAmount(ctx, planChange("p-holiday", "c-food", 0, "")); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if err := s.DeletePlannedAmount(ctx, planChange("p-holiday", "c-food", 0, "")); !errors.Is(err, ErrPotLineNotFound) {
		t.Errorf("second delete: err = %v, want ErrPotLineNotFound", err)
	}
	if err := s.SetPlannedAmount(ctx, planChange("p-holiday", "c-food", -1, "")); !errors.Is(err, ErrInvalidAmount) {
		t.Errorf("negative amount: err = %v, want ErrInvalidAmount", err)
	}
}

func TestUpdatePotNameMustStayUniqueAmongActivePots(t *testing.T) {
	_, s := setup(t)
	name := strings.ToUpper("holiday") // collides with the existing "Holiday" pot
	err := s.UpdatePot(context.Background(), "l1", "p-default", PotUpdate{Name: &name})
	if err == nil {
		t.Error("renaming to an existing active pot name should fail")
	}
}
