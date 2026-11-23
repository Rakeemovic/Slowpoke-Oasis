package ledger

import (
	"errors"
	"testing"
	"time"
)

func TestOnlyAuthorDeletesAndOwnerEditsOthers(t *testing.T) {
	db, s := setup(t)
	ctx := t.Context()
	mustExec(t, db, `INSERT INTO user (id, name, created_at) VALUES ('bob', 'Bob', '2026-01-01T00:00:00Z')`)
	mustExec(t, db, `INSERT INTO user (id, name, created_at) VALUES ('carol', 'Carol', '2026-01-01T00:00:00Z')`)

	hid, lid, err := s.CreateHousehold(ctx, "u1", "Flat", time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if err := s.AddHouseholdMember(ctx, hid, "bob"); err != nil {
		t.Fatal(err)
	}
	if err := s.AddHouseholdMember(ctx, hid, "carol"); err != nil {
		t.Fatal(err)
	}
	cat, err := s.CreateCategory(ctx, NewCategory{LedgerID: lid, Name: "Garden"})
	if err != nil {
		t.Fatal(err)
	}
	book := func(user string) string {
		id, err := s.CreateTransaction(ctx, NewTransaction{
			LedgerID: lid, UserID: user, Type: TypeExpense,
			CategoryID: cat, AmountCents: 100, BookedOn: "2026-11-01",
		}, time.Now())
		if err != nil {
			t.Fatal(err)
		}
		return id
	}
	bobs := book("bob")
	carols := book("carol")

	auth := func(user, id string, a TransactionAction) error {
		return s.AuthorizeTransactionChange(ctx, user, lid, id, a)
	}

	// Editing: the author and the owner may, other members may not.
	if err := auth("bob", bobs, ActionEdit); err != nil {
		t.Errorf("author edits own booking: %v", err)
	}
	if err := auth("u1", bobs, ActionEdit); err != nil {
		t.Errorf("owner edits booking of bob: %v", err)
	}
	if err := auth("carol", bobs, ActionEdit); !errors.Is(err, ErrNotAllowed) {
		t.Errorf("carol edits bob's booking: err = %v, want ErrNotAllowed", err)
	}

	// Deleting: only the author, not the owner.
	if err := auth("u1", bobs, ActionDelete); !errors.Is(err, ErrNotAllowed) {
		t.Errorf("owner deletes bob's booking: err = %v, want ErrNotAllowed", err)
	}
	if err := auth("bob", bobs, ActionDelete); err != nil {
		t.Errorf("author deletes own booking: %v", err)
	}

	// After Carol leaves, her booking belongs to the placeholder. Only the owner may delete it.
	if err := s.LeaveHousehold(ctx, hid, "carol"); err != nil {
		t.Fatal(err)
	}
	if err := auth("carol", carols, ActionDelete); !errors.Is(err, ErrNotAllowed) {
		t.Errorf("former member deletes own old booking: err = %v, want ErrNotAllowed", err)
	}
	if err := auth("u1", carols, ActionDelete); err != nil {
		t.Errorf("owner deletes booking of a former member: %v", err)
	}

	if err := auth("u1", "unknown", ActionEdit); !errors.Is(err, ErrTransactionNotFound) {
		t.Errorf("unknown transaction: err = %v, want ErrTransactionNotFound", err)
	}
}

func TestPersonalLedgerOwnerMayChangeOwnBookings(t *testing.T) {
	_, s := setup(t)
	ctx := t.Context()
	id := createExpense(t, s, "p-default", "2026-11-10", 1000)

	if err := s.AuthorizeTransactionChange(ctx, "u1", "l1", id, ActionDelete); err != nil {
		t.Errorf("owner deletes own booking: %v", err)
	}
	if err := s.AuthorizeTransactionChange(ctx, "stranger", "l1", id, ActionEdit); !errors.Is(err, ErrNotAllowed) {
		t.Errorf("stranger edits: err = %v, want ErrNotAllowed", err)
	}
}
