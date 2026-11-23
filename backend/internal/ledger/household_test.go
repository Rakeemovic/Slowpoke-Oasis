package ledger

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/Rakeemovic/slowpoke-oasis/backend/internal/store"
)

func TestCreateHouseholdMakesCreatorOwnerWithSharedLedger(t *testing.T) {
	db, s := setup(t)
	ctx := context.Background()

	hid, lid, err := s.CreateHousehold(ctx, "u1", "Flat", time.Now())
	if err != nil {
		t.Fatalf("create household: %v", err)
	}
	role, err := s.HouseholdRole(ctx, "u1", hid)
	if err != nil || role != "owner" {
		t.Fatalf("role = %q, err = %v; want owner", role, err)
	}

	var householdID, userID string
	var defaultPots int
	if err := db.QueryRow(`SELECT household_id, COALESCE(user_id, '') FROM ledger WHERE id = ?`, lid).Scan(&householdID, &userID); err != nil {
		t.Fatal(err)
	}
	if householdID != hid || userID != "" {
		t.Errorf("ledger household_id = %q, user_id = %q; want %q and empty", householdID, userID, hid)
	}
	if err := db.QueryRow(`SELECT COUNT(*) FROM pot WHERE ledger_id = ? AND is_default = 1`, lid).Scan(&defaultPots); err != nil {
		t.Fatal(err)
	}
	if defaultPots != 1 {
		t.Errorf("default pots = %d, want 1 (rule L4)", defaultPots)
	}
}

func TestOwnerLeavingHandsOverAndLastMemberDeletesHousehold(t *testing.T) {
	db, s := setup(t)
	ctx := context.Background()
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
	cat, err := s.CreateCategory(ctx, lid, "Garden", false)
	if err != nil {
		t.Fatal(err)
	}
	book := func(user string, cents int64) string {
		id, err := s.CreateTransaction(ctx, NewTransaction{
			LedgerID: lid, UserID: user, Type: TypeExpense,
			CategoryID: cat, AmountCents: cents, BookedOn: "2026-11-01",
		}, time.Now())
		if err != nil {
			t.Fatalf("book for %s: %v", user, err)
		}
		return id
	}
	ownerBooking := book("u1", 50000)
	memberBooking := book("bob", 2000)

	// The owner leaves: their booking stays under the placeholder and the
	// longest-standing member (bob) becomes owner.
	if err := s.LeaveHousehold(ctx, hid, "u1"); err != nil {
		t.Fatalf("owner leaves: %v", err)
	}
	var owner string
	if err := db.QueryRow(`SELECT user_id FROM transactions WHERE id = ?`, ownerBooking).Scan(&owner); err != nil {
		t.Fatal(err)
	}
	if owner != FormerMemberID {
		t.Errorf("booking of the leaving owner has user_id %q, want %q", owner, FormerMemberID)
	}
	if role, _ := s.HouseholdRole(ctx, "bob", hid); role != "owner" {
		t.Errorf("bob's role = %q, want owner after handover", role)
	}
	_ = memberBooking

	// Bob and Carol leave; the last one out removes the household and its data.
	if err := s.LeaveHousehold(ctx, hid, "bob"); err != nil {
		t.Fatal(err)
	}
	if err := s.LeaveHousehold(ctx, hid, "carol"); err != nil {
		t.Fatal(err)
	}
	for _, q := range []string{
		`SELECT COUNT(*) FROM household WHERE id = '` + hid + `'`,
		`SELECT COUNT(*) FROM ledger WHERE id = '` + lid + `'`,
		`SELECT COUNT(*) FROM transactions`,
	} {
		var n int
		if err := db.QueryRow(q).Scan(&n); err != nil {
			t.Fatal(err)
		}
		if n != 0 {
			t.Errorf("%s: %d rows left, want 0", q, n)
		}
	}
}

func TestDeletionShowsDeleterAndCanBeRestoredWithinRetention(t *testing.T) {
	_, s := setup(t)
	ctx := context.Background()
	now := time.Now()

	id := createExpense(t, s, "p-default", "2026-11-16", 100)
	if err := s.DeleteTransaction(ctx, "l1", id, "u1", now); err != nil {
		t.Fatal(err)
	}

	deleted, err := s.ListDeletedTransactions(ctx, "l1", now, 0, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(deleted) != 1 || deleted[0].DeletedBy != "u1" {
		t.Fatalf("deleted = %+v, want one entry deleted by u1", deleted)
	}

	if err := s.RestoreTransaction(ctx, "l1", id, now); err != nil {
		t.Fatalf("restore: %v", err)
	}
	deleted, _ = s.ListDeletedTransactions(ctx, "l1", now, 0, 0)
	if len(deleted) != 0 {
		t.Errorf("%d deleted entries after restore, want 0", len(deleted))
	}
	active, err := s.ListTransactions(ctx, ListFilter{LedgerID: "l1"})
	if err != nil {
		t.Fatal(err)
	}
	if len(active) != 1 {
		t.Errorf("%d active transactions after restore, want 1", len(active))
	}
}

func TestRestoreFailsAfterRetentionPeriod(t *testing.T) {
	db, s := setup(t)
	ctx := context.Background()
	now := time.Now()

	id := createExpense(t, s, "p-default", "2026-11-16", 100)
	if err := s.DeleteTransaction(ctx, "l1", id, "u1", now); err != nil {
		t.Fatal(err)
	}
	mustExec(t, db, `UPDATE transactions SET deleted_at = '`+now.AddDate(0, 0, -31).UTC().Format(time.RFC3339)+`' WHERE id = '`+id+`'`)

	if err := s.RestoreTransaction(ctx, "l1", id, now); !errors.Is(err, ErrTransactionNotFound) {
		t.Errorf("restore after 31 days: err = %v, want ErrTransactionNotFound", err)
	}
	deleted, _ := s.ListDeletedTransactions(ctx, "l1", now, 0, 0)
	if len(deleted) != 0 {
		t.Errorf("%d entries listed after retention, want 0", len(deleted))
	}
}

func TestTransferOwnershipSwapsRolesAndKeepsOneOwner(t *testing.T) {
	db, s := setup(t)
	ctx := context.Background()
	mustExec(t, db, `INSERT INTO user (id, name, created_at) VALUES ('bob', 'Bob', '2026-01-01T00:00:00Z')`)
	mustExec(t, db, `INSERT INTO user (id, name, created_at) VALUES ('carol', 'Carol', '2026-01-01T00:00:00Z')`)

	hid, _, err := s.CreateHousehold(ctx, "u1", "Flat", time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if err := s.AddHouseholdMember(ctx, hid, "bob"); err != nil {
		t.Fatal(err)
	}

	roleOf := func(user string) string {
		t.Helper()
		role, err := s.HouseholdRole(ctx, user, hid)
		if err != nil {
			t.Fatalf("role of %s: %v", user, err)
		}
		return role
	}

	if err := s.TransferOwnership(ctx, hid, "u1", "carol"); !errors.Is(err, ErrMemberNotFound) {
		t.Errorf("transfer to non-member: err = %v, want ErrMemberNotFound", err)
	}
	if err := s.TransferOwnership(ctx, hid, "bob", "bob"); !errors.Is(err, ErrNotOwner) {
		t.Errorf("member transfers: err = %v, want ErrNotOwner", err)
	}
	if err := s.TransferOwnership(ctx, hid, "u1", "u1"); err != nil || roleOf("u1") != RoleOwner {
		t.Errorf("transfer to oneself: err = %v, role = %q; want no change", err, roleOf("u1"))
	}

	if err := s.TransferOwnership(ctx, hid, "u1", "bob"); err != nil {
		t.Fatalf("transfer to bob: %v", err)
	}
	if roleOf("u1") != RoleMember || roleOf("bob") != RoleOwner {
		t.Errorf("roles after transfer: u1 = %q, bob = %q; want member and owner", roleOf("u1"), roleOf("bob"))
	}

	// The former owner can no longer transfer the role.
	if err := s.TransferOwnership(ctx, hid, "u1", "u1"); !errors.Is(err, ErrNotOwner) {
		t.Errorf("former owner transfers again: err = %v, want ErrNotOwner", err)
	}
	var owners int
	if err := db.QueryRow(`SELECT COUNT(*) FROM household_member WHERE household_id = ? AND role = 'owner'`, hid).Scan(&owners); err != nil {
		t.Fatal(err)
	}
	if owners != 1 {
		t.Errorf("%d owners, want 1", owners)
	}
}

func TestHouseholdCannotHaveSecondLedger(t *testing.T) {
	db, s := setup(t)
	hid, _, err := s.CreateHousehold(context.Background(), "u1", "Flat", time.Now())
	if err != nil {
		t.Fatalf("create household: %v", err)
	}
	_, err = db.Exec(`INSERT INTO ledger (id, name, household_id, created_at) VALUES ('l-second', 'Second', ?, '2026-01-01T00:00:00Z')`, hid)
	if !store.IsUniqueViolation(err) {
		t.Errorf("second household ledger: err = %v, want unique violation (rule L5)", err)
	}
}
