package ledger

import (
	"context"
	"time"
)

// retentionDays is how long soft-deleted transactions stay in the database (rule X10).
const retentionDays = 30

// retentionCutoff returns the timestamp before which soft-deleted transactions
// are no longer kept.
func retentionCutoff(now time.Time) string {
	return timestamp(now.AddDate(0, 0, -retentionDays))
}

// DeletedTransaction is a soft-deleted transaction that can still be restored.
// The household sees who booked and who deleted it (rules X13, X18).
type DeletedTransaction struct {
	Transaction
	DeletedAt string
	DeletedBy string
	// DeletedByUsername is empty for the former-member placeholder.
	DeletedByUsername string
}

// ListDeletedTransactions returns the transactions of a ledger that were
// soft-deleted within the retention period, most recent deletion first.
// Limit and offset page through the result; a limit of 0 returns everything.
func (s *Store) ListDeletedTransactions(ctx context.Context, ledgerID string, now time.Time, limit, offset int) ([]DeletedTransaction, error) {
	if ledgerID == "" {
		return nil, ErrLedgerRequired
	}
	pc, pargs := pageClause(limit, offset)
	args := append([]any{ledgerID, retentionCutoff(now)}, pargs...)
	// The id breaks ties between deletions at the same moment, so pages keep
	// a stable order and no entry shows up twice or not at all.
	rows, err := s.db.QueryContext(ctx, `
		SELECT `+transactionColumns+`,
		       t.deleted_at, COALESCE(t.deleted_by, ''),
		       COALESCE(deleter.username, '')
		FROM `+transactionTables+`
		LEFT JOIN user deleter ON deleter.id = t.deleted_by
		WHERE t.ledger_id = ? AND t.deleted_at IS NOT NULL AND t.deleted_at >= ?
		ORDER BY t.deleted_at DESC, t.id`+pc,
		args...,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []DeletedTransaction
	for rows.Next() {
		var d DeletedTransaction
		targets := append(d.scanTargets(), &d.DeletedAt, &d.DeletedBy, &d.DeletedByUsername)
		if err := rows.Scan(targets...); err != nil {
			return nil, err
		}
		out = append(out, d)
	}
	return out, rows.Err()
}

// RestoreTransaction undoes a soft delete. It only works within the retention
// period, because older deletions may already have been purged.
func (s *Store) RestoreTransaction(ctx context.Context, ledgerID, id string, now time.Time) error {
	res, err := s.db.ExecContext(ctx, `
		UPDATE transactions
		SET deleted_at = NULL, deleted_by = NULL, updated_at = ?
		WHERE deleted_at IS NOT NULL AND deleted_at >= ? AND `+selfAndLinkedLeg,
		timestamp(now), retentionCutoff(now), id, ledgerID, id, ledgerID,
	)
	if err != nil {
		return err
	}
	return requireAffected(res, ErrTransactionNotFound)
}
