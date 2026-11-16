package ledger

import (
	"context"
	"database/sql"
	"testing"
	"time"
)

func TestPurgeRemovesOnlyTransactionsDeletedMoreThan30DaysAgo(t *testing.T) {
	db, s := setup(t)
	ctx := context.Background()
	now := time.Now()

	old := createExpense(t, s, "p-default", "2026-11-01", 100)
	recent := createExpense(t, s, "p-default", "2026-11-02", 200)
	kept := createExpense(t, s, "p-default", "2026-11-03", 300)

	if err := s.DeleteTransaction(ctx, "l1", old, "u1", now); err != nil {
		t.Fatalf("delete old: %v", err)
	}
	if err := s.DeleteTransaction(ctx, "l1", recent, "u1", now); err != nil {
		t.Fatalf("delete recent: %v", err)
	}
	// Backdate the deletions: one 31 days ago, one 10 days ago.
	mustExec(t, db, `UPDATE transactions SET deleted_at = '`+now.AddDate(0, 0, -31).UTC().Format(time.RFC3339)+`' WHERE id = '`+old+`'`)
	mustExec(t, db, `UPDATE transactions SET deleted_at = '`+now.AddDate(0, 0, -10).UTC().Format(time.RFC3339)+`' WHERE id = '`+recent+`'`)

	removed, err := s.PurgeDeletedTransactions(ctx, now)
	if err != nil {
		t.Fatalf("purge: %v", err)
	}
	if removed != 1 {
		t.Errorf("removed %d rows, want 1", removed)
	}

	var count int
	if err := db.QueryRow(`SELECT COUNT(*) FROM transactions WHERE id IN (?, ?, ?)`, old, recent, kept).Scan(&count); err != nil {
		t.Fatalf("count: %v", err)
	}
	if count != 2 {
		t.Errorf("remaining rows = %d, want 2 (the 10-day-old deletion and the active one)", count)
	}
}

func TestRemoveUserDataRemovesEverythingOfTheUser(t *testing.T) {
	db, s := setup(t)
	ctx := context.Background()
	createExpense(t, s, "p-default", "2026-11-16", 100)
	mustExec(t, db, `INSERT INTO pot_line (pot_id, ledger_id, category_id, amount_cents) VALUES ('p-default', 'l1', 'c-food', 5000)`)

	err := s.inTx(ctx, func(tx *sql.Tx) error { return RemoveUserData(ctx, tx, "u1") })
	if err != nil {
		t.Fatalf("remove user data: %v", err)
	}

	for _, table := range []string{"ledger", "pot", "category", "transactions", "pot_line"} {
		var n int
		if err := db.QueryRow(`SELECT COUNT(*) FROM ` + table).Scan(&n); err != nil {
			t.Fatalf("count %s: %v", table, err)
		}
		if n != 0 {
			t.Errorf("%s: %d rows left, want 0", table, n)
		}
	}
}
