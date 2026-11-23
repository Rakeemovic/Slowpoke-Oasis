package ledger

import (
	"context"
	"database/sql"
	"errors"
	"time"
)

// TransactionPatch changes some fields of a transaction. Nil fields stay unchanged.
type TransactionPatch struct {
	Type             *TxType
	CategoryID       *string
	PotID            *string
	CounterpartPotID *string
	AmountCents      *int64
	BookedOn         *string
	Note             *string
	IsSpontaneous    *bool
	// CategoryScope says which bookings of the same recurring payment take
	// the new category too (rule R9). Empty means only this booking.
	CategoryScope CategoryScope
	// EditorID is the user who makes the change. Only needed with a
	// CategoryScope: a member changes only their own other bookings.
	EditorID string
}

// apply returns in with the fields of the patch that are set.
func (p TransactionPatch) apply(in NewTransaction) NewTransaction {
	if p.Type != nil {
		in.Type = *p.Type
	}
	if p.CategoryID != nil {
		in.CategoryID = *p.CategoryID
	}
	if p.PotID != nil {
		in.PotID = *p.PotID
	}
	if p.CounterpartPotID != nil {
		in.CounterpartPotID = *p.CounterpartPotID
	}
	if p.AmountCents != nil {
		in.AmountCents = *p.AmountCents
	}
	if p.BookedOn != nil {
		in.BookedOn = *p.BookedOn
	}
	if p.Note != nil {
		in.Note = *p.Note
	}
	if p.IsSpontaneous != nil {
		in.IsSpontaneous = *p.IsSpontaneous
	} else if in.Type != TypeExpense {
		// Turning a spontaneous expense into income or a transfer drops the
		// mark, which only expenses can have (rule X20).
		in.IsSpontaneous = false
	}
	return in
}

// UpdateTransaction applies a patch to a transaction that is not soft-deleted.
//
// The result is checked with the same rules as a new transaction. The author
// stays the same. Archived pots reject a transaction only when it moves into
// them (rule T5), so existing bookings can still be corrected.
//
// Reading, checking and writing happen in one database transaction, so two
// changes to the same booking cannot overwrite each other half-way.
func (s *Store) UpdateTransaction(ctx context.Context, ledgerID, id string, p TransactionPatch, now time.Time) error {
	return s.inTx(ctx, func(tx *sql.Tx) error {
		// The stored amount is signed; the input form is always positive.
		old := NewTransaction{LedgerID: ledgerID}
		err := tx.QueryRowContext(ctx, `
			SELECT user_id, type, COALESCE(category_id, ''), pot_id, COALESCE(counterpart_pot_id, ''),
			       abs(amount_cents), booked_on, COALESCE(note, ''), is_spontaneous,
			       COALESCE(recurring_payment_id, ''), COALESCE(recurring_due_on, '')
			FROM transactions
			WHERE id = ? AND ledger_id = ? AND deleted_at IS NULL`,
			id, ledgerID,
		).Scan(&old.UserID, &old.Type, &old.CategoryID, &old.PotID, &old.CounterpartPotID,
			&old.AmountCents, &old.BookedOn, &old.Note, &old.IsSpontaneous,
			&old.RecurringPaymentID, &old.RecurringDueOn)
		if errors.Is(err, sql.ErrNoRows) {
			return ErrTransactionNotFound
		}
		if err != nil {
			return err
		}
		if old.Type == TypeTransferOut || old.Type == TypeTransferIn {
			return ErrLinkedTransaction
		}

		// The link to a recurring payment stays (rule R6), so a booking that
		// pays an occurrence cannot become income or a transfer.
		if !p.CategoryScope.valid() {
			return ErrInvalidCategoryScope
		}
		in := p.apply(old)
		if err := in.Validate(); err != nil {
			return err
		}
		if in.PotID, err = resolvePot(ctx, tx, ledgerID, in.PotID); err != nil {
			return err
		}

		if in.PotID != old.PotID {
			if err := ensurePotActive(ctx, tx, ledgerID, in.PotID); err != nil {
				return err
			}
		}
		if in.CounterpartPotID != "" && in.CounterpartPotID != old.CounterpartPotID {
			if err := ensurePotActive(ctx, tx, ledgerID, in.CounterpartPotID); err != nil {
				return err
			}
		}
		if in.Type != TypeTransfer && (in.Type != old.Type || in.CategoryID != old.CategoryID) {
			newlyChosen := in.CategoryID != old.CategoryID
			if err := ensureCategoryFits(ctx, tx, ledgerID, in.CategoryID, in.Type, newlyChosen); err != nil {
				return err
			}
		}

		guard, err := guardReserves(ctx, tx, old.PotID, old.CounterpartPotID, in.PotID, in.CounterpartPotID)
		if err != nil {
			return err
		}
		_, err = tx.ExecContext(ctx, `
			UPDATE transactions
			SET type = ?, category_id = ?, pot_id = ?, counterpart_pot_id = ?,
			    amount_cents = ?, booked_on = ?, note = ?, is_spontaneous = ?, updated_at = ?
			WHERE id = ? AND ledger_id = ?`,
			in.Type, nullIfEmpty(in.CategoryID), in.PotID, nullIfEmpty(in.CounterpartPotID),
			in.storedAmount(), in.BookedOn, nullIfEmpty(in.Note), in.IsSpontaneous, timestamp(now),
			id, ledgerID,
		)
		if err != nil {
			return err
		}
		if err := guard.check(ctx, tx); err != nil {
			return err
		}
		if !p.CategoryScope.series() {
			return nil
		}
		return applyCategoryToSeries(ctx, tx, seriesChange{
			ledgerID: ledgerID, bookingID: id, paymentID: old.RecurringPaymentID, dueOn: old.RecurringDueOn,
			categoryID: in.CategoryID, categoryGiven: p.CategoryID != nil,
			scope: p.CategoryScope, editorID: p.EditorID, now: now,
		})
	})
}
