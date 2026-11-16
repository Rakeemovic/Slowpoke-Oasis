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

	summary, err := s.PotSummary(context.Background(), "l2", "p-l2", "", "")
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
