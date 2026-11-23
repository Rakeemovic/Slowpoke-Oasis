package ledger

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"
)

func ptr[T any](v T) *T { return &v }

// categoryByName finds a category of the ledger or fails the test.
func categoryByName(t *testing.T, s *Store, ledgerID, name string) Category {
	t.Helper()
	list, err := s.ListCategories(context.Background(), ledgerID)
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range list {
		if c.Name == name {
			return c
		}
	}
	t.Fatalf("category %q not found in ledger %s", name, ledgerID)
	return Category{}
}

func TestDefaultCategoriesHavePurposesAndFallbacks(t *testing.T) {
	_, s := setup(t)
	ctx := context.Background()
	lid, err := s.CreatePersonalLedger(ctx, "u1", "Savings", 0, time.Now())
	if err != nil {
		t.Fatal(err)
	}

	if c := categoryByName(t, s, lid, "Groceries"); c.Purpose != PurposeNeed {
		t.Errorf("Groceries purpose = %q, want need", c.Purpose)
	}
	if c := categoryByName(t, s, lid, "Leisure"); c.Purpose != PurposeWant {
		t.Errorf("Leisure purpose = %q, want want", c.Purpose)
	}
	for _, name := range []string{"Other", "Other income"} {
		c := categoryByName(t, s, lid, name)
		if !c.IsFallback || c.Purpose != "" {
			t.Errorf("%s: fallback = %v, purpose = %q; want fallback without purpose", name, c.IsFallback, c.Purpose)
		}
	}
}

func TestPurposeRules(t *testing.T) {
	_, s := setup(t)
	ctx := context.Background()

	if _, err := s.CreateCategory(ctx, NewCategory{LedgerID: "l1", Name: "Bonus", IsIncome: true, Purpose: PurposeNeed}); !errors.Is(err, ErrPurposeNeedsExpense) {
		t.Errorf("income with purpose: err = %v, want ErrPurposeNeedsExpense", err)
	}
	if _, err := s.CreateCategory(ctx, NewCategory{LedgerID: "l1", Name: "Fun", Purpose: "luxury"}); !errors.Is(err, ErrInvalidPurpose) {
		t.Errorf("unknown purpose: err = %v, want ErrInvalidPurpose", err)
	}
	id, err := s.CreateCategory(ctx, NewCategory{LedgerID: "l1", Name: "ETF", Purpose: PurposeSaving})
	if err != nil {
		t.Fatal(err)
	}

	if err := s.UpdateCategory(ctx, "l1", "c-salary", CategoryUpdate{Purpose: ptr(PurposeWant)}); !errors.Is(err, ErrPurposeNeedsExpense) {
		t.Errorf("purpose on income: err = %v, want ErrPurposeNeedsExpense", err)
	}
	if err := s.UpdateCategory(ctx, "l1", id, CategoryUpdate{Purpose: ptr(Purpose(""))}); err != nil {
		t.Fatal(err)
	}
	if c := categoryByName(t, s, "l1", "ETF"); c.Purpose != "" {
		t.Errorf("purpose = %q after removing it, want empty", c.Purpose)
	}
}

func TestUpdateCategoryRenamesAndArchives(t *testing.T) {
	_, s := setup(t)
	ctx := context.Background()

	if err := s.UpdateCategory(ctx, "l1", "c-food", CategoryUpdate{Name: ptr(" Food "), Archived: ptr(true)}); err != nil {
		t.Fatal(err)
	}
	c := categoryByName(t, s, "l1", "Food")
	if !c.Archived {
		t.Error("category is not archived")
	}
	if err := s.UpdateCategory(ctx, "l1", "c-food", CategoryUpdate{Name: ptr("salary")}); !errors.Is(err, ErrNameTaken) {
		t.Errorf("duplicate name: err = %v, want ErrNameTaken", err)
	}
	if err := s.UpdateCategory(ctx, "l1", "missing", CategoryUpdate{Name: ptr("X")}); !errors.Is(err, ErrCategoryNotFound) {
		t.Errorf("unknown category: err = %v, want ErrCategoryNotFound", err)
	}
}

func TestArchivedCategoryRejectsNewBookingsOnly(t *testing.T) {
	_, s := setup(t)
	ctx := context.Background()

	existing, err := s.CreateTransaction(ctx, baseExpense(), time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if err := s.UpdateCategory(ctx, "l1", "c-food", CategoryUpdate{Archived: ptr(true)}); err != nil {
		t.Fatal(err)
	}

	if _, err := s.CreateTransaction(ctx, baseExpense(), time.Now()); !errors.Is(err, ErrCategoryArchived) {
		t.Errorf("new booking: err = %v, want ErrCategoryArchived", err)
	}
	// The existing booking can still be corrected.
	if err := s.UpdateTransaction(ctx, "l1", existing, TransactionPatch{AmountCents: ptr(int64(999))}, time.Now()); err != nil {
		t.Errorf("correcting an existing booking: %v", err)
	}
	// Moving another booking into the archived category is refused.
	other, err := s.CreateCategory(ctx, NewCategory{LedgerID: "l1", Name: "Garden"})
	if err != nil {
		t.Fatal(err)
	}
	in := baseExpense()
	in.CategoryID = other
	id, err := s.CreateTransaction(ctx, in, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if err := s.UpdateTransaction(ctx, "l1", id, TransactionPatch{CategoryID: ptr("c-food")}, time.Now()); !errors.Is(err, ErrCategoryArchived) {
		t.Errorf("moving into archived category: err = %v, want ErrCategoryArchived", err)
	}
}

func TestCategoryLimitCountsArchived(t *testing.T) {
	_, s := setup(t)
	ctx := context.Background()

	// setup creates 2 categories; fill up to the limit, the last one archived.
	var last string
	for i := 2; i < maxCategoriesPerLedger; i++ {
		id, err := s.CreateCategory(ctx, NewCategory{LedgerID: "l1", Name: fmt.Sprintf("Cat %d", i)})
		if err != nil {
			t.Fatalf("category %d: %v", i, err)
		}
		last = id
	}
	if err := s.UpdateCategory(ctx, "l1", last, CategoryUpdate{Archived: ptr(true)}); err != nil {
		t.Fatal(err)
	}
	if _, err := s.CreateCategory(ctx, NewCategory{LedgerID: "l1", Name: "One more"}); !errors.Is(err, ErrCategoryLimit) {
		t.Errorf("err = %v, want ErrCategoryLimit", err)
	}
	// Deleting still works at the limit, although it creates the fallback.
	if _, err := s.DeleteCategory(ctx, "l1", last, ""); err != nil {
		t.Errorf("delete at the limit: %v", err)
	}
}

func TestDeleteCategoryMovesTransactionsAndPlans(t *testing.T) {
	db, s := setup(t)
	ctx := context.Background()
	now := time.Now()

	// The ledger from setup has no fallback yet; deleting creates "Other".
	active, err := s.CreateTransaction(ctx, baseExpense(), now)
	if err != nil {
		t.Fatal(err)
	}
	deleted, err := s.CreateTransaction(ctx, baseExpense(), now)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.DeleteTransaction(ctx, "l1", deleted, "u1", now); err != nil {
		t.Fatal(err)
	}
	if err := s.SetPlannedAmount(ctx, planChange("p-default", "c-food", 30000, ScopeFromNowOn)); err != nil {
		t.Fatal(err)
	}

	res, err := s.DeleteCategory(ctx, "l1", "c-food", "")
	if err != nil {
		t.Fatal(err)
	}
	if res.MovedTransactions != 2 {
		t.Errorf("moved = %d, want 2 (soft-deleted ones included)", res.MovedTransactions)
	}
	other := categoryByName(t, s, "l1", "Other")
	if !other.IsFallback || other.ID != res.TargetID {
		t.Fatalf("fallback = %+v, want the new Other category %s", other, res.TargetID)
	}
	for _, id := range []string{active, deleted} {
		var cat string
		if err := db.QueryRow(`SELECT category_id FROM transactions WHERE id = ?`, id).Scan(&cat); err != nil {
			t.Fatal(err)
		}
		if cat != other.ID {
			t.Errorf("transaction %s: category = %s, want %s", id, cat, other.ID)
		}
	}

	// A second deletion reuses the fallback and adds planned amounts up.
	garden, err := s.CreateCategory(ctx, NewCategory{LedgerID: "l1", Name: "Garden"})
	if err != nil {
		t.Fatal(err)
	}
	if err := s.SetPlannedAmount(ctx, planChange("p-default", garden, 5000, ScopeFromNowOn)); err != nil {
		t.Fatal(err)
	}
	if res, err = s.DeleteCategory(ctx, "l1", garden, ""); err != nil || res.TargetID != other.ID {
		t.Fatalf("second delete: %+v, %v", res, err)
	}
	var planned int64
	if err := db.QueryRow(`SELECT amount_cents FROM pot_line WHERE pot_id = 'p-default' AND category_id = ?`, other.ID).Scan(&planned); err != nil {
		t.Fatal(err)
	}
	if planned != 35000 {
		t.Errorf("planned = %d, want 35000", planned)
	}
}

func TestDeleteIncomeCategoryUsesIncomeFallback(t *testing.T) {
	_, s := setup(t)
	ctx := context.Background()

	res, err := s.DeleteCategory(ctx, "l1", "c-salary", "")
	if err != nil {
		t.Fatal(err)
	}
	if c := categoryByName(t, s, "l1", "Other income"); c.ID != res.TargetID || !c.IsIncome {
		t.Errorf("fallback = %+v, want income category Other income", c)
	}
}

func TestFallbackCategoryIsProtected(t *testing.T) {
	_, s := setup(t)
	ctx := context.Background()
	lid, err := s.CreatePersonalLedger(ctx, "u1", "Savings", 0, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	other := categoryByName(t, s, lid, "Other")

	if _, err := s.DeleteCategory(ctx, lid, other.ID, ""); !errors.Is(err, ErrFallbackProtected) {
		t.Errorf("delete: err = %v, want ErrFallbackProtected", err)
	}
	if err := s.UpdateCategory(ctx, lid, other.ID, CategoryUpdate{Archived: ptr(true)}); !errors.Is(err, ErrFallbackProtected) {
		t.Errorf("archive: err = %v, want ErrFallbackProtected", err)
	}
	if err := s.UpdateCategory(ctx, lid, other.ID, CategoryUpdate{Name: ptr("Misc")}); err != nil {
		t.Errorf("rename: %v", err)
	}
	// Renamed, it is still the fallback.
	garden, err := s.CreateCategory(ctx, NewCategory{LedgerID: lid, Name: "Garden"})
	if err != nil {
		t.Fatal(err)
	}
	if res, err := s.DeleteCategory(ctx, lid, garden, ""); err != nil || res.TargetID != other.ID {
		t.Errorf("delete after rename: %+v, %v; want fallback %s", res, err, other.ID)
	}
}

func TestExistingCategoryWithFallbackNameBecomesFallback(t *testing.T) {
	db, s := setup(t)
	ctx := context.Background()
	// An old ledger: "Other" exists but is archived and not marked.
	mustExec(t, db, `INSERT INTO category (id, ledger_id, name, archived) VALUES ('c-other', 'l1', 'other', 1)`)

	// Deleting "other" itself marks it first and is then refused, without
	// leaving any change behind.
	if _, err := s.DeleteCategory(ctx, "l1", "c-other", ""); !errors.Is(err, ErrFallbackProtected) {
		t.Errorf("delete other: err = %v, want ErrFallbackProtected", err)
	}
	if c := categoryByName(t, s, "l1", "other"); c.IsFallback || !c.Archived {
		t.Errorf("after refused delete: %+v, want unchanged", c)
	}

	res, err := s.DeleteCategory(ctx, "l1", "c-food", "")
	if err != nil {
		t.Fatal(err)
	}
	c := categoryByName(t, s, "l1", "other")
	if res.TargetID != "c-other" || !c.IsFallback || c.Archived {
		t.Errorf("fallback = %+v (res %+v), want c-other marked and restored", c, res)
	}
}

func TestFallbackNameUsedByOtherKind(t *testing.T) {
	db, s := setup(t)
	ctx := context.Background()
	mustExec(t, db, `INSERT INTO category (id, ledger_id, name, is_income) VALUES ('c-other', 'l1', 'Other', 1)`)

	if _, err := s.DeleteCategory(ctx, "l1", "c-food", ""); !errors.Is(err, ErrFallbackNameTaken) {
		t.Errorf("err = %v, want ErrFallbackNameTaken", err)
	}
}

func TestAuthorizeLedgerOwner(t *testing.T) {
	db, s := setup(t)
	ctx := context.Background()
	mustExec(t, db, `INSERT INTO user (id, name, created_at) VALUES ('bob', 'Bob', '2026-01-01T00:00:00Z')`)
	hid, lid, err := s.CreateHousehold(ctx, "u1", "Flat", time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if err := s.AddHouseholdMember(ctx, hid, "bob"); err != nil {
		t.Fatal(err)
	}

	cases := []struct {
		user, ledger string
		want         error
	}{
		{"u1", "l1", nil},
		{"u1", lid, nil},
		{"bob", lid, ErrNotOwner},
		{"bob", "l1", ErrNotOwner},
		{"u1", "missing", ErrLedgerNotFound},
	}
	for _, c := range cases {
		if err := s.AuthorizeLedgerOwner(ctx, c.user, c.ledger); !errors.Is(err, c.want) {
			t.Errorf("%s on %s: err = %v, want %v", c.user, c.ledger, err, c.want)
		}
	}
}

func TestSpontaneousOnlyForExpenses(t *testing.T) {
	db, s := setup(t)
	ctx := context.Background()

	in := baseExpense()
	in.IsSpontaneous = true
	id, err := s.CreateTransaction(ctx, in, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	got, err := s.GetTransaction(ctx, "l1", id)
	if err != nil || !got.IsSpontaneous {
		t.Fatalf("got %+v, %v; want spontaneous", got, err)
	}

	income := baseExpense()
	income.Type, income.CategoryID, income.IsSpontaneous = TypeIncome, "c-salary", true
	if _, err := s.CreateTransaction(ctx, income, time.Now()); !errors.Is(err, ErrSpontaneousExpense) {
		t.Errorf("spontaneous income: err = %v, want ErrSpontaneousExpense", err)
	}

	// Turning the expense into income drops the mark.
	typ := TypeIncome
	if err := s.UpdateTransaction(ctx, "l1", id, TransactionPatch{Type: &typ, CategoryID: ptr("c-salary")}, time.Now()); err != nil {
		t.Fatal(err)
	}
	var spontaneous bool
	if err := db.QueryRow(`SELECT is_spontaneous FROM transactions WHERE id = ?`, id).Scan(&spontaneous); err != nil {
		t.Fatal(err)
	}
	if spontaneous {
		t.Error("income still marked as spontaneous")
	}
}

func TestDeleteCategoryMovesToChosenTarget(t *testing.T) {
	db, s := setup(t)
	ctx := context.Background()

	garden, err := s.CreateCategory(ctx, NewCategory{LedgerID: "l1", Name: "Garden"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.CreateTransaction(ctx, baseExpense(), time.Now()); err != nil {
		t.Fatal(err)
	}
	if err := s.SetPlannedAmount(ctx, planChange("p-default", "c-food", 1000, ScopeFromNowOn)); err != nil {
		t.Fatal(err)
	}

	res, err := s.DeleteCategory(ctx, "l1", "c-food", garden)
	if err != nil {
		t.Fatal(err)
	}
	if res.TargetID != garden || res.MovedTransactions != 1 {
		t.Errorf("res = %+v, want 1 transaction moved to Garden", res)
	}
	if c := categoryByName(t, s, "l1", "Garden"); c.TransactionCount != 1 {
		t.Errorf("Garden has %d transactions, want 1", c.TransactionCount)
	}
	var planned int64
	if err := db.QueryRow(`SELECT amount_cents FROM pot_line WHERE category_id = ?`, garden).Scan(&planned); err != nil || planned != 1000 {
		t.Errorf("planned = %d, %v; want 1000", planned, err)
	}
	// With a chosen target, no fallback category is created.
	var fallbacks int
	if err := db.QueryRow(`SELECT COUNT(*) FROM category WHERE ledger_id = 'l1' AND is_fallback = 1`).Scan(&fallbacks); err != nil || fallbacks != 0 {
		t.Errorf("fallbacks = %d, %v; want 0", fallbacks, err)
	}
}

func TestDeleteCategoryRejectsInvalidTargets(t *testing.T) {
	_, s := setup(t)
	ctx := context.Background()

	archived, err := s.CreateCategory(ctx, NewCategory{LedgerID: "l1", Name: "Old"})
	if err != nil {
		t.Fatal(err)
	}
	if err := s.UpdateCategory(ctx, "l1", archived, CategoryUpdate{Archived: ptr(true)}); err != nil {
		t.Fatal(err)
	}

	for name, target := range map[string]string{
		"itself":     "c-food",
		"other kind": "c-salary",
		"archived":   archived,
		"unknown":    "missing",
	} {
		if _, err := s.DeleteCategory(ctx, "l1", "c-food", target); !errors.Is(err, ErrInvalidMoveTarget) {
			t.Errorf("%s: err = %v, want ErrInvalidMoveTarget", name, err)
		}
	}
	if c := categoryByName(t, s, "l1", "Groceries"); c.ID != "c-food" {
		t.Error("category was deleted despite the invalid target")
	}
}
