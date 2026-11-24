package ledger

import (
	"context"
	"errors"
	"testing"
	"time"
)

// makeSeparate turns the holiday pot into a real sub-account.
func makeSeparate(t *testing.T, s *Store, separate bool) {
	t.Helper()
	if err := s.UpdatePot(context.Background(), "l1", "p-holiday", PotUpdate{SeparateAccount: &separate}); err != nil {
		t.Fatalf("set separate account: %v", err)
	}
}

func balanceOn(t *testing.T, s *Store, day string) Balance {
	t.Helper()
	b, err := s.LedgerBalance(context.Background(), "l1", day, time.Now())
	if err != nil {
		t.Fatalf("balance on %s: %v", day, err)
	}
	return b
}

func TestEstimatedBalanceLeavesSeparateAccountsOut(t *testing.T) {
	_, s := setup(t)
	salary := baseExpense()
	salary.Type, salary.CategoryID, salary.AmountCents, salary.BookedOn = TypeIncome, "c-salary", 100000, "2026-11-01"
	if _, err := s.CreateTransaction(context.Background(), salary, time.Now()); err != nil {
		t.Fatal(err)
	}
	moveToPot(t, s, "p-default", "p-holiday", 30000, "2026-11-05")
	createExpense(t, s, "p-holiday", "2026-11-06", 2000)

	// As a thought-up pot, the money still counts as being on the account.
	if b := balanceOn(t, s, "2026-11-30"); b.EstimatedCents != 98000 || b.SeparateCents != 0 || len(b.SeparateAccounts) != 0 {
		t.Errorf("before: estimated %d, separate %d (%d accounts); want 98000, 0 (0)", b.EstimatedCents, b.SeparateCents, len(b.SeparateAccounts))
	}

	makeSeparate(t, s, true)
	b := balanceOn(t, s, "2026-11-30")
	if b.TotalCents != 98000 || b.AvailableCents != 70000 || b.SeparateCents != 28000 || b.EstimatedCents != 70000 {
		t.Errorf("after: total %d, available %d, separate %d, estimated %d; want 98000, 70000, 28000, 70000",
			b.TotalCents, b.AvailableCents, b.SeparateCents, b.EstimatedCents)
	}
	want := SeparateAccount{PotID: "p-holiday", Name: "Holiday", BalanceCents: 28000}
	if len(b.SeparateAccounts) != 1 || b.SeparateAccounts[0] != want {
		t.Errorf("separate accounts = %+v, want [%+v]", b.SeparateAccounts, want)
	}

	// The balance follows the day asked for, like the other amounts.
	if b := balanceOn(t, s, "2026-11-05"); b.SeparateCents != 30000 || b.EstimatedCents != 70000 {
		t.Errorf("on 5 Nov: separate %d, estimated %d; want 30000, 70000", b.SeparateCents, b.EstimatedCents)
	}

	// Back to a thought-up pot: the estimate counts it again at once.
	makeSeparate(t, s, false)
	if b := balanceOn(t, s, "2026-11-30"); b.EstimatedCents != 98000 {
		t.Errorf("switched back: estimated %d, want 98000", b.EstimatedCents)
	}
}

func TestSeparateAccountKeepsItsMoneyWhenArchived(t *testing.T) {
	_, s := setup(t)
	salary := baseExpense()
	salary.Type, salary.CategoryID, salary.AmountCents, salary.BookedOn = TypeIncome, "c-salary", 50000, "2026-11-01"
	if _, err := s.CreateTransaction(context.Background(), salary, time.Now()); err != nil {
		t.Fatal(err)
	}
	moveToPot(t, s, "p-default", "p-holiday", 10000, "2026-11-05")
	makeSeparate(t, s, true)
	archived := true
	if err := s.UpdatePot(context.Background(), "l1", "p-holiday", PotUpdate{Archived: &archived}); err != nil {
		t.Fatal(err)
	}
	// Archiving the pot does not move the money at the bank.
	if b := balanceOn(t, s, "2026-11-30"); b.SeparateCents != 10000 || b.EstimatedCents != 40000 {
		t.Errorf("separate %d, estimated %d; want 10000, 40000", b.SeparateCents, b.EstimatedCents)
	}
}

func TestSeparateAccountRules(t *testing.T) {
	_, s := setup(t)
	ctx := context.Background()
	yes := true
	reserve := loadPayment(t, s, mustCreateRecurring(t, s, insurance())).ReservePotID
	for name, pot := range map[string]string{"main pot": "p-default", "reserve": reserve} {
		if err := s.UpdatePot(ctx, "l1", pot, PotUpdate{SeparateAccount: &yes}); !errors.Is(err, ErrSeparateNotAllowed) {
			t.Errorf("%s: err = %v, want ErrSeparateNotAllowed", name, err)
		}
	}

	id, err := s.CreatePot(ctx, NewPot{LedgerID: "l1", Name: "Savings", SeparateAccount: true})
	if err != nil {
		t.Fatalf("create separate pot: %v", err)
	}
	pots, err := s.ListPots(ctx, "l1")
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range pots {
		if want := p.ID == id; p.IsSeparateAccount != want {
			t.Errorf("pot %s: IsSeparateAccount = %v, want %v", p.Name, p.IsSeparateAccount, want)
		}
	}
}
