package ledger

import (
	"context"
	"testing"
	"time"
)

func TestDeletedByIsOnlyAllowedForDeletedTransactions(t *testing.T) {
	db, s := setup(t)
	id := createExpense(t, s, "p-default", "2026-11-16", 100)

	_, err := db.Exec(`UPDATE transactions SET deleted_by = 'u1' WHERE id = ?`, id)
	if err == nil {
		t.Fatal("deleted_by without deleted_at was accepted, want a CHECK violation")
	}
}

func TestReleaseHouseholdTransactionsKeepsThemUnderPlaceholder(t *testing.T) {
	db, s := setup(t)
	ctx := context.Background()

	mustExec(t, db, `INSERT INTO user (id, name, created_at) VALUES ('bob', 'Bob', '2026-01-01T00:00:00Z')`)
	mustExec(t, db, `INSERT INTO household (id, name, created_at) VALUES ('h1', 'Flat', '2026-01-01T00:00:00Z')`)
	mustExec(t, db, `INSERT INTO household_member (household_id, user_id) VALUES ('h1', 'bob')`)
	mustExec(t, db, `INSERT INTO ledger (id, name, household_id, created_at) VALUES ('lh', 'Shared', 'h1', '2026-01-01T00:00:00Z')`)
	mustExec(t, db, `INSERT INTO pot (id, ledger_id, name, kind, is_default) VALUES ('ph', 'lh', 'Monthly', 'monthly', 1)`)
	mustExec(t, db, `INSERT INTO category (id, ledger_id, name) VALUES ('ch', 'lh', 'Rent')`)

	id, err := s.CreateTransaction(ctx, NewTransaction{
		LedgerID: "lh", UserID: "bob", Type: TypeExpense,
		CategoryID: "ch", AmountCents: 50000, BookedOn: "2026-11-16",
	}, time.Now())
	if err != nil {
		t.Fatalf("create household expense: %v", err)
	}

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := ReleaseHouseholdTransactionsTx(ctx, tx, "bob"); err != nil {
		tx.Rollback()
		t.Fatalf("release: %v", err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatal(err)
	}

	var owner string
	var amount int64
	if err := db.QueryRow(`SELECT user_id, amount_cents FROM transactions WHERE id = ?`, id).Scan(&owner, &amount); err != nil {
		t.Fatal(err)
	}
	if owner != FormerMemberID {
		t.Errorf("user_id = %q, want %q", owner, FormerMemberID)
	}
	if amount != -50000 {
		t.Errorf("amount_cents = %d, want -50000 (unchanged)", amount)
	}
}
