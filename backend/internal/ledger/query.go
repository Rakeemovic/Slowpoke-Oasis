package ledger

import (
	"context"
	"errors"
	"time"
)

// Errors for reading and deleting transactions.
var (
	ErrLedgerRequired      = errors.New("a ledger ID is required")
	ErrTransactionNotFound = errors.New("transaction not found")
)

// Transaction is a stored transaction as returned to callers.
// Optional values are empty strings when not set.
type Transaction struct {
	ID               string
	LedgerID         string
	UserID           string
	Type             TxType
	CategoryID       string
	PotID            string
	CounterpartPotID string
	AmountCents      int64
	BookedOn         string
	Note             string
}

// ListFilter selects transactions of one ledger. Empty optional fields mean
// "no restriction". From and To are inclusive and use the format YYYY-MM-DD.
type ListFilter struct {
	LedgerID string
	PotID    string
	From     string
	To       string
}

// ListTransactions returns the non-deleted transactions of a ledger that match
// the filter, newest booking date first.
func (s *Store) ListTransactions(ctx context.Context, f ListFilter) ([]Transaction, error) {
	if f.LedgerID == "" {
		return nil, ErrLedgerRequired
	}
	for _, d := range []string{f.From, f.To} {
		if d == "" {
			continue
		}
		if _, err := time.Parse("2006-01-02", d); err != nil {
			return nil, ErrInvalidDate
		}
	}

	query := `
		SELECT id, ledger_id, user_id, type,
		       COALESCE(category_id, ''), pot_id, COALESCE(counterpart_pot_id, ''),
		       amount_cents, booked_on, COALESCE(note, '')
		FROM transactions
		WHERE ledger_id = ? AND deleted_at IS NULL`
	args := []any{f.LedgerID}

	if f.PotID != "" {
		query += ` AND pot_id = ?`
		args = append(args, f.PotID)
	}
	if f.From != "" {
		query += ` AND booked_on >= ?`
		args = append(args, f.From)
	}
	if f.To != "" {
		query += ` AND booked_on <= ?`
		args = append(args, f.To)
	}
	query += ` ORDER BY booked_on DESC, created_at DESC`

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []Transaction
	for rows.Next() {
		var t Transaction
		var typ string
		if err := rows.Scan(&t.ID, &t.LedgerID, &t.UserID, &typ, &t.CategoryID,
			&t.PotID, &t.CounterpartPotID, &t.AmountCents, &t.BookedOn, &t.Note); err != nil {
			return nil, err
		}
		t.Type = TxType(typ)
		out = append(out, t)
	}
	return out, rows.Err()
}

// DeleteTransaction soft-deletes a transaction (rule X10): it sets deleted_at
// and deleted_by and keeps the row until the cleanup job removes it after 30 days.
func (s *Store) DeleteTransaction(ctx context.Context, ledgerID, id, userID string, now time.Time) error {
	ts := now.UTC().Format(time.RFC3339)
	res, err := s.db.ExecContext(ctx, `
		UPDATE transactions
		SET deleted_at = ?, deleted_by = ?, updated_at = ?
		WHERE deleted_at IS NULL AND (
		    (id = ? AND ledger_id = ?)
		    OR id IN (SELECT linked_transaction_id FROM transactions
		              WHERE id = ? AND ledger_id = ? AND linked_transaction_id IS NOT NULL)
		)`,
		ts, userID, ts, id, ledgerID, id, ledgerID,
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
