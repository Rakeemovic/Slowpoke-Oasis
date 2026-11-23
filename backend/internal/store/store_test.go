package store

import (
	"database/sql"
	"errors"
	"io/fs"
	"path/filepath"
	"sort"
	"testing"

	"github.com/Rakeemovic/slowpoke-oasis/backend/migrations"
)

func migrationNames(t *testing.T) []string {
	t.Helper()
	names, err := fs.Glob(migrations.FS, "*.sql")
	if err != nil {
		t.Fatalf("list migrations: %v", err)
	}
	sort.Strings(names)
	return names
}

func TestForeignKeysAreEnforced(t *testing.T) {
	db, err := Open(":memory:")
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer db.Close()

	_, err = db.Exec(`INSERT INTO ledger (id, name, user_id, created_at) VALUES ('l1', 'Home', 'nobody', '2026-01-01T00:00:00Z')`)
	if !IsConstraintViolation(err) {
		t.Fatalf("ledger for an unknown user: got %v, want a constraint violation", err)
	}
	if IsUniqueViolation(err) {
		t.Errorf("a foreign key violation must not count as a unique violation")
	}
}

func TestUniqueViolationIsRecognized(t *testing.T) {
	db, err := Open(":memory:")
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer db.Close()

	insert := `INSERT INTO user (id, name, created_at) VALUES ('u1', 'Alice', '2026-01-01T00:00:00Z')`
	if _, err := db.Exec(insert); err != nil {
		t.Fatalf("first insert: %v", err)
	}
	_, err = db.Exec(insert)
	if !IsUniqueViolation(err) || !IsConstraintViolation(err) {
		t.Errorf("duplicate primary key: got %v, want a unique violation", err)
	}
	if IsUniqueViolation(errors.New("UNIQUE constraint failed")) {
		t.Errorf("an error that only looks like a unique violation must not match")
	}
}

// openSchemaBefore opens a fresh database file with all migrations before
// upgrade applied, so a test can seed old data and then run Open on path.
func openSchemaBefore(t *testing.T, upgrade string) (old *sql.DB, path string) {
	t.Helper()
	names := migrationNames(t)
	cut := sort.SearchStrings(names, upgrade)
	if cut == len(names) || names[cut] != upgrade {
		t.Fatalf("migration %s not found", upgrade)
	}
	path = filepath.Join(t.TempDir(), "test.db")
	old, err := open(path, names[:cut])
	if err != nil {
		t.Fatalf("open old schema: %v", err)
	}
	return old, path
}

// Accounts from before migration 0010 get their oldest personal ledger as the
// primary one; later personal ledgers and household ledgers stay unmarked.
func TestPrimaryLedgerMigrationMarksOldestPersonalLedger(t *testing.T) {
	old, path := openSchemaBefore(t, "0010_primary_personal_ledger.sql")
	seed := []string{
		`INSERT INTO user (id, name, created_at) VALUES ('u1', 'Alice', '2026-01-01T00:00:00Z')`,
		`INSERT INTO household (id, name, created_at) VALUES ('h1', 'Home', '2026-01-01T00:00:00Z')`,
		`INSERT INTO ledger (id, name, user_id, created_at) VALUES ('later', 'Savings', 'u1', '2026-02-01T00:00:00Z')`,
		`INSERT INTO ledger (id, name, user_id, created_at) VALUES ('first', 'Personal', 'u1', '2026-01-01T00:00:00Z')`,
		`INSERT INTO ledger (id, name, household_id, created_at) VALUES ('shared', 'Home', 'h1', '2025-01-01T00:00:00Z')`,
	}
	for _, q := range seed {
		if _, err := old.Exec(q); err != nil {
			t.Fatalf("seed %q: %v", q, err)
		}
	}
	old.Close()

	db, err := Open(path)
	if err != nil {
		t.Fatalf("upgrade: %v", err)
	}
	defer db.Close()

	primary, err := db.Query(`SELECT id FROM ledger WHERE is_primary = 1`)
	if err != nil {
		t.Fatalf("query primary ledgers: %v", err)
	}
	defer primary.Close()
	var ids []string
	for primary.Next() {
		var id string
		if err := primary.Scan(&id); err != nil {
			t.Fatal(err)
		}
		ids = append(ids, id)
	}
	if len(ids) != 1 || ids[0] != "first" {
		t.Errorf("primary ledgers = %v, want [first]", ids)
	}
}

// Ledgers from before migration 0012 without any category get the defaults;
// ledgers that already have categories, even only archived ones, keep them.
func TestDefaultCategoryMigrationFillsOnlyEmptyLedgers(t *testing.T) {
	old, path := openSchemaBefore(t, "0012_default_categories.sql")
	seed := []string{
		`INSERT INTO user (id, name, created_at) VALUES ('u1', 'Alice', '2026-01-01T00:00:00Z')`,
		`INSERT INTO ledger (id, name, user_id, created_at) VALUES ('empty', 'Personal', 'u1', '2026-01-01T00:00:00Z')`,
		`INSERT INTO ledger (id, name, user_id, created_at) VALUES ('own', 'Savings', 'u1', '2026-01-01T00:00:00Z')`,
		`INSERT INTO category (id, ledger_id, name, archived) VALUES ('c1', 'own', 'Old', 1)`,
	}
	for _, q := range seed {
		if _, err := old.Exec(q); err != nil {
			t.Fatalf("seed %q: %v", q, err)
		}
	}
	old.Close()

	db, err := Open(path)
	if err != nil {
		t.Fatalf("upgrade: %v", err)
	}
	defer db.Close()

	counts := map[string]int{"empty": 14, "own": 1}
	for ledgerID, want := range counts {
		var got int
		if err := db.QueryRow(`SELECT count(*) FROM category WHERE ledger_id = ?`, ledgerID).Scan(&got); err != nil {
			t.Fatal(err)
		}
		if got != want {
			t.Errorf("ledger %s: %d categories, want %d", ledgerID, got, want)
		}
	}
	var badIDs int
	if err := db.QueryRow(`SELECT count(*) FROM category WHERE ledger_id = 'empty' AND (length(id) != 32 OR id != lower(id))`).Scan(&badIDs); err != nil {
		t.Fatal(err)
	}
	if badIDs != 0 {
		t.Errorf("%d category IDs are not 32 lower-case hex characters", badIDs)
	}
}

// Transfers between ledgers that were booked before migration 0007 must
// survive the table rebuild with their links, and afterwards a ledger with
// one leg of such a transfer can be deleted.
func TestOrphanedLegMigrationKeepsExistingLinks(t *testing.T) {
	old, path := openSchemaBefore(t, "0007_orphaned_transfer_legs.sql")
	seed := []string{
		`INSERT INTO user (id, name, created_at) VALUES ('u1', 'Alice', '2026-01-01T00:00:00Z')`,
		`INSERT INTO ledger (id, name, user_id, created_at) VALUES ('l1', 'Home', 'u1', '2026-01-01T00:00:00Z')`,
		`INSERT INTO ledger (id, name, user_id, created_at) VALUES ('l2', 'Savings', 'u1', '2026-01-01T00:00:00Z')`,
		`INSERT INTO pot (id, ledger_id, name, kind, is_default) VALUES ('p1', 'l1', 'Monthly', 'monthly', 1)`,
		`INSERT INTO pot (id, ledger_id, name, kind) VALUES ('p2', 'l2', 'Goal', 'one_time')`,
	}
	for _, q := range seed {
		if _, err := old.Exec(q); err != nil {
			t.Fatalf("seed %q: %v", q, err)
		}
	}
	// The legs point at each other, so both go in before the deferred check.
	tx, err := old.Begin()
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	const leg = `
		INSERT INTO transactions (id, ledger_id, user_id, type, pot_id, amount_cents, booked_on,
		                          created_at, updated_at, linked_transaction_id)
		VALUES (?, ?, 'u1', ?, ?, ?, '2026-11-16', '2026-11-16T00:00:00Z', '2026-11-16T00:00:00Z', ?)`
	if _, err := tx.Exec(leg, "out", "l1", "transfer_out", "p1", -5000, "in"); err != nil {
		t.Fatalf("insert out leg: %v", err)
	}
	if _, err := tx.Exec(leg, "in", "l2", "transfer_in", "p2", 5000, "out"); err != nil {
		t.Fatalf("insert in leg: %v", err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatalf("commit legs: %v", err)
	}
	old.Close()

	db, err := Open(path)
	if err != nil {
		t.Fatalf("upgrade: %v", err)
	}
	defer db.Close()

	var link string
	if err := db.QueryRow(`SELECT linked_transaction_id FROM transactions WHERE id = 'in'`).Scan(&link); err != nil {
		t.Fatalf("read link after upgrade: %v", err)
	}
	if link != "out" {
		t.Errorf("link after upgrade = %q, want %q", link, "out")
	}

	// Removing the source side leaves the other leg in place, without a link.
	if _, err := db.Exec(`DELETE FROM transactions WHERE ledger_id = 'l1'`); err != nil {
		t.Fatalf("delete one leg: %v", err)
	}
	var orphaned bool
	if err := db.QueryRow(`SELECT linked_transaction_id IS NULL FROM transactions WHERE id = 'in'`).Scan(&orphaned); err != nil {
		t.Fatalf("read remaining leg: %v", err)
	}
	if !orphaned {
		t.Errorf("the remaining leg should have lost its link")
	}
}

// Migration 0017 renames only default pots still called "Monthly", and only
// where no active pot is called "Main" yet (rule T1).
func TestMainPotMigrationRenamesOnlyUntouchedDefaultPots(t *testing.T) {
	old, path := openSchemaBefore(t, "0017_main_pot.sql")
	seed := []string{
		`INSERT INTO user (id, name, created_at) VALUES ('u1', 'Alice', '2026-01-01T00:00:00Z')`,
		`INSERT INTO ledger (id, name, user_id, created_at) VALUES ('l1', 'Personal', 'u1', '2026-01-01T00:00:00Z')`,
		`INSERT INTO ledger (id, name, user_id, created_at) VALUES ('l2', 'Home', 'u1', '2026-01-01T00:00:00Z')`,
		`INSERT INTO ledger (id, name, user_id, created_at) VALUES ('l3', 'Savings', 'u1', '2026-01-01T00:00:00Z')`,
		`INSERT INTO pot (id, ledger_id, name, kind, is_default) VALUES ('standard', 'l1', 'Monthly', 'monthly', 1)`,
		`INSERT INTO pot (id, ledger_id, name, kind) VALUES ('other', 'l1', 'Monthly bills', 'one_time')`,
		`INSERT INTO pot (id, ledger_id, name, kind, is_default) VALUES ('taken', 'l2', 'Monthly', 'monthly', 1)`,
		`INSERT INTO pot (id, ledger_id, name, kind) VALUES ('main', 'l2', 'MAIN', 'one_time')`,
		`INSERT INTO pot (id, ledger_id, name, kind, is_default) VALUES ('chosen', 'l3', 'Daily', 'monthly', 1)`,
	}
	for _, q := range seed {
		if _, err := old.Exec(q); err != nil {
			t.Fatalf("seed %q: %v", q, err)
		}
	}
	old.Close()

	db, err := Open(path)
	if err != nil {
		t.Fatalf("upgrade: %v", err)
	}
	defer db.Close()

	want := map[string]string{
		"standard": "Main",
		"other":    "Monthly bills",
		"taken":    "Monthly", // "MAIN" is already taken in this ledger
		"main":     "MAIN",
		"chosen":   "Daily",
	}
	for id, name := range want {
		var got string
		if err := db.QueryRow(`SELECT name FROM pot WHERE id = ?`, id).Scan(&got); err != nil {
			t.Fatal(err)
		}
		if got != name {
			t.Errorf("pot %s: name %q, want %q", id, got, name)
		}
	}
}
