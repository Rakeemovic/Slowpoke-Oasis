// Package ledger contains the business rules for ledgers, pots and transactions.
package ledger

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"time"
)

// dateLayout is the format of all calendar dates: YYYY-MM-DD.
const dateLayout = "2006-01-02"

// Store persists ledger data.
type Store struct {
	db *sql.DB
}

// New returns a Store backed by db.
func New(db *sql.DB) *Store {
	return &Store{db: db}
}

// queryer is the read side of *sql.DB and *sql.Tx, so the same check can run
// on its own or as part of a transaction.
type queryer interface {
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}

// inTx runs fn in a database transaction. The transaction is committed if fn
// returns nil and rolled back otherwise.
func (s *Store) inTx(ctx context.Context, fn func(tx *sql.Tx) error) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback() // no effect after a successful Commit

	if err := fn(tx); err != nil {
		return err
	}
	return tx.Commit()
}

// requireAffected returns notFound if a statement changed no rows.
func requireAffected(res sql.Result, notFound error) error {
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return notFound
	}
	return nil
}

// queryIDs runs a query that returns one ID column and collects the values.
func queryIDs(ctx context.Context, q queryer, query string, args ...any) ([]string, error) {
	rows, err := q.QueryContext(ctx, query, args...)
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

// newID returns a random 128-bit identifier as hex.
func newID() string {
	b := make([]byte, 16)
	rand.Read(b) // never fails: crypto/rand aborts the program instead
	return hex.EncodeToString(b)
}

// timestamp formats a point in time the way it is stored: RFC 3339 in UTC.
// All stored timestamps share this format, so string order is time order.
func timestamp(t time.Time) string {
	return t.UTC().Format(time.RFC3339)
}

// validDate reports whether s is a calendar date in the format YYYY-MM-DD.
func validDate(s string) bool {
	_, err := time.Parse(dateLayout, s)
	return err == nil
}

// nullIfEmpty maps an empty string to SQL NULL.
func nullIfEmpty(s string) any {
	if s == "" {
		return nil
	}
	return s
}

// nullIfZero maps 0 to SQL NULL, used for optional amounts.
func nullIfZero(v int64) any {
	if v == 0 {
		return nil
	}
	return v
}
