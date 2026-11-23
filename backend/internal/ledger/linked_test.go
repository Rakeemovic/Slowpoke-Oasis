package ledger

import (
	"context"
	"database/sql"
	"errors"
	"testing"
	"time"
)

// addSecondLedger adds ledger l2 with its own default pot (p-l2) to the test data.
func addSecondLedger(t *testing.T, db *sql.DB) {
	t.Helper()
	mustExec(t, db, `INSERT INTO ledger (id, name, user_id, created_at) VALUES ('l2', 'Gilde', 'u1', '2026-01-01T00:00:00Z')`)
	mustExec(t, db, `INSERT INTO pot (id, ledger_id, name, kind, is_default) VALUES ('p-l2', 'l2', 'Gemeinschaftskonto', 'monthly', 1)`)
}

func TestLinkedTransferMovesMoneyBetweenLedgers(t *testing.T) {
	db, s := setup(t)
	addSecondLedger(t, db)

	outID, inID, err := s.CreateLinkedTransfer(context.Background(), LinkedTransfer{
		UserID:       "u1",
		FromLedgerID: "l1",
		ToLedgerID:   "l2",
		AmountCents:  5000,
		BookedOn:     "2026-11-16",
	}, time.Now())
	if err != nil {
		t.Fatalf("create linked transfer: %v", err)
	}
	if outID == "" || inID == "" || outID == inID {
		t.Fatalf("got ids %q and %q, want two different ids", outID, inID)
	}

	if got := balances(t, s, "l1")["p-default"]; got != -5000 {
		t.Errorf("source balance = %d, want -5000", got)
	}
	if got := balances(t, s, "l2")["p-l2"]; got != 5000 {
		t.Errorf("target balance = %d, want 5000", got)
	}

	summary, err := s.PotSummary(context.Background(), SummaryQuery{LedgerID: "l2", PotID: "p-l2", Today: today})
	if err != nil {
		t.Fatalf("summary: %v", err)
	}
	if summary.TransfersInCents != 5000 {
		t.Errorf("transfers in = %d, want 5000", summary.TransfersInCents)
	}
}

func TestLinkedTransferRejectsSameLedger(t *testing.T) {
	_, s := setup(t)

	_, _, err := s.CreateLinkedTransfer(context.Background(), LinkedTransfer{
		UserID:       "u1",
		FromLedgerID: "l1",
		ToLedgerID:   "l1",
		AmountCents:  5000,
		BookedOn:     "2026-11-16",
	}, time.Now())
	if !errors.Is(err, ErrSameLedger) {
		t.Fatalf("got %v, want ErrSameLedger", err)
	}
}

func TestLinkedTransferRejectsZeroAmount(t *testing.T) {
	db, s := setup(t)
	addSecondLedger(t, db)

	_, _, err := s.CreateLinkedTransfer(context.Background(), LinkedTransfer{
		UserID:       "u1",
		FromLedgerID: "l1",
		ToLedgerID:   "l2",
		AmountCents:  0,
		BookedOn:     "2026-11-16",
	}, time.Now())
	if err == nil {
		t.Fatal("zero amount was accepted")
	}
}

func TestLinkedLegCannotBeEdited(t *testing.T) {
	db, s := setup(t)
	addSecondLedger(t, db)
	outID, inID, err := s.CreateLinkedTransfer(context.Background(), LinkedTransfer{
		UserID: "u1", FromLedgerID: "l1", ToLedgerID: "l2", AmountCents: 5000, BookedOn: "2026-11-16",
	}, time.Now())
	if err != nil {
		t.Fatalf("create linked transfer: %v", err)
	}

	amount := int64(100)
	for _, tc := range []struct{ ledgerID, id string }{{"l1", outID}, {"l2", inID}} {
		err := s.UpdateTransaction(context.Background(), tc.ledgerID, tc.id, TransactionPatch{AmountCents: &amount}, time.Now())
		if !errors.Is(err, ErrLinkedTransaction) {
			t.Errorf("update of leg %s: got %v, want ErrLinkedTransaction", tc.id, err)
		}
	}
}

func TestDeletingOneLegDeletesBothAndRestoreBringsBothBack(t *testing.T) {
	db, s := setup(t)
	addSecondLedger(t, db)
	outID, _, err := s.CreateLinkedTransfer(context.Background(), LinkedTransfer{
		UserID: "u1", FromLedgerID: "l1", ToLedgerID: "l2", AmountCents: 5000, BookedOn: "2026-11-16",
	}, time.Now())
	if err != nil {
		t.Fatalf("create linked transfer: %v", err)
	}

	if err := s.DeleteTransaction(context.Background(), "l1", outID, "u1", time.Now()); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if got := balances(t, s, "l2")["p-l2"]; got != 0 {
		t.Errorf("target balance after delete = %d, want 0", got)
	}
	if got := balances(t, s, "l1")["p-default"]; got != 0 {
		t.Errorf("source balance after delete = %d, want 0", got)
	}

	if err := s.RestoreTransaction(context.Background(), "l1", outID, time.Now()); err != nil {
		t.Fatalf("restore: %v", err)
	}
	if got := balances(t, s, "l2")["p-l2"]; got != 5000 {
		t.Errorf("target balance after restore = %d, want 5000", got)
	}
	if got := balances(t, s, "l1")["p-default"]; got != -5000 {
		t.Errorf("source balance after restore = %d, want -5000", got)
	}
}

func TestLinkedTransferFailsForUnknownTargetPot(t *testing.T) {
	db, s := setup(t)
	mustExec(t, db, `INSERT INTO ledger (id, name, user_id, created_at) VALUES ('l2', 'Gilde', 'u1', '2026-01-01T00:00:00Z')`)

	_, _, err := s.CreateLinkedTransfer(context.Background(), LinkedTransfer{
		UserID: "u1", FromLedgerID: "l1", ToLedgerID: "l2", ToPotID: "p-missing",
		AmountCents: 5000, BookedOn: "2026-11-16",
	}, time.Now())
	if err == nil {
		t.Fatal("transfer into a pot of another ledger was accepted")
	}
}

// When the last member leaves, the household and its ledger are deleted. The
// personal side of an earlier transfer into the household stays (rule X16).
func TestDeletedLedgerLeavesTheOtherLegWithoutLink(t *testing.T) {
	db, s := setup(t)
	ctx := context.Background()

	householdID, sharedID, err := s.CreateHousehold(ctx, "u1", "Flat", time.Now())
	if err != nil {
		t.Fatalf("create household: %v", err)
	}
	outID, _, err := s.CreateLinkedTransfer(ctx, LinkedTransfer{
		UserID:       "u1",
		FromLedgerID: "l1",
		ToLedgerID:   sharedID,
		AmountCents:  5000,
		BookedOn:     "2026-11-16",
	}, time.Now())
	if err != nil {
		t.Fatalf("create linked transfer: %v", err)
	}

	if err := s.LeaveHousehold(ctx, householdID, "u1"); err != nil {
		t.Fatalf("last member leaves: %v", err)
	}

	var linked sql.NullString
	if err := db.QueryRow(`SELECT linked_transaction_id FROM transactions WHERE id = ?`, outID).Scan(&linked); err != nil {
		t.Fatalf("the personal leg should still exist: %v", err)
	}
	if linked.Valid {
		t.Errorf("link = %q, want none: the other leg is gone", linked.String)
	}
	if got := balances(t, s, "l1")["p-default"]; got != -5000 {
		t.Errorf("source balance = %d, want -5000: the money did leave", got)
	}

	// The remaining leg behaves like any other booking of its author.
	if err := s.DeleteTransaction(ctx, "l1", outID, "u1", time.Now()); err != nil {
		t.Fatalf("delete remaining leg: %v", err)
	}
	if err := s.RestoreTransaction(ctx, "l1", outID, time.Now()); err != nil {
		t.Fatalf("restore remaining leg: %v", err)
	}
}
