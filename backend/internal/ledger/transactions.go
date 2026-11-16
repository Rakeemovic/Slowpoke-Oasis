// Package ledger contains the business rules for ledgers, pots and transactions.
package ledger

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"time"
	"unicode/utf8"
)

// TxType is the kind of a transaction (rules X2–X4).
type TxType string

const (
	TypeExpense  TxType = "expense"
	TypeIncome   TxType = "income"
	TypeTransfer TxType = "transfer"
)

// Validation errors. Callers can compare them with errors.Is.
var (
	ErrInvalidType          = errors.New("invalid transaction type")
	ErrAmountNotPositive    = errors.New("amount must be greater than zero")
	ErrInvalidDate          = errors.New("booked_on must be a date in the format YYYY-MM-DD")
	ErrCategoryRequired     = errors.New("expenses and income need a category")
	ErrCategoryForbidden    = errors.New("transfers cannot have a category")
	ErrCounterpartRequired  = errors.New("transfers need a counterpart pot")
	ErrCounterpartForbidden = errors.New("only transfers can have a counterpart pot")
	ErrSamePot              = errors.New("transfer source and target pot must differ")
	ErrNoteTooLong          = errors.New("note must have at most 500 characters")
)

// NewTransaction is the input for CreateTransaction. AmountCents is always
// positive; the sign is derived from Type (rule X2).
type NewTransaction struct {
	LedgerID         string
	UserID           string
	Type             TxType
	CategoryID       string // required for expense and income, empty for transfers
	PotID            string // empty means the ledger's default pot (rule X5)
	CounterpartPotID string // required for transfers, empty otherwise
	AmountCents      int64
	BookedOn         string // YYYY-MM-DD
	Note             string
}

// Validate checks the rules that do not need the database (X1–X4, X8, X9).
func (in NewTransaction) Validate() error {
	switch in.Type {
	case TypeExpense, TypeIncome:
		if in.CategoryID == "" {
			return ErrCategoryRequired
		}
		if in.CounterpartPotID != "" {
			return ErrCounterpartForbidden
		}
	case TypeTransfer:
		if in.CategoryID != "" {
			return ErrCategoryForbidden
		}
		if in.CounterpartPotID == "" {
			return ErrCounterpartRequired
		}
		if in.CounterpartPotID == in.PotID {
			return ErrSamePot
		}
	default:
		return ErrInvalidType
	}

	if in.AmountCents <= 0 {
		return ErrAmountNotPositive
	}
	if _, err := time.Parse("2006-01-02", in.BookedOn); err != nil {
		return ErrInvalidDate
	}
	if utf8.RuneCountInString(in.Note) > 500 {
		return ErrNoteTooLong
	}
	return nil
}

// Store persists ledger data.
type Store struct {
	db *sql.DB
}

// New returns a Store backed by db.
func New(db *sql.DB) *Store {
	return &Store{db: db}
}

// CreateTransaction validates the input, resolves the default pot if none is
// given, and inserts the transaction. It returns the new transaction ID.
//
// Rules that need the database (foreign keys, CHECK constraints) are enforced
// by the schema, so an error from the insert is returned as-is.
func (s *Store) CreateTransaction(ctx context.Context, in NewTransaction, now time.Time) (string, error) {
	if err := in.Validate(); err != nil {
		return "", err
	}

	potID := in.PotID
	if potID == "" {
		err := s.db.QueryRowContext(ctx,
			`SELECT id FROM pot WHERE ledger_id = ? AND is_default = 1`, in.LedgerID,
		).Scan(&potID)
		if err != nil {
			return "", fmt.Errorf("resolve default pot: %w", err)
		}
	}

	amount := in.AmountCents
	if in.Type == TypeExpense {
		amount = -amount
	}

	id, err := newID()
	if err != nil {
		return "", err
	}

	ts := now.UTC().Format(time.RFC3339)
	_, err = s.db.ExecContext(ctx, `
		INSERT INTO transactions
			(id, ledger_id, user_id, type, category_id, pot_id, counterpart_pot_id,
			 amount_cents, booked_on, note, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		id, in.LedgerID, in.UserID, string(in.Type),
		nullIfEmpty(in.CategoryID), potID, nullIfEmpty(in.CounterpartPotID),
		amount, in.BookedOn, nullIfEmpty(in.Note), ts, ts,
	)
	if err != nil {
		return "", err
	}
	return id, nil
}

// nullIfEmpty maps an empty string to SQL NULL.
func nullIfEmpty(s string) any {
	if s == "" {
		return nil
	}
	return s
}

// newID returns a random 128-bit identifier as hex.
func newID() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}
