package ledger

import (
	"context"
	"database/sql"
	"errors"
	"time"
)

// Errors for reading and deleting transactions.
var (
	ErrLedgerRequired      = errors.New("a ledger ID is required")
	ErrLedgerNotFound      = errors.New("ledger not found")
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
	// Username of the author (rule X18). Empty for the former-member
	// placeholder, which has no username.
	Username string
	// IsSpontaneous marks an unplanned expense (rule X20).
	IsSpontaneous bool
	// RecurringPaymentID and RecurringDueOn name the occurrence of a
	// recurring payment that the expense pays (rule R6), or are empty.
	RecurringPaymentID string
	RecurringDueOn     string
}

// transactionColumns lists the columns of a Transaction for a query that
// reads FROM transactionTables. scanTargets returns the matching targets.
const transactionColumns = `
	t.id, t.ledger_id, t.user_id, t.type,
	COALESCE(t.category_id, ''), t.pot_id, COALESCE(t.counterpart_pot_id, ''),
	t.amount_cents, t.booked_on, COALESCE(t.note, ''), COALESCE(author.username, ''),
	t.is_spontaneous, COALESCE(t.recurring_payment_id, ''), COALESCE(t.recurring_due_on, '')`

// transactionTables names the transactions table t and joins its author.
const transactionTables = `transactions t LEFT JOIN user author ON author.id = t.user_id`

func (t *Transaction) scanTargets() []any {
	return []any{&t.ID, &t.LedgerID, &t.UserID, &t.Type, &t.CategoryID,
		&t.PotID, &t.CounterpartPotID, &t.AmountCents, &t.BookedOn, &t.Note, &t.Username,
		&t.IsSpontaneous, &t.RecurringPaymentID, &t.RecurringDueOn}
}

// ListFilter selects transactions of one ledger. Empty optional fields mean
// "no restriction". From and To are inclusive and use the format YYYY-MM-DD.
type ListFilter struct {
	LedgerID string
	PotID    string
	From     string
	To       string
	// Limit and Offset page through the result. A limit of 0 returns everything.
	Limit  int
	Offset int
}

// ListTransactions returns the non-deleted transactions of a ledger that match
// the filter, newest booking date first.
func (s *Store) ListTransactions(ctx context.Context, f ListFilter) ([]Transaction, error) {
	if f.LedgerID == "" {
		return nil, ErrLedgerRequired
	}
	if err := validateRange(f.From, f.To); err != nil {
		return nil, err
	}

	query := `SELECT ` + transactionColumns + `
		FROM ` + transactionTables + `
		WHERE t.ledger_id = ? AND t.deleted_at IS NULL`
	args := []any{f.LedgerID}

	if f.PotID != "" {
		query += ` AND t.pot_id = ?`
		args = append(args, f.PotID)
	}
	rc, rargs := rangeClause(f.From, f.To)
	query += rc + ` ORDER BY t.booked_on DESC, t.created_at DESC`
	args = append(args, rargs...)
	pc, pargs := pageClause(f.Limit, f.Offset)
	query += pc
	args = append(args, pargs...)

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []Transaction
	for rows.Next() {
		var t Transaction
		if err := rows.Scan(t.scanTargets()...); err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

// pageClause returns the LIMIT/OFFSET part of a list query and its arguments.
// A limit of 0 means "everything", so there is no clause.
func pageClause(limit, offset int) (string, []any) {
	if limit <= 0 {
		return "", nil
	}
	return ` LIMIT ? OFFSET ?`, []any{limit, offset}
}

// GetTransaction returns one non-deleted transaction of a ledger, or
// ErrTransactionNotFound.
func (s *Store) GetTransaction(ctx context.Context, ledgerID, id string) (Transaction, error) {
	var t Transaction
	err := s.db.QueryRowContext(ctx, `SELECT `+transactionColumns+`
		FROM `+transactionTables+`
		WHERE t.id = ? AND t.ledger_id = ? AND t.deleted_at IS NULL`,
		id, ledgerID,
	).Scan(t.scanTargets()...)
	if errors.Is(err, sql.ErrNoRows) {
		return Transaction{}, ErrTransactionNotFound
	}
	return t, err
}

// selfAndLinkedLeg is a WHERE condition that matches a transaction of a
// ledger and, for a transfer between ledgers, its other leg (rule X14), so
// both legs are always deleted and restored together. It takes the
// transaction ID and the ledger ID, twice each.
const selfAndLinkedLeg = `(
	(id = ? AND ledger_id = ?)
	OR id IN (SELECT linked_transaction_id FROM transactions
	          WHERE id = ? AND ledger_id = ? AND linked_transaction_id IS NOT NULL)
)`

// DeleteTransaction soft-deletes a transaction (rule X10): it sets deleted_at
// and deleted_by and keeps the row until the cleanup job removes it after 30 days.
// Deleting money that went into a reserve must not take it below zero (R12).
func (s *Store) DeleteTransaction(ctx context.Context, ledgerID, id, userID string, now time.Time) error {
	ts := timestamp(now)
	return s.changeGuarded(ctx, ledgerID, id, func(tx *sql.Tx) (sql.Result, error) {
		return tx.ExecContext(ctx, `
			UPDATE transactions
			SET deleted_at = ?, deleted_by = ?, updated_at = ?
			WHERE deleted_at IS NULL AND `+selfAndLinkedLeg,
			ts, userID, ts, id, ledgerID, id, ledgerID,
		)
	})
}

// changeGuarded runs an update of a transaction and its linked leg in a
// database transaction. It fails with ErrTransactionNotFound if no row
// changed, and with ErrReserveShort if a reserve falls below zero (R12).
func (s *Store) changeGuarded(ctx context.Context, ledgerID, id string, change func(tx *sql.Tx) (sql.Result, error)) error {
	return s.inTx(ctx, func(tx *sql.Tx) error {
		pots, err := transactionPots(ctx, tx, ledgerID, id)
		if err != nil {
			return err
		}
		guard, err := guardReserves(ctx, tx, pots...)
		if err != nil {
			return err
		}
		res, err := change(tx)
		if err != nil {
			return err
		}
		if err := requireAffected(res, ErrTransactionNotFound); err != nil {
			return err
		}
		return guard.check(ctx, tx)
	})
}
