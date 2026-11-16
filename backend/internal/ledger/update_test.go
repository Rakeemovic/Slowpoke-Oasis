package ledger

import (
	"errors"
	"testing"
	"time"
)

func TestUpdateChangesFieldsAndKeepsAuthor(t *testing.T) {
	_, s := setup(t)
	ctx := t.Context()
	id := createExpense(t, s, "p-default", "2026-11-10", 1000)

	amount, note := int64(1500), "groceries"
	if err := s.UpdateTransaction(ctx, "l1", id, TransactionPatch{AmountCents: &amount, Note: &note}, time.Now()); err != nil {
		t.Fatalf("update: %v", err)
	}
	list, err := s.ListTransactions(ctx, ListFilter{LedgerID: "l1"})
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 1 || list[0].AmountCents != -1500 || list[0].Note != "groceries" || list[0].UserID != "u1" {
		t.Errorf("after update: %+v, want amount -1500, note groceries, author u1", list)
	}
}

func TestUpdateSwitchesTypeUnderTheSameRules(t *testing.T) {
	db, s := setup(t)
	ctx := t.Context()
	id := createExpense(t, s, "p-default", "2026-11-10", 1000)

	income := TypeIncome
	food := "c-food"
	if err := s.UpdateTransaction(ctx, "l1", id, TransactionPatch{Type: &income}, time.Now()); !errors.Is(err, ErrCategoryKindMismatch) {
		t.Errorf("expense category as income: err = %v, want ErrCategoryKindMismatch", err)
	}

	salary := "c-salary"
	if err := s.UpdateTransaction(ctx, "l1", id, TransactionPatch{Type: &income, CategoryID: &salary}, time.Now()); err != nil {
		t.Fatalf("switch to income: %v", err)
	}
	var stored int64
	if err := db.QueryRow(`SELECT amount_cents FROM transactions WHERE id = ?`, id).Scan(&stored); err != nil {
		t.Fatal(err)
	}
	if stored != 1000 {
		t.Errorf("stored amount = %d, want +1000 after switching to income", stored)
	}

	transfer := TypeTransfer
	if err := s.UpdateTransaction(ctx, "l1", id, TransactionPatch{Type: &transfer, CategoryID: &food}, time.Now()); err == nil {
		t.Error("transfer without counterpart pot was accepted")
	}
}

func TestUpdateCannotMoveIntoArchivedPot(t *testing.T) {
	db, s := setup(t)
	ctx := t.Context()
	id := createExpense(t, s, "p-default", "2026-11-10", 1000)
	mustExec(t, db, `UPDATE pot SET archived = 1 WHERE id = 'p-holiday'`)

	holiday := "p-holiday"
	if err := s.UpdateTransaction(ctx, "l1", id, TransactionPatch{PotID: &holiday}, time.Now()); !errors.Is(err, ErrPotArchived) {
		t.Errorf("move into archived pot: err = %v, want ErrPotArchived", err)
	}
}

func TestUpdateDeletedTransactionIsNotFound(t *testing.T) {
	_, s := setup(t)
	ctx := t.Context()
	id := createExpense(t, s, "p-default", "2026-11-10", 1000)
	if err := s.DeleteTransaction(ctx, "l1", id, "u1", time.Now()); err != nil {
		t.Fatal(err)
	}
	note := "late"
	if err := s.UpdateTransaction(ctx, "l1", id, TransactionPatch{Note: &note}, time.Now()); !errors.Is(err, ErrTransactionNotFound) {
		t.Errorf("update deleted: err = %v, want ErrTransactionNotFound", err)
	}
}
