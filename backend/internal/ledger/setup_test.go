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

// Personal and household ledgers both start with the default categories
// (rule K4), so a new user can book without creating a category first.
func TestNewLedgersStartWithDefaultCategories(t *testing.T) {
	_, s := setup(t)
	ctx := context.Background()

	personal, err := s.CreatePersonalLedger(ctx, "u1", "Savings", 0, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	_, household, err := s.CreateHousehold(ctx, "u1", "Flat", time.Now())
	if err != nil {
		t.Fatal(err)
	}

	for _, ledgerID := range []string{personal, household} {
		got, err := s.ListCategories(ctx, ledgerID)
		if err != nil {
			t.Fatal(err)
		}
		if len(got) != len(defaultCategories) {
			t.Fatalf("ledger %s: got %d categories, want %d", ledgerID, len(got), len(defaultCategories))
		}
		income := map[string]bool{}
		for _, c := range got {
			income[c.Name] = c.IsIncome
		}
		for _, want := range defaultCategories {
			isIncome, ok := income[want.name]
			if !ok || isIncome != want.isIncome {
				t.Errorf("ledger %s: category %q missing or wrong is_income", ledgerID, want.name)
			}
		}
	}
}
