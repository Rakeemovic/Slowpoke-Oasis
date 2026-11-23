package ledger

import (
	"context"
	"errors"
	"time"
	"unicode/utf8"
)

// TxType is the kind of a transaction (rules X2–X4).
type TxType string

const (
	TypeExpense  TxType = "expense"
	TypeIncome   TxType = "income"
	TypeTransfer TxType = "transfer"
	// The two legs of a transfer between ledgers (rule X14). They are created
	// by CreateLinkedTransfer only and cannot be entered directly.
	TypeTransferOut TxType = "transfer_out"
	TypeTransferIn  TxType = "transfer_in"
)

// maxNoteLength is the longest note a transaction can have (rule X9).
const maxNoteLength = 500

// Validation errors. Callers can compare them with errors.Is.
var (
	ErrInvalidType          = errors.New("invalid transaction type")
	ErrAmountNotPositive    = errors.New("amount must be greater than zero")
	ErrInvalidDate          = errors.New("dates must have the format YYYY-MM-DD")
	ErrCategoryRequired     = errors.New("expenses and income need a category")
	ErrCategoryForbidden    = errors.New("transfers cannot have a category")
	ErrCounterpartRequired  = errors.New("transfers need a counterpart pot")
	ErrCounterpartForbidden = errors.New("only transfers can have a counterpart pot")
	ErrSamePot              = errors.New("transfer source and target pot must differ")
	ErrNoteTooLong          = errors.New("note must have at most 500 characters")
	ErrSpontaneousExpense   = errors.New("only expenses can be marked as spontaneous")
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
	// IsSpontaneous marks an unplanned expense (rule X20).
	IsSpontaneous bool
}

// Validate checks the rules that do not need the database (X1–X4, X8, X9,
// X20).
func (in NewTransaction) Validate() error {
	if in.IsSpontaneous && in.Type != TypeExpense {
		return ErrSpontaneousExpense
	}
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
	return validateBooking(in.AmountCents, in.BookedOn, in.Note)
}

// validateBooking checks the rules that every booking shares, whatever its
// type: a positive amount (X1), a valid date (X8) and a short note (X9).
func validateBooking(amountCents int64, bookedOn, note string) error {
	if amountCents <= 0 {
		return ErrAmountNotPositive
	}
	if !validDate(bookedOn) {
		return ErrInvalidDate
	}
	if utf8.RuneCountInString(note) > maxNoteLength {
		return ErrNoteTooLong
	}
	return nil
}

// storedAmount returns the amount with the sign the database expects:
// negative for expenses, positive otherwise (rule X2).
func (in NewTransaction) storedAmount() int64 {
	if in.Type == TypeExpense {
		return -in.AmountCents
	}
	return in.AmountCents
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

	potID, err := resolvePot(ctx, s.db, in.LedgerID, in.PotID)
	if err != nil {
		return "", err
	}
	if err := ensurePotActive(ctx, s.db, in.LedgerID, potID); err != nil {
		return "", err
	}
	if in.CounterpartPotID != "" {
		if err := ensurePotActive(ctx, s.db, in.LedgerID, in.CounterpartPotID); err != nil {
			return "", err
		}
	}
	if in.Type != TypeTransfer {
		if err := ensureCategoryFits(ctx, s.db, in.LedgerID, in.CategoryID, in.Type, true); err != nil {
			return "", err
		}
	}

	id := newID()
	ts := timestamp(now)
	_, err = s.db.ExecContext(ctx, `
		INSERT INTO transactions
			(id, ledger_id, user_id, type, category_id, pot_id, counterpart_pot_id,
			 amount_cents, booked_on, note, is_spontaneous, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		id, in.LedgerID, in.UserID, in.Type,
		nullIfEmpty(in.CategoryID), potID, nullIfEmpty(in.CounterpartPotID),
		in.storedAmount(), in.BookedOn, nullIfEmpty(in.Note), in.IsSpontaneous, ts, ts,
	)
	if err != nil {
		return "", err
	}
	return id, nil
}
