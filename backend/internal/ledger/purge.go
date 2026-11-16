package ledger

import (
	"context"
	"database/sql"
	"time"
)

// PurgeDeletedTransactions permanently removes transactions that were
// soft-deleted more than retentionDays before now. It returns how many rows were removed.
func (s *Store) PurgeDeletedTransactions(ctx context.Context, now time.Time) (int64, error) {
	res, err := s.db.ExecContext(ctx,
		`DELETE FROM transactions WHERE deleted_at IS NOT NULL AND deleted_at < ?`, retentionCutoff(now),
	)
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}

// RemoveUserData permanently removes everything a user owns, as part of
// deleting the account in tx: the personal ledgers with their categories, pots
// and transactions, and the memberships in all households. Bookings in
// households stay there under the placeholder (rule X12).
func RemoveUserData(ctx context.Context, tx *sql.Tx, userID string) error {
	if err := deletePersonalLedgers(ctx, tx, userID); err != nil {
		return err
	}
	return leaveAllHouseholds(ctx, tx, userID)
}

// deletePersonalLedgers removes all personal ledgers of a user.
func deletePersonalLedgers(ctx context.Context, tx *sql.Tx, userID string) error {
	ids, err := queryIDs(ctx, tx, `SELECT id FROM ledger WHERE user_id = ?`, userID)
	if err != nil {
		return err
	}
	return deleteLedgers(ctx, tx, ids)
}

// deleteLedgers removes the ledgers with the given IDs and everything in them.
// The other leg of a transfer into or out of such a ledger stays in its own
// ledger and loses its link (rule X16).
func deleteLedgers(ctx context.Context, tx *sql.Tx, ledgerIDs []string) error {
	statements := []string{
		`DELETE FROM transactions WHERE ledger_id = ?`,
		`DELETE FROM pot_line WHERE ledger_id = ?`,
		// The default pot is protected from deletion by a trigger, so clear its flag first.
		`UPDATE pot SET is_default = 0 WHERE ledger_id = ?`,
		`DELETE FROM pot WHERE ledger_id = ?`,
		`DELETE FROM category WHERE ledger_id = ?`,
		`DELETE FROM ledger WHERE id = ?`,
	}
	for _, id := range ledgerIDs {
		for _, q := range statements {
			if _, err := tx.ExecContext(ctx, q, id); err != nil {
				return err
			}
		}
	}
	return nil
}
