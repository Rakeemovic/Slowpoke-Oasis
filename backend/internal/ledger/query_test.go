package ledger

import (
	"context"
	"errors"
	"testing"
	"time"
)

// createExpense stores an expense of the given amount in a pot on a given day.
func createExpense(t *testing.T, s *Store, potID, bookedOn string, amount int64) string {
	t.Helper()
	in := baseExpense()
	in.PotID = potID
	in.BookedOn = bookedOn
	in.AmountCents = amount
	id, err := s.CreateTransaction(context.Background(), in, time.Now())
	if err != nil {
		t.Fatalf("create expense: %v", err)
	}
	return id
}

func TestListReturnsOnlyTheRequestedLedger(t *testing.T) {
	db, s := setup(t)
	mustExec(t, db, `INSERT INTO ledger (id, name, user_id, created_at) VALUES ('l2', 'Other', 'u1', '2026-01-01T00:00:00Z')`)
	mustExec(t, db, `INSERT INTO pot (id, ledger_id, name, kind, is_default) VALUES ('p-other', 'l2', 'Monthly', 'monthly', 1)`)
	mustExec(t, db, `INSERT INTO category (id, ledger_id, name) VALUES ('c-other', 'l2', 'Groceries')`)
	createExpense(t, s, "p-default", "2026-11-16", 100)

	in := baseExpense()
	in.LedgerID = "l2"
	in.PotID = "p-other"
	in.CategoryID = "c-other" // categories belong to a ledger, so use one from l2
	if _, err := s.CreateTransaction(context.Background(), in, time.Now()); err != nil {
		t.Fatalf("create in l2: %v", err)
	}

	got, err := s.ListTransactions(context.Background(), ListFilter{LedgerID: "l1"})
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("got %d transactions for l1, want 1", len(got))
	}
}

func TestListFiltersByPot(t *testing.T) {
	_, s := setup(t)
	createExpense(t, s, "p-default", "2026-11-01", 100)
	createExpense(t, s, "p-holiday", "2026-11-10", 200)

	got, err := s.ListTransactions(context.Background(), ListFilter{LedgerID: "l1", PotID: "p-holiday"})
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(got) != 1 || got[0].PotID != "p-holiday" {
		t.Fatalf("got %+v, want exactly one transaction in p-holiday", got)
	}
}

func TestListFiltersByDateRange(t *testing.T) {
	_, s := setup(t)
	createExpense(t, s, "p-default", "2026-11-10", 100)
	createExpense(t, s, "p-default", "2026-11-16", 100)
	createExpense(t, s, "p-default", "2026-11-20", 100)

	got, err := s.ListTransactions(context.Background(), ListFilter{
		LedgerID: "l1", From: "2026-11-11", To: "2026-11-16",
	})
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(got) != 1 || got[0].BookedOn != "2026-11-16" {
		t.Fatalf("got %+v, want only the booking from 2026-11-16 (both bounds inclusive)", got)
	}
}

func TestListIsNewestFirst(t *testing.T) {
	_, s := setup(t)
	createExpense(t, s, "p-default", "2026-11-01", 100)
	createExpense(t, s, "p-default", "2026-11-20", 100)

	got, err := s.ListTransactions(context.Background(), ListFilter{LedgerID: "l1"})
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(got) != 2 || got[0].BookedOn != "2026-11-20" {
		t.Fatalf("got %+v, want the 20th first", got)
	}
}

func TestListRequiresLedgerAndValidDates(t *testing.T) {
	_, s := setup(t)
	ctx := context.Background()

	if _, err := s.ListTransactions(ctx, ListFilter{}); !errors.Is(err, ErrLedgerRequired) {
		t.Errorf("empty ledger: err = %v, want ErrLedgerRequired", err)
	}
	if _, err := s.ListTransactions(ctx, ListFilter{LedgerID: "l1", From: "11.11.2026"}); !errors.Is(err, ErrInvalidDate) {
		t.Errorf("bad from date: err = %v, want ErrInvalidDate", err)
	}
}

func TestSoftDeletedTransactionsAreHidden(t *testing.T) {
	_, s := setup(t)
	ctx := context.Background()
	id := createExpense(t, s, "p-default", "2026-11-16", 100)

	if err := s.DeleteTransaction(ctx, "l1", id, "u1", time.Now()); err != nil {
		t.Fatalf("delete: %v", err)
	}

	got, err := s.ListTransactions(ctx, ListFilter{LedgerID: "l1"})
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("got %d transactions after delete, want 0", len(got))
	}
}

func TestDeleteFailsForWrongLedgerOrAlreadyDeleted(t *testing.T) {
	_, s := setup(t)
	ctx := context.Background()
	id := createExpense(t, s, "p-default", "2026-11-16", 100)

	if err := s.DeleteTransaction(ctx, "l-other", id, "u1", time.Now()); !errors.Is(err, ErrTransactionNotFound) {
		t.Errorf("wrong ledger: err = %v, want ErrTransactionNotFound", err)
	}

	if err := s.DeleteTransaction(ctx, "l1", id, "u1", time.Now()); err != nil {
		t.Fatalf("first delete: %v", err)
	}
	if err := s.DeleteTransaction(ctx, "l1", id, "u1", time.Now()); !errors.Is(err, ErrTransactionNotFound) {
		t.Errorf("second delete: err = %v, want ErrTransactionNotFound", err)
	}
}
