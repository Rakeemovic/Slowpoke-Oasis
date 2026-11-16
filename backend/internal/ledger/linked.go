package ledger

import (
	"context"
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

	fromPot, err := s.resolvePot(ctx, in.FromLedgerID, in.FromPotID)
	if err != nil {
		return "", "", err
	}
	toPot, err := s.resolvePot(ctx, in.ToLedgerID, in.ToPotID)
	if err != nil {
		return "", "", err
	}

	// The same checks as for a normal transfer (amount, date, note).
	check := NewTransaction{
		LedgerID:         in.FromLedgerID,
		UserID:           in.UserID,
		Type:             TypeTransfer,
		PotID:            fromPot,
		CounterpartPotID: toPot,
		AmountCents:      in.AmountCents,
		BookedOn:         in.BookedOn,
		Note:             in.Note,
	}
	if err := check.Validate(); err != nil {
		return "", "", err
	}
	if err := s.ensurePotActive(ctx, in.FromLedgerID, fromPot); err != nil {
		return "", "", err
	}
	if err := s.ensurePotActive(ctx, in.ToLedgerID, toPot); err != nil {
		return "", "", err
	}

	outID, err := newID()
	if err != nil {
		return "", "", err
	}
	inID, err := newID()
	if err != nil {
		return "", "", err
	}
	ts := now.UTC().Format(time.RFC3339)

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return "", "", err
	}
	defer tx.Rollback() // no effect after a successful Commit

	// The legs reference each other, so the foreign keys are checked at commit.
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO transactions (id, ledger_id, user_id, type, pot_id, amount_cents,
		                          booked_on, note, created_at, updated_at, linked_transaction_id)
		VALUES (?, ?, ?, 'transfer_out', ?, ?, ?, ?, ?, ?, ?)`,
		outID, in.FromLedgerID, in.UserID, fromPot, -in.AmountCents,
		in.BookedOn, nullIfEmpty(in.Note), ts, ts, inID,
	); err != nil {
		return "", "", err
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO transactions (id, ledger_id, user_id, type, pot_id, amount_cents,
		                          booked_on, note, created_at, updated_at, linked_transaction_id)
		VALUES (?, ?, ?, 'transfer_in', ?, ?, ?, ?, ?, ?, ?)`,
		inID, in.ToLedgerID, in.UserID, toPot, in.AmountCents,
		in.BookedOn, nullIfEmpty(in.Note), ts, ts, outID,
	); err != nil {
		return "", "", err
	}
	if err := tx.Commit(); err != nil {
		return "", "", err
	}
	return outID, inID, nil
}

// resolvePot returns potID, or the default pot of the ledger if potID is empty.
func (s *Store) resolvePot(ctx context.Context, ledgerID, potID string) (string, error) {
	if potID != "" {
		return potID, nil
	}
	var id string
	err := s.db.QueryRowContext(ctx,
		`SELECT id FROM pot WHERE ledger_id = ? AND is_default = 1`, ledgerID,
	).Scan(&id)
	return id, err
}
