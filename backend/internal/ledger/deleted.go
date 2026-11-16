package ledger

import (
	"context"
	"time"
)

// DeletedTransaction is a soft-deleted transaction that can still be restored.
// The names are shown to the household, so everyone can see who booked and who
// deleted the transaction (rule X13). Departed members appear as the placeholder.
type DeletedTransaction struct {
	Transaction
	DeletedAt     string
	DeletedBy     string
	BookedByName  string
	DeletedByName string
}

// ListDeletedTransactions returns the transactions of a ledger that were
// soft-deleted within the retention period, most recent deletion first.
func (s *Store) ListDeletedTransactions(ctx context.Context, ledgerID string, now time.Time) ([]DeletedTransaction, error) {
	if ledgerID == "" {
		return nil, ErrLedgerRequired
	}
	rows, err := s.db.QueryContext(ctx, `
		SELECT t.id, t.ledger_id, t.user_id, t.type,
		       COALESCE(t.category_id, ''), t.pot_id, COALESCE(t.counterpart_pot_id, ''),
		       t.amount_cents, t.booked_on, COALESCE(t.note, ''),
		       t.deleted_at, COALESCE(t.deleted_by, ''),
		       COALESCE(booker.name, ''), COALESCE(deleter.name, '')
		FROM transactions t
		LEFT JOIN user booker ON booker.id = t.user_id
		LEFT JOIN user deleter ON deleter.id = t.deleted_by
		WHERE t.ledger_id = ? AND t.deleted_at IS NOT NULL AND t.deleted_at >= ?
		ORDER BY t.deleted_at DESC`,
		ledgerID, retentionCutoff(now),
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []DeletedTransaction
	for rows.Next() {
		var d DeletedTransaction
		var typ string
		if err := rows.Scan(&d.ID, &d.LedgerID, &d.UserID, &typ, &d.CategoryID,
			&d.PotID, &d.CounterpartPotID, &d.AmountCents, &d.BookedOn, &d.Note,
			&d.DeletedAt, &d.DeletedBy, &d.BookedByName, &d.DeletedByName); err != nil {
			return nil, err
		}
		d.Type = TxType(typ)
		out = append(out, d)
	}
	return out, rows.Err()
}

// RestoreTransaction undoes a soft delete. It only works within the retention
// period, because older deletions may already have been purged.
func (s *Store) RestoreTransaction(ctx context.Context, ledgerID, id string, now time.Time) error {
	ts := now.UTC().Format(time.RFC3339)
	res, err := s.db.ExecContext(ctx, `
		UPDATE transactions
		SET deleted_at = NULL, deleted_by = NULL, updated_at = ?
		WHERE id = ? AND ledger_id = ?
		  AND deleted_at IS NOT NULL AND deleted_at >= ?`,
		ts, id, ledgerID, retentionCutoff(now),
	)
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return ErrTransactionNotFound
	}
	return nil
}
