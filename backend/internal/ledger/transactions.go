package ledger

import (
	"context"
	"database/sql"
	"errors"
	"time"
	"unicode/utf8"

	"github.com/Rakeemovic/slowpoke-oasis/backend/internal/store"
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
	// RecurringPaymentID and RecurringDueOn name the occurrence of a
	// recurring payment that this expense pays (rule R6). Both or neither.
	RecurringPaymentID string
	RecurringDueOn     string
}

// Validate checks the rules that do not need the database (X1–X4, X8, X9,
// X20, R6).
func (in NewTransaction) Validate() error {
	if in.IsSpontaneous && in.Type != TypeExpense {
		return ErrSpontaneousExpense
	}
	if err := in.validateRecurringLink(); err != nil {
		return err
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

// validateRecurringLink checks the link to a recurring payment (rule R6).
func (in NewTransaction) validateRecurringLink() error {
	if (in.RecurringPaymentID == "") != (in.RecurringDueOn == "") {
		return ErrRecurringLinkRequired
	}
	if in.RecurringPaymentID == "" {
		return nil
	}
	if in.Type != TypeExpense {
		return ErrRecurringNeedsExpense
	}
	if !validDate(in.RecurringDueOn) {
		return ErrInvalidDate
	}
	return nil
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
	id, _, err := s.createTransaction(ctx, in, false, now)
	return id, err
}

// CreateTransactionWithTopUp books an expense from the reserve of a recurring
// payment like CreateTransaction, but first moves what the reserve lacks from
// the default pot into it, in the same database transaction (rule R13). It
// returns the new ID and the amount moved (0 if the reserve held enough).
func (s *Store) CreateTransactionWithTopUp(ctx context.Context, in NewTransaction, now time.Time) (string, int64, error) {
	return s.createTransaction(ctx, in, true, now)
}

func (s *Store) createTransaction(ctx context.Context, in NewTransaction, topUp bool, now time.Time) (string, int64, error) {
	if err := in.Validate(); err != nil {
		return "", 0, err
	}
	var id string
	var toppedUp int64
	err := s.inTx(ctx, func(tx *sql.Tx) error {
		var err error
		if in.PotID, err = resolvePot(ctx, tx, in.LedgerID, in.PotID); err != nil {
			return err
		}
		if err := checkNewTransaction(ctx, tx, in); err != nil {
			return err
		}
		if topUp {
			if toppedUp, err = topUpReserve(ctx, tx, in, in.PotID, now); err != nil {
				return err
			}
		}
		guard, err := guardReserves(ctx, tx, in.PotID)
		if err != nil {
			return err
		}
		if id, err = insertTransaction(ctx, tx, in, now); err != nil {
			return err
		}
		return guard.check(ctx, tx)
	})
	if err != nil {
		return "", 0, err
	}
	return id, toppedUp, nil
}

// checkNewTransaction checks the rules of a new transaction that need the
// database: active pots (T5), a fitting category (K3, K8) and an open
// occurrence of a recurring payment (R6). in.PotID must be resolved.
func checkNewTransaction(ctx context.Context, q queryer, in NewTransaction) error {
	if err := ensurePotActive(ctx, q, in.LedgerID, in.PotID); err != nil {
		return err
	}
	if in.CounterpartPotID != "" {
		if err := ensurePotActive(ctx, q, in.LedgerID, in.CounterpartPotID); err != nil {
			return err
		}
	}
	if in.Type != TypeTransfer {
		if err := ensureCategoryFits(ctx, q, in.LedgerID, in.CategoryID, in.Type, true); err != nil {
			return err
		}
	}
	if in.RecurringPaymentID != "" {
		return ensureOccurrenceOpen(ctx, q, in.LedgerID, in.RecurringPaymentID, in.RecurringDueOn)
	}
	return nil
}

// insertTransaction stores a checked transaction whose pot is resolved and
// returns its new ID.
func insertTransaction(ctx context.Context, tx *sql.Tx, in NewTransaction, now time.Time) (string, error) {
	id := newID()
	ts := timestamp(now)
	_, err := tx.ExecContext(ctx, `
		INSERT INTO transactions
			(id, ledger_id, user_id, type, category_id, pot_id, counterpart_pot_id,
			 amount_cents, booked_on, note, is_spontaneous,
			 recurring_payment_id, recurring_due_on, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		id, in.LedgerID, in.UserID, in.Type,
		nullIfEmpty(in.CategoryID), in.PotID, nullIfEmpty(in.CounterpartPotID),
		in.storedAmount(), in.BookedOn, nullIfEmpty(in.Note), in.IsSpontaneous,
		nullIfEmpty(in.RecurringPaymentID), nullIfEmpty(in.RecurringDueOn), ts, ts,
	)
	if store.IsUniqueViolation(err) {
		// Another booking for the same occurrence came first (rule R6).
		return "", ErrOccurrenceBooked
	}
	if err != nil {
		return "", err
	}
	return id, nil
}
