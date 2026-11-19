package ledger

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestLedgerBalanceSplitsAvailableAndTotalUpToADay(t *testing.T) {
	_, s := setup(t)
	ctx := context.Background()

	// 500 EUR salary into the default pot.
	in := baseExpense()
	in.Type = TypeIncome
	in.CategoryID = "c-salary"
	in.AmountCents = 50000
	in.BookedOn = "2026-11-01"
	if _, err := s.CreateTransaction(ctx, in, time.Now()); err != nil {
		t.Fatalf("income: %v", err)
	}
	// 200 EUR set aside in the holiday pot, 30 EUR of it spent.
	transfer := NewTransaction{
		LedgerID: "l1", UserID: "u1", Type: TypeTransfer,
		PotID: "p-default", CounterpartPotID: "p-holiday",
		AmountCents: 20000, BookedOn: "2026-11-02",
	}
	if _, err := s.CreateTransaction(ctx, transfer, time.Now()); err != nil {
		t.Fatalf("transfer: %v", err)
	}
	createExpense(t, s, "p-holiday", "2026-11-03", 3000)
	// 10 EUR groceries, of which a deleted 5 EUR booking no longer counts.
	createExpense(t, s, "p-default", "2026-11-04", 1000)
	deleted := createExpense(t, s, "p-default", "2026-11-04", 500)
	if err := s.DeleteTransaction(ctx, "l1", deleted, "u1", time.Now()); err != nil {
		t.Fatalf("delete: %v", err)
	}
	// Rent planned for next month.
	createExpense(t, s, "p-default", "2026-12-01", 40000)

	tests := []struct {
		asOf             string
		available, total int64
	}{
		{"2026-10-31", 0, 0},
		{"2026-11-01", 50000, 50000},
		{"2026-11-02", 30000, 50000},        // set aside: no longer available, still in total
		{"2026-11-30", 29000, 46000},        // holiday expense only lowers the total
		{"2026-12-01", 29000 - 40000, 6000}, // planned rent counts from its day on
	}
	for _, tt := range tests {
		got, err := s.LedgerBalance(ctx, "l1", tt.asOf, time.Now())
		if err != nil {
			t.Fatalf("balance %s: %v", tt.asOf, err)
		}
		if got.AvailableCents != tt.available || got.TotalCents != tt.total {
			t.Errorf("%s: available %d, total %d; want %d, %d",
				tt.asOf, got.AvailableCents, got.TotalCents, tt.available, tt.total)
		}
	}
}

func TestLedgerBalanceDefaultsToToday(t *testing.T) {
	_, s := setup(t)
	createExpense(t, s, "p-default", "2026-11-19", 100)
	createExpense(t, s, "p-default", "2026-11-20", 100)

	now := time.Date(2026, 11, 19, 12, 0, 0, 0, time.UTC)
	got, err := s.LedgerBalance(context.Background(), "l1", "", now)
	if err != nil {
		t.Fatal(err)
	}
	if got.AsOf != "2026-11-19" || got.TotalCents != -100 {
		t.Errorf("got %+v, want as of 2026-11-19 with total -100", got)
	}
}

func TestLedgerBalanceRejectsInvalidDate(t *testing.T) {
	_, s := setup(t)
	if _, err := s.LedgerBalance(context.Background(), "l1", "19.11.2026", time.Now()); !errors.Is(err, ErrInvalidDate) {
		t.Errorf("err = %v, want ErrInvalidDate", err)
	}
}
