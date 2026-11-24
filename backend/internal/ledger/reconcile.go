package ledger

import (
	"context"
	"database/sql"
	"errors"
	"time"
)

var (
	// ErrNotAnAccount: only the bank account of a ledger (its default pot)
	// and pots that are separate accounts at the bank can be reconciled.
	ErrNotAnAccount = errors.New("only the bank account and separate accounts can be reconciled")
	// ErrInvalidBalance: the entered balance is outside the amount limit (X11).
	ErrInvalidBalance = errors.New("the balance must be between -100,000,000 and 100,000,000")
	// ErrCorrectionTransaction: a correction can only be deleted or restored.
	ErrCorrectionTransaction = errors.New("a balance correction can only be deleted or restored, not edited")
)

// maxBalanceCents is the largest balance that can be entered, the same limit
// as for a single booking (rule X11).
const maxBalanceCents = 10_000_000_000

// Reconciliation is the last time the user compared an account with the bank.
type Reconciliation struct {
	ReconciledOn string // YYYY-MM-DD
	ActualCents  int64  // the balance the bank showed
}

// ReconcileInput is the real balance of an account on a day.
type ReconcileInput struct {
	LedgerID string
	UserID   string
	// PotID is a pot that is a separate account at the bank; empty means the
	// bank account of the ledger.
	PotID       string
	ActualCents int64
	// Today is the app's local date: the day of the balance and of the
	// correction.
	Today string
	// Correct books the difference as a correction (rule X22).
	Correct bool
}

// ReconcileResult compares the real balance with the ledger. DifferenceCents
// is ActualCents − ExpectedCents: positive when the bank holds more.
type ReconcileResult struct {
	PotID           string
	ExpectedCents   int64
	ActualCents     int64
	DifferenceCents int64
	// CorrectionID is the booked correction, empty if none was booked.
	CorrectionID string
}

// Reconcile compares the real balance of an account with the ledger, books
// the difference as a correction if asked to, and keeps it as the account's
// last reconciliation (rule L9). The bank account is compared with the
// estimated account balance, a separate account with its pot balance, both
// on in.Today. All of it happens in one database transaction, so a booking
// made at the same time cannot slip between comparing and correcting.
func (s *Store) Reconcile(ctx context.Context, in ReconcileInput, now time.Time) (ReconcileResult, error) {
	if !validDate(in.Today) {
		return ReconcileResult{}, ErrInvalidDate
	}
	if in.ActualCents > maxBalanceCents || in.ActualCents < -maxBalanceCents {
		return ReconcileResult{}, ErrInvalidBalance
	}
	var out ReconcileResult
	err := s.inTx(ctx, func(tx *sql.Tx) error {
		b, err := ledgerBalance(ctx, tx, in.LedgerID, in.Today)
		if err != nil {
			return err
		}
		potID, expected, err := accountBalance(ctx, tx, b, in.LedgerID, in.PotID)
		if err != nil {
			return err
		}
		out = ReconcileResult{
			PotID:           potID,
			ExpectedCents:   expected,
			ActualCents:     in.ActualCents,
			DifferenceCents: in.ActualCents - expected,
		}
		if in.Correct && out.DifferenceCents != 0 {
			if out.CorrectionID, err = bookCorrection(ctx, tx, in, potID, out.DifferenceCents, now); err != nil {
				return err
			}
		}
		return saveReconciliation(ctx, tx, in.LedgerID, potID, in.Today, in.ActualCents)
	})
	if err != nil {
		return ReconcileResult{}, err
	}
	return out, nil
}

// accountBalance returns the pot that stands for the account and the balance
// the ledger expects for it: the estimated account balance for the bank
// account, the pot balance for a separate account.
func accountBalance(ctx context.Context, q queryer, b Balance, ledgerID, potID string) (string, int64, error) {
	if potID == "" || potID == b.DefaultPotID {
		return b.DefaultPotID, b.EstimatedCents, nil
	}
	for _, a := range b.SeparateAccounts {
		if a.PotID == potID {
			return a.PotID, a.BalanceCents, nil
		}
	}
	// Tell a pot of the ledger that is no account from one that does not exist.
	var exists bool
	err := q.QueryRowContext(ctx, `SELECT EXISTS (SELECT 1 FROM pot WHERE id = ? AND ledger_id = ?)`, potID, ledgerID).Scan(&exists)
	if err != nil {
		return "", 0, err
	}
	if !exists {
		return "", 0, ErrPotNotFound
	}
	return "", 0, ErrNotAnAccount
}

// bookCorrection books the difference into the account's pot on the day of
// the reconciliation. Archived pots take no new bookings (rule T5).
func bookCorrection(ctx context.Context, tx *sql.Tx, in ReconcileInput, potID string, cents int64, now time.Time) (string, error) {
	if err := ensurePotActive(ctx, tx, in.LedgerID, potID); err != nil {
		return "", err
	}
	if cents > maxBalanceCents || cents < -maxBalanceCents {
		// Larger than one booking may be (X11); the user has to book part
		// of it by hand.
		return "", ErrInvalidBalance
	}
	return insertTransaction(ctx, tx, NewTransaction{
		LedgerID:    in.LedgerID,
		UserID:      in.UserID,
		Type:        TypeCorrection,
		PotID:       potID,
		AmountCents: cents, // stored with its sign: only expenses are negated
		BookedOn:    in.Today,
	}, now)
}

// saveReconciliation keeps only the last reconciliation of an account.
func saveReconciliation(ctx context.Context, tx *sql.Tx, ledgerID, potID, day string, actual int64) error {
	_, err := tx.ExecContext(ctx, `
		INSERT INTO reconciliation (pot_id, ledger_id, reconciled_on, actual_cents)
		VALUES (?, ?, ?, ?)
		ON CONFLICT (pot_id) DO UPDATE
		SET reconciled_on = excluded.reconciled_on, actual_cents = excluded.actual_cents`,
		potID, ledgerID, day, actual)
	return err
}

// lastReconciliations returns the last reconciliation of every account of a
// ledger, keyed by pot ID.
func lastReconciliations(ctx context.Context, q queryer, ledgerID string) (map[string]*Reconciliation, error) {
	rows, err := q.QueryContext(ctx, `
		SELECT pot_id, reconciled_on, actual_cents FROM reconciliation WHERE ledger_id = ?`, ledgerID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := map[string]*Reconciliation{}
	for rows.Next() {
		var potID string
		r := &Reconciliation{}
		if err := rows.Scan(&potID, &r.ReconciledOn, &r.ActualCents); err != nil {
			return nil, err
		}
		out[potID] = r
	}
	return out, rows.Err()
}
