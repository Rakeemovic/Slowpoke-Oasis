package ledger

import (
	"context"
	"database/sql"
	"log"
	"time"
)

// retentionDays is how long soft-deleted transactions stay in the database (rule X10).
const retentionDays = 30

// retentionCutoff returns the timestamp before which soft-deleted transactions
// are no longer kept. It uses the same RFC3339 format as the stored timestamps.
func retentionCutoff(now time.Time) string {
	return now.UTC().AddDate(0, 0, -retentionDays).Format(time.RFC3339)
}

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

// RunPurgeLoop purges once right away and then at every interval until ctx is cancelled.
func (s *Store) RunPurgeLoop(ctx context.Context, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		n, err := s.PurgeDeletedTransactions(ctx, time.Now())
		switch {
		case err != nil:
			log.Printf("purge deleted transactions: %v", err)
		case n > 0:
			log.Printf("purged %d deleted transactions", n)
		}

		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

// DeletePersonalLedgers permanently removes all personal ledgers of a user,
// including their categories, pots and transactions. It is used when an
// account is deleted.
func (s *Store) DeletePersonalLedgers(ctx context.Context, userID string) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback() // no effect after a successful Commit

	if err := DeletePersonalLedgersTx(ctx, tx, userID); err != nil {
		return err
	}
	return tx.Commit()
}

// DeletePersonalLedgersTx is like DeletePersonalLedgers but runs inside an
// existing transaction, so it can be combined with other deletions.
func DeletePersonalLedgersTx(ctx context.Context, tx *sql.Tx, userID string) error {
	ids, err := queryIDs(ctx, tx, `SELECT id FROM ledger WHERE user_id = ?`, userID)
	if err != nil {
		return err
	}
	return deleteLedgersTx(ctx, tx, ids)
}

// queryIDs runs a query that returns one ID column and collects the values.
func queryIDs(ctx context.Context, tx *sql.Tx, query string, args ...any) ([]string, error) {
	rows, err := tx.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

// deleteLedgersTx removes the ledgers with the given IDs and everything in them.
func deleteLedgersTx(ctx context.Context, tx *sql.Tx, ledgerIDs []string) error {
	for _, id := range ledgerIDs {
		statements := []string{
			`DELETE FROM transactions WHERE ledger_id = ?`,
			`DELETE FROM pot_line WHERE ledger_id = ?`,
			// The default pot is protected from deletion by a trigger, so clear its flag first.
			`UPDATE pot SET is_default = 0 WHERE ledger_id = ?`,
			`DELETE FROM pot WHERE ledger_id = ?`,
			`DELETE FROM category WHERE ledger_id = ?`,
			`DELETE FROM ledger WHERE id = ?`,
		}
		for _, q := range statements {
			if _, err := tx.ExecContext(ctx, q, id); err != nil {
				return err
			}
		}
	}
	return nil
}
