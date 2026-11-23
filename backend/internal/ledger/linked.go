package ledger

import (
	"context"
	"database/sql"
	"errors"
	"time"
)

// Errors for transfers between ledgers.
var (
	ErrSameLedger        = errors.New("source and target are in the same ledger, use a normal transfer")
	ErrLinkedTransaction = errors.New("a transfer between ledgers can only be deleted or restored, not edited")
)

// LinkedTransfer moves money from a pot in one ledger to a pot in another
// ledger, for example from a personal ledger into a household ledger (rule X14).
// An empty pot ID means the default pot of that ledger.
type LinkedTransfer struct {
	UserID       string
	FromLedgerID string
	FromPotID    string
	ToLedgerID   string
	ToPotID      string
	AmountCents  int64
	BookedOn     string
	Note         string
}

// CreateLinkedTransfer books both legs of a transfer between two ledgers in one
// database transaction: a transfer_out in the source ledger and a transfer_in
// in the target ledger. Both legs have the same amount with opposite signs and
// point at each other. It returns the IDs of the outgoing and incoming leg.
//
// The caller must check that the user may access both ledgers.
func (s *Store) CreateLinkedTransfer(ctx context.Context, in LinkedTransfer, now time.Time) (string, string, error) {
	if in.FromLedgerID == in.ToLedgerID {
		return "", "", ErrSameLedger
	}
	if err := validateBooking(in.AmountCents, in.BookedOn, in.Note); err != nil {
		return "", "", err
	}

	outID, inID := newID(), newID()
	ts := timestamp(now)

	err := s.inTx(ctx, func(tx *sql.Tx) error {
		// insertLeg books one side. The legs reference each other, so the
		// foreign keys between them are only checked at commit.
		insertLeg := func(id, linkedID, ledgerID, potID string, typ TxType, amount int64) error {
			potID, err := resolvePot(ctx, tx, ledgerID, potID)
			if err != nil {
				return err
			}
			if err := ensurePotActive(ctx, tx, ledgerID, potID); err != nil {
				return err
			}
			guard, err := guardReserves(ctx, tx, potID) // rule R12
			if err != nil {
				return err
			}
			_, err = tx.ExecContext(ctx, `
				INSERT INTO transactions (id, ledger_id, user_id, type, pot_id, amount_cents,
				                          booked_on, note, created_at, updated_at, linked_transaction_id)
				VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
				id, ledgerID, in.UserID, typ, potID, amount,
				in.BookedOn, nullIfEmpty(in.Note), ts, ts, linkedID,
			)
			if err != nil {
				return err
			}
			return guard.check(ctx, tx)
		}

		if err := insertLeg(outID, inID, in.FromLedgerID, in.FromPotID, TypeTransferOut, -in.AmountCents); err != nil {
			return err
		}
		return insertLeg(inID, outID, in.ToLedgerID, in.ToPotID, TypeTransferIn, in.AmountCents)
	})
	if err != nil {
		return "", "", err
	}
	return outID, inID, nil
}

// LinkedPartner returns the ledger and ID of the other leg of a linked transfer,
// including a soft-deleted one. For a transaction that is not linked, or whose
// other leg was removed with its ledger (rule X16), both results are empty.
func (s *Store) LinkedPartner(ctx context.Context, ledgerID, id string) (string, string, error) {
	var partnerLedgerID, partnerID string
	err := s.db.QueryRowContext(ctx, `
		SELECT p.ledger_id, p.id
		FROM transactions t
		JOIN transactions p ON p.id = t.linked_transaction_id
		WHERE t.id = ? AND t.ledger_id = ?`,
		id, ledgerID,
	).Scan(&partnerLedgerID, &partnerID)
	if errors.Is(err, sql.ErrNoRows) {
		return "", "", nil
	}
	return partnerLedgerID, partnerID, err
}
