package ledger

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

func TestCreatePersonalLedgerAddsDefaultPot(t *testing.T) {
	db, s := setup(t)
	ctx := context.Background()
	mustExec(t, db, `INSERT INTO user (id, name, created_at) VALUES ('bob', 'Bob', '2026-01-01T00:00:00Z')`)

	ledgerID, err := s.CreatePersonalLedger(ctx, "bob", "Home", 23, time.Now())
	if err != nil {
		t.Fatalf("create ledger: %v", err)
	}

	var payday int
	var currency string
	if err := db.QueryRow(`SELECT payday_day, currency FROM ledger WHERE id = ?`, ledgerID).Scan(&payday, &currency); err != nil {
		t.Fatalf("read ledger: %v", err)
	}
	if payday != 23 || currency != "EUR" {
		t.Errorf("payday, currency = %d, %s; want 23, EUR", payday, currency)
	}

	var defaults int
	if err := db.QueryRow(`SELECT COUNT(*) FROM pot WHERE ledger_id = ? AND is_default = 1`, ledgerID).Scan(&defaults); err != nil {
		t.Fatalf("count default pots: %v", err)
	}
	if defaults != 1 {
		t.Errorf("default pots = %d, want 1", defaults)
	}
}

func TestPaydayDefaultsAndRange(t *testing.T) {
	db, s := setup(t)
	ctx := context.Background()
	mustExec(t, db, `INSERT INTO user (id, name, created_at) VALUES ('bob', 'Bob', '2026-01-01T00:00:00Z')`)

	if _, err := s.CreatePersonalLedger(ctx, "bob", "Default", 0, time.Now()); err != nil {
		t.Errorf("payday 0 should default to 1, got %v", err)
	}
	if _, err := s.CreatePersonalLedger(ctx, "bob", "Too late", 32, time.Now()); !errors.Is(err, ErrInvalidPayday) {
		t.Errorf("payday 32: err = %v, want ErrInvalidPayday", err)
	}
}

func TestInvalidNamesAreRejected(t *testing.T) {
	_, s := setup(t)
	ctx := context.Background()

	if _, err := s.CreateCategory(ctx, "l1", strings.Repeat("x", 51), false); !errors.Is(err, ErrInvalidName) {
		t.Errorf("51-character category name: err = %v, want ErrInvalidName", err)
	}
	if _, err := s.CreateCategory(ctx, "l1", "   ", false); !errors.Is(err, ErrInvalidName) {
		t.Errorf("blank category name: err = %v, want ErrInvalidName", err)
	}
}

func TestCategoryNamesAreUniquePerLedgerIgnoringCase(t *testing.T) {
	_, s := setup(t)
	ctx := context.Background()

	// "Groceries" already exists in the test setup for ledger l1.
	if _, err := s.CreateCategory(ctx, "l1", "groceries", false); err == nil {
		t.Fatal("expected a duplicate name to be rejected, got nil")
	}
	if _, err := s.CreateCategory(ctx, "l1", "Rent", false); err != nil {
		t.Errorf("new name should be accepted: %v", err)
	}
}
