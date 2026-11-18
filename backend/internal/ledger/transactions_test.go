package ledger

import (
	"context"
	"database/sql"
	"errors"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Rakeemovic/slowpoke-oasis/backend/internal/store"
)

// setup opens an in-memory database with one user, one personal ledger,
// a default pot, a second pot and one category.
func setup(t *testing.T) (*sql.DB, *Store) {
	t.Helper()
	db, err := store.Open(":memory:")
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { db.Close() })

	mustExec(t, db, `INSERT INTO user (id, name, created_at) VALUES ('u1', 'Alice', '2026-01-01T00:00:00Z')`)
	mustExec(t, db, `INSERT INTO ledger (id, name, user_id, created_at) VALUES ('l1', 'Personal', 'u1', '2026-01-01T00:00:00Z')`)
	mustExec(t, db, `INSERT INTO pot (id, ledger_id, name, kind, is_default) VALUES ('p-default', 'l1', 'Monthly', 'monthly', 1)`)
	mustExec(t, db, `INSERT INTO pot (id, ledger_id, name, kind) VALUES ('p-holiday', 'l1', 'Holiday', 'one_time')`)
	mustExec(t, db, `INSERT INTO category (id, ledger_id, name) VALUES ('c-food', 'l1', 'Groceries')`)
	mustExec(t, db, `INSERT INTO category (id, ledger_id, name, is_income) VALUES ('c-salary', 'l1', 'Salary', 1)`)

	return db, New(db)
}

func mustExec(t *testing.T, db *sql.DB, query string) {
	t.Helper()
	if _, err := db.Exec(query); err != nil {
		t.Fatalf("exec %q: %v", query, err)
	}
}

func baseExpense() NewTransaction {
	return NewTransaction{
		LedgerID:    "l1",
		UserID:      "u1",
		Type:        TypeExpense,
		CategoryID:  "c-food",
		AmountCents: 1250,
		BookedOn:    "2026-11-16",
	}
}

func TestExpenseIsStoredNegativeInDefaultPot(t *testing.T) {
	db, s := setup(t)

	id, err := s.CreateTransaction(context.Background(), baseExpense(), time.Now())
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	var amount int64
	var potID string
	if err := db.QueryRow(`SELECT amount_cents, pot_id FROM transactions WHERE id = ?`, id).Scan(&amount, &potID); err != nil {
		t.Fatalf("read back: %v", err)
	}
	if amount != -1250 {
		t.Errorf("amount = %d, want -1250", amount)
	}
	if potID != "p-default" {
		t.Errorf("pot = %q, want default pot p-default", potID)
	}
}

func TestIncomeIsStoredPositive(t *testing.T) {
	db, s := setup(t)

	in := baseExpense()
	in.Type = TypeIncome
	in.CategoryID = "c-salary"
	id, err := s.CreateTransaction(context.Background(), in, time.Now())
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	var amount int64
	if err := db.QueryRow(`SELECT amount_cents FROM transactions WHERE id = ?`, id).Scan(&amount); err != nil {
		t.Fatalf("read back: %v", err)
	}
	if amount != 1250 {
		t.Errorf("amount = %d, want 1250", amount)
	}
}

func TestTransferMovesBetweenPots(t *testing.T) {
	db, s := setup(t)

	in := NewTransaction{
		LedgerID:         "l1",
		UserID:           "u1",
		Type:             TypeTransfer,
		PotID:            "p-default",
		CounterpartPotID: "p-holiday",
		AmountCents:      20000,
		BookedOn:         "2026-11-16",
	}
	id, err := s.CreateTransaction(context.Background(), in, time.Now())
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	var amount int64
	var category sql.NullString
	var counterpart sql.NullString
	err = db.QueryRow(
		`SELECT amount_cents, category_id, counterpart_pot_id FROM transactions WHERE id = ?`, id,
	).Scan(&amount, &category, &counterpart)
	if err != nil {
		t.Fatalf("read back: %v", err)
	}
	if amount != 20000 {
		t.Errorf("amount = %d, want 20000", amount)
	}
	if category.Valid {
		t.Errorf("category = %q, want NULL", category.String)
	}
	if counterpart.String != "p-holiday" {
		t.Errorf("counterpart = %q, want p-holiday", counterpart.String)
	}
}

func TestValidationRejectsInvalidInput(t *testing.T) {
	_, s := setup(t)

	cases := []struct {
		name    string
		mutate  func(in *NewTransaction)
		wantErr error
	}{
		{"zero amount", func(in *NewTransaction) { in.AmountCents = 0 }, ErrAmountNotPositive},
		{"negative input amount", func(in *NewTransaction) { in.AmountCents = -5 }, ErrAmountNotPositive},
		{"unknown type", func(in *NewTransaction) { in.Type = "refund" }, ErrInvalidType},
		{"expense without category", func(in *NewTransaction) { in.CategoryID = "" }, ErrCategoryRequired},
		{"income with counterpart", func(in *NewTransaction) {
			in.Type = TypeIncome
			in.CounterpartPotID = "p-holiday"
		}, ErrCounterpartForbidden},
		{"transfer with category", func(in *NewTransaction) {
			in.Type = TypeTransfer
			in.CounterpartPotID = "p-holiday"
		}, ErrCategoryForbidden},
		{"transfer without counterpart", func(in *NewTransaction) {
			in.Type = TypeTransfer
			in.CategoryID = ""
		}, ErrCounterpartRequired},
		{"transfer to same pot", func(in *NewTransaction) {
			in.Type = TypeTransfer
			in.CategoryID = ""
			in.PotID = "p-default"
			in.CounterpartPotID = "p-default"
		}, ErrSamePot},
		{"date with dots", func(in *NewTransaction) { in.BookedOn = "16.11.2026" }, ErrInvalidDate},
		{"note too long", func(in *NewTransaction) { in.Note = strings.Repeat("x", 501) }, ErrNoteTooLong},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			in := baseExpense()
			tc.mutate(&in)
			_, err := s.CreateTransaction(context.Background(), in, time.Now())
			if !errors.Is(err, tc.wantErr) {
				t.Errorf("err = %v, want %v", err, tc.wantErr)
			}
		})
	}
}

func TestDatabaseRejectsTransferAcrossLedgers(t *testing.T) {
	db, s := setup(t)
	mustExec(t, db, `INSERT INTO ledger (id, name, user_id, created_at) VALUES ('l2', 'Other', 'u1', '2026-01-01T00:00:00Z')`)
	mustExec(t, db, `INSERT INTO pot (id, ledger_id, name, kind, is_default) VALUES ('p-other', 'l2', 'Monthly', 'monthly', 1)`)

	in := NewTransaction{
		LedgerID:         "l1",
		UserID:           "u1",
		Type:             TypeTransfer,
		PotID:            "p-default",
		CounterpartPotID: "p-other", // belongs to ledger l2
		AmountCents:      100,
		BookedOn:         "2026-11-16",
	}
	if _, err := s.CreateTransaction(context.Background(), in, time.Now()); err == nil {
		t.Fatal("expected foreign key error for a pot from another ledger, got nil")
	}
}

func TestDefaultPotCannotBeDeleted(t *testing.T) {
	db, _ := setup(t)
	if _, err := db.Exec(`DELETE FROM pot WHERE id = 'p-default'`); err == nil {
		t.Fatal("expected the default pot to be protected from deletion")
	}
}

func TestMigrationsAreIdempotent(t *testing.T) {
	path := filepath.Join(t.TempDir(), "test.db")

	first, err := store.Open(path)
	if err != nil {
		t.Fatalf("first open: %v", err)
	}
	first.Close()

	second, err := store.Open(path)
	if err != nil {
		t.Fatalf("second open should not re-apply migrations: %v", err)
	}
	second.Close()
}
