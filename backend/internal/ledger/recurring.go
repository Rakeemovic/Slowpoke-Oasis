package ledger

import (
	"context"
	"database/sql"
	"errors"
	"time"
)

// Errors for recurring payments.
var (
	ErrRecurringNotFound = errors.New("recurring payment not found")
	ErrRecurringArchived = errors.New("this recurring payment is archived")
	// ErrRecurringNeedsExpense guards rules R2 and R6.
	ErrRecurringNeedsExpense = errors.New("recurring payments and the bookings that pay them are expenses")
	ErrRecurringAmount       = errors.New("amount_cents must be greater than zero and at most 100,000,000.00")
)

// maxRecurringAmount is the highest amount of a recurring payment in cents,
// the same limit as for a booking (rule X11).
const maxRecurringAmount = 10_000_000_000

// NewRecurringPayment is the input for CreateRecurringPayment.
type NewRecurringPayment struct {
	LedgerID    string
	Name        string
	AmountCents int64
	CategoryID  string
	// PotID is the pot to book from; empty means the ledger's default pot. A
	// payment due every few months books from its own reserve instead and
	// takes no pot (rule R10).
	PotID string
	Schedule
	// Today is the app's local date (YYYY-MM-DD). Occurrences due before it
	// are never open (rule R5).
	Today string
}

// RecurringPayment is a stored recurring payment.
type RecurringPayment struct {
	ID          string
	Name        string
	AmountCents int64
	CategoryID  string
	PotID       string
	Schedule
	TrackedFrom string
	Archived    bool
	// ReservePotID is the active reserve of the payment, or "" (rule R10). It
	// stays when the payment is archived or becomes monthly.
	ReservePotID string
}

// RecurringPaymentUpdate changes some fields of a recurring payment. Nil
// fields stay unchanged; an empty LastMonth removes the end.
type RecurringPaymentUpdate struct {
	Name           *string
	AmountCents    *int64
	CategoryID     *string
	PotID          *string
	IntervalMonths *int
	DueDay         *int
	FirstMonth     *string
	LastMonth      *string
	Archived       *bool
	// Today is the app's local date. It becomes the new start of tracking
	// when the due dates change or the payment is taken out of the archive.
	Today string
}

// validRecurringAmount checks the amount of a recurring payment (rule R1).
func validRecurringAmount(cents int64) error {
	if cents <= 0 || cents > maxRecurringAmount {
		return ErrRecurringAmount
	}
	return nil
}

// CreateRecurringPayment checks and stores a new recurring payment of an
// expense category (rules R1–R5). It returns the new ID.
func (s *Store) CreateRecurringPayment(ctx context.Context, in NewRecurringPayment, now time.Time) (string, error) {
	name, err := cleanName(in.Name)
	if err != nil {
		return "", err
	}
	if err := validRecurringAmount(in.AmountCents); err != nil {
		return "", err
	}
	if err := in.Schedule.validate(); err != nil {
		return "", err
	}
	if !validDate(in.Today) {
		return "", ErrInvalidDate
	}
	if in.PotID != "" && in.IntervalMonths > 1 {
		return "", ErrReservePotFixed
	}

	id := newID()
	err = s.inTx(ctx, func(tx *sql.Tx) error {
		potID, err := resolvePot(ctx, tx, in.LedgerID, in.PotID)
		if err != nil {
			return err
		}
		if err := ensurePotActive(ctx, tx, in.LedgerID, potID); err != nil {
			return err
		}
		if err := ensurePotChoosable(ctx, tx, in.LedgerID, potID, id); err != nil {
			return err
		}
		if err := ensureCategoryFits(ctx, tx, in.LedgerID, in.CategoryID, TypeExpense, true); err != nil {
			return recurringCategoryError(err)
		}
		_, err = tx.ExecContext(ctx, `
			INSERT INTO recurring_payment
				(id, ledger_id, name, amount_cents, category_id, pot_id,
				 interval_months, due_day, first_month, last_month, tracked_from, created_at)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			id, in.LedgerID, name, in.AmountCents, in.CategoryID, potID,
			in.IntervalMonths, in.DueDay, in.FirstMonth, nullIfEmpty(in.LastMonth), in.Today, timestamp(now),
		)
		if err != nil {
			return err
		}
		// The reserve refers to the payment, so it comes second (rule R10).
		p := RecurringPayment{ID: id, Name: name, AmountCents: in.AmountCents, PotID: potID, Schedule: in.Schedule}
		if err := syncReserve(ctx, tx, in.LedgerID, &p); err != nil {
			return err
		}
		return setPaymentPot(ctx, tx, id, potID, p.PotID)
	})
	if err != nil {
		return "", err
	}
	return id, nil
}

// recurringCategoryError reports an income category as ErrRecurringNeedsExpense,
// which explains the problem better than the general kind mismatch.
func recurringCategoryError(err error) error {
	if errors.Is(err, ErrCategoryKindMismatch) {
		return ErrRecurringNeedsExpense
	}
	return err
}

// recurringColumns lists the columns of a RecurringPayment, in the order of
// scanTargets.
const recurringColumns = `id, name, amount_cents, category_id, pot_id,
	interval_months, due_day, first_month, COALESCE(last_month, ''), tracked_from, archived,
	COALESCE((SELECT p.id FROM pot p
	          WHERE p.reserve_for = recurring_payment.id AND p.archived = 0), '')`

func (p *RecurringPayment) scanTargets() []any {
	return []any{&p.ID, &p.Name, &p.AmountCents, &p.CategoryID, &p.PotID,
		&p.IntervalMonths, &p.DueDay, &p.FirstMonth, &p.LastMonth, &p.TrackedFrom, &p.Archived,
		&p.ReservePotID}
}

// ListRecurringPayments returns all recurring payments of a ledger, archived
// ones included, ordered by name.
func (s *Store) ListRecurringPayments(ctx context.Context, ledgerID string) ([]RecurringPayment, error) {
	return queryRecurringPayments(ctx, s.db, `WHERE ledger_id = ? ORDER BY name COLLATE NOCASE, id`, ledgerID)
}

// queryRecurringPayments reads the recurring payments selected by the SQL
// that follows the table name.
func queryRecurringPayments(ctx context.Context, q queryer, where string, args ...any) ([]RecurringPayment, error) {
	rows, err := q.QueryContext(ctx, `SELECT `+recurringColumns+` FROM recurring_payment `+where, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := []RecurringPayment{}
	for rows.Next() {
		var p RecurringPayment
		if err := rows.Scan(p.scanTargets()...); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

// loadRecurringPayment reads one recurring payment of a ledger.
func loadRecurringPayment(ctx context.Context, q queryer, ledgerID, id string) (RecurringPayment, error) {
	var p RecurringPayment
	err := q.QueryRowContext(ctx,
		`SELECT `+recurringColumns+` FROM recurring_payment WHERE id = ? AND ledger_id = ?`, id, ledgerID,
	).Scan(p.scanTargets()...)
	if errors.Is(err, sql.ErrNoRows) {
		return p, ErrRecurringNotFound
	}
	return p, err
}

// UpdateRecurringPayment applies an update (rules R1–R5). Changing the due
// dates, or taking the payment out of the archive, starts tracking anew on
// u.Today: earlier occurrences do not become open (rule R5).
func (s *Store) UpdateRecurringPayment(ctx context.Context, ledgerID, id string, u RecurringPaymentUpdate) error {
	if !validDate(u.Today) {
		return ErrInvalidDate
	}
	return s.inTx(ctx, func(tx *sql.Tx) error {
		old, err := loadRecurringPayment(ctx, tx, ledgerID, id)
		if err != nil {
			return err
		}
		p, err := u.apply(old)
		if err != nil {
			return err
		}
		if p.savesInReserve() && p.PotID != old.PotID {
			return ErrReservePotFixed
		}
		if err := checkChangedReferences(ctx, tx, ledgerID, old, p); err != nil {
			return err
		}
		if err := syncReserve(ctx, tx, ledgerID, &p); err != nil {
			return err
		}
		if p.Name != old.Name {
			if err := renameReserve(ctx, tx, ledgerID, id, old.Name, p.Name); err != nil {
				return err
			}
		}
		if !p.Schedule.sameDates(old.Schedule) || (old.Archived && !p.Archived) {
			p.TrackedFrom = u.Today
		}
		_, err = tx.ExecContext(ctx, `
			UPDATE recurring_payment
			SET name = ?, amount_cents = ?, category_id = ?, pot_id = ?, interval_months = ?,
			    due_day = ?, first_month = ?, last_month = ?, tracked_from = ?, archived = ?
			WHERE id = ? AND ledger_id = ?`,
			p.Name, p.AmountCents, p.CategoryID, p.PotID, p.IntervalMonths,
			p.DueDay, p.FirstMonth, nullIfEmpty(p.LastMonth), p.TrackedFrom, p.Archived,
			id, ledgerID,
		)
		return err
	})
}

// apply returns p with the fields of the update that are set, checked.
func (u RecurringPaymentUpdate) apply(p RecurringPayment) (RecurringPayment, error) {
	var err error
	if u.Name != nil {
		if p.Name, err = cleanName(*u.Name); err != nil {
			return p, err
		}
	}
	if u.AmountCents != nil {
		if err := validRecurringAmount(*u.AmountCents); err != nil {
			return p, err
		}
		p.AmountCents = *u.AmountCents
	}
	setIfGiven(&p.CategoryID, u.CategoryID)
	setIfGiven(&p.PotID, u.PotID)
	setIfGiven(&p.IntervalMonths, u.IntervalMonths)
	setIfGiven(&p.DueDay, u.DueDay)
	setIfGiven(&p.FirstMonth, u.FirstMonth)
	setIfGiven(&p.LastMonth, u.LastMonth)
	setIfGiven(&p.Archived, u.Archived)
	return p, p.Schedule.validate()
}

// setIfGiven sets *dst to *src if src is not nil.
func setIfGiven[T any](dst *T, src *T) {
	if src != nil {
		*dst = *src
	}
}

// checkChangedReferences checks a newly chosen category (expense, not
// archived) and a newly chosen pot (active), as for a new payment.
func checkChangedReferences(ctx context.Context, tx *sql.Tx, ledgerID string, old, p RecurringPayment) error {
	if p.CategoryID != old.CategoryID {
		if err := ensureCategoryFits(ctx, tx, ledgerID, p.CategoryID, TypeExpense, true); err != nil {
			return recurringCategoryError(err)
		}
	}
	if p.PotID != old.PotID {
		if err := ensurePotActive(ctx, tx, ledgerID, p.PotID); err != nil {
			return err
		}
		return ensurePotChoosable(ctx, tx, ledgerID, p.PotID, p.ID)
	}
	return nil
}

// setPaymentPot stores the pot a payment books from, if it changed.
func setPaymentPot(ctx context.Context, tx *sql.Tx, paymentID, oldPotID, potID string) error {
	if potID == oldPotID {
		return nil
	}
	_, err := tx.ExecContext(ctx, `UPDATE recurring_payment SET pot_id = ? WHERE id = ?`, potID, paymentID)
	return err
}
