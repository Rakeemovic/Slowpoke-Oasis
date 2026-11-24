package ledger

import (
	"context"
	"errors"
	"testing"
	"time"
)

// reconcile compares an account (empty potID = bank account) on 2026-11-20.
func reconcile(t *testing.T, s *Store, potID string, actual int64, correct bool) ReconcileResult {
	t.Helper()
	got, err := s.Reconcile(context.Background(), ReconcileInput{
		LedgerID: "l1", UserID: "u1", PotID: potID, ActualCents: actual, Today: "2026-11-20", Correct: correct,
	}, time.Now())
	if err != nil {
		t.Fatalf("reconcile %q: %v", potID, err)
	}
	return got
}

func estimated(t *testing.T, s *Store) Balance {
	t.Helper()
	b, err := s.LedgerBalance(context.Background(), "l1", "2026-11-20", time.Now())
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func TestReconcileComparesWithTheEstimatedBalanceAndCorrects(t *testing.T) {
	_, s := setup(t)
	createExpense(t, s, "p-default", "2026-11-10", 2000)

	// Only compared: nothing is booked, but the reconciliation is kept.
	got := reconcile(t, s, "", -2500, false)
	if got.PotID != "p-default" || got.ExpectedCents != -2000 || got.DifferenceCents != -500 || got.CorrectionID != "" {
		t.Errorf("compare = %+v, want expected -2000, difference -500, no correction", got)
	}
	if b := estimated(t, s); b.EstimatedCents != -2000 || b.Reconciled == nil || b.Reconciled.ActualCents != -2500 {
		t.Errorf("after compare: estimated %d, reconciled %+v", b.EstimatedCents, b.Reconciled)
	}

	// Corrected: the difference is booked into the main pot, without category.
	got = reconcile(t, s, "", 1000, true)
	if got.DifferenceCents != 3000 || got.CorrectionID == "" {
		t.Fatalf("correct = %+v, want difference 3000 and a correction", got)
	}
	b := estimated(t, s)
	if b.EstimatedCents != 1000 || b.AvailableCents != 1000 {
		t.Errorf("after correction: estimated %d, available %d, want 1000", b.EstimatedCents, b.AvailableCents)
	}
	if b.Reconciled == nil || *b.Reconciled != (Reconciliation{ReconciledOn: "2026-11-20", ActualCents: 1000}) {
		t.Errorf("reconciled = %+v, want only the last one", b.Reconciled)
	}
	tx, err := s.GetTransaction(context.Background(), "l1", got.CorrectionID)
	if err != nil {
		t.Fatal(err)
	}
	if tx.Type != TypeCorrection || tx.CategoryID != "" || tx.AmountCents != 3000 || tx.BookedOn != "2026-11-20" {
		t.Errorf("correction = %+v", tx)
	}

	// No difference, no booking.
	if got := reconcile(t, s, "", 1000, true); got.DifferenceCents != 0 || got.CorrectionID != "" {
		t.Errorf("no difference = %+v, want no correction", got)
	}
}

func TestReconcileASeparateAccount(t *testing.T) {
	_, s := setup(t)
	ctx := context.Background()
	in := baseExpense()
	in.Type, in.CategoryID, in.PotID, in.AmountCents = TypeIncome, "c-salary", "p-holiday", 5000
	if _, err := s.CreateTransaction(ctx, in, time.Now()); err != nil {
		t.Fatal(err)
	}

	// Not yet a separate account: it cannot be reconciled on its own.
	_, err := s.Reconcile(ctx, ReconcileInput{LedgerID: "l1", UserID: "u1", PotID: "p-holiday", Today: "2026-11-20"}, time.Now())
	if !errors.Is(err, ErrNotAnAccount) {
		t.Errorf("plain pot: err = %v, want ErrNotAnAccount", err)
	}
	_, err = s.Reconcile(ctx, ReconcileInput{LedgerID: "l1", UserID: "u1", PotID: "nope", Today: "2026-11-20"}, time.Now())
	if !errors.Is(err, ErrPotNotFound) {
		t.Errorf("unknown pot: err = %v, want ErrPotNotFound", err)
	}

	separate := true
	if err := s.UpdatePot(ctx, "l1", "p-holiday", PotUpdate{SeparateAccount: &separate}); err != nil {
		t.Fatal(err)
	}
	got := reconcile(t, s, "p-holiday", 4200, true)
	if got.ExpectedCents != 5000 || got.DifferenceCents != -800 || got.CorrectionID == "" {
		t.Errorf("separate = %+v, want expected 5000, difference -800", got)
	}
	b := estimated(t, s)
	if len(b.SeparateAccounts) != 1 || b.SeparateAccounts[0].BalanceCents != 4200 || b.SeparateAccounts[0].Reconciled == nil {
		t.Errorf("separate accounts = %+v", b.SeparateAccounts)
	}
	// The bank account stays as it was and has not been reconciled.
	if b.EstimatedCents != 0 || b.Reconciled != nil {
		t.Errorf("bank account: estimated %d, reconciled %+v", b.EstimatedCents, b.Reconciled)
	}
}

func TestACorrectionCanOnlyBeDeletedAndRestored(t *testing.T) {
	_, s := setup(t)
	ctx := context.Background()
	id := reconcile(t, s, "", 700, true).CorrectionID

	// It changes the balance, but no category report (rule X22).
	if totals, err := s.CategoryTotals(ctx, "l1", "", ""); err != nil || len(totals) != 0 {
		t.Errorf("category totals = %+v, %v, want none", totals, err)
	}

	amount := int64(1)
	if err := s.UpdateTransaction(ctx, "l1", id, TransactionPatch{AmountCents: &amount}, time.Now()); !errors.Is(err, ErrCorrectionTransaction) {
		t.Errorf("update: err = %v, want ErrCorrectionTransaction", err)
	}
	perms, err := s.Permissions(ctx, "u1", []Transaction{{ID: id}})
	if err != nil {
		t.Fatal(err)
	}
	if p := perms[id]; p.CanEdit || !p.CanDelete {
		t.Errorf("permissions = %+v, want delete only", p)
	}
	if err := s.DeleteTransaction(ctx, "l1", id, "u1", time.Now()); err != nil {
		t.Fatal(err)
	}
	if b := estimated(t, s); b.EstimatedCents != 0 {
		t.Errorf("after delete: estimated %d, want 0", b.EstimatedCents)
	}
	if err := s.RestoreTransaction(ctx, "l1", id, time.Now()); err != nil {
		t.Fatal(err)
	}
	if b := estimated(t, s); b.EstimatedCents != 700 {
		t.Errorf("after restore: estimated %d, want 700", b.EstimatedCents)
	}
}

func TestACorrectionCannotBeBookedDirectly(t *testing.T) {
	in := baseExpense()
	in.Type, in.CategoryID = TypeCorrection, ""
	if err := in.Validate(); !errors.Is(err, ErrInvalidType) {
		t.Errorf("err = %v, want ErrInvalidType", err)
	}
}

func TestReconcileRejectsInvalidInput(t *testing.T) {
	_, s := setup(t)
	ctx := context.Background()
	cases := map[string]struct {
		in   ReconcileInput
		want error
	}{
		"date":    {ReconcileInput{LedgerID: "l1", Today: "tomorrow"}, ErrInvalidDate},
		"too big": {ReconcileInput{LedgerID: "l1", Today: "2026-11-20", ActualCents: maxBalanceCents + 1}, ErrInvalidBalance},
	}
	for name, c := range cases {
		if _, err := s.Reconcile(ctx, c.in, time.Now()); !errors.Is(err, c.want) {
			t.Errorf("%s: err = %v, want %v", name, err, c.want)
		}
	}
}
