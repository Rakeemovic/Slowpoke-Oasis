package ledger

import (
	"context"
	"database/sql"
	"errors"
	"time"
)

// ErrInvalidCategoryScope rejects a category scope that does not fit (rule R9).
var ErrInvalidCategoryScope = errors.New(`category_scope must be "this", "this_and_earlier", "this_and_later" or "all", ` +
	`and a scope other than "this" needs a category_id on a booking of a recurring payment`)

// CategoryScope says which bookings of a recurring payment get a new category
// when the category of one of them changes (rule R9).
type CategoryScope string

const (
	ScopeThis         CategoryScope = "this"
	ScopeThisEarlier  CategoryScope = "this_and_earlier"
	ScopeThisLater    CategoryScope = "this_and_later"
	ScopeWholeSeries  CategoryScope = "all"
	scopeNotSpecified CategoryScope = ""
)

func (sc CategoryScope) valid() bool {
	switch sc {
	case scopeNotSpecified, ScopeThis, ScopeThisEarlier, ScopeThisLater, ScopeWholeSeries:
		return true
	}
	return false
}

// series reports whether the scope reaches beyond the one booking.
func (sc CategoryScope) series() bool {
	return sc == ScopeThisEarlier || sc == ScopeThisLater || sc == ScopeWholeSeries
}

// changesPayment reports whether the recurring payment itself takes the new
// category, so that future bookings use it too.
func (sc CategoryScope) changesPayment() bool {
	return sc == ScopeThisLater || sc == ScopeWholeSeries
}

// seriesChange is a new category for the other bookings of a recurring payment.
type seriesChange struct {
	ledgerID, bookingID string
	paymentID, dueOn    string // the occurrence the edited booking pays
	categoryID          string
	categoryGiven       bool // the patch named a category
	scope               CategoryScope
	editorID            string
	now                 time.Time
}

// applyCategoryToSeries gives the other bookings of the same recurring
// payment the new category (rule R9). Earlier and later are measured by the
// due date they pay, including that of the edited booking. Deleted bookings
// keep their category. The owner changes every booking; a member changes only
// their own, the ones they may edit. Scopes that reach into the future also
// change the payment, which only the owner may do (rule R8).
func applyCategoryToSeries(ctx context.Context, tx *sql.Tx, c seriesChange) error {
	if c.paymentID == "" || !c.categoryGiven {
		return ErrInvalidCategoryScope
	}
	isOwner, err := ownsLedgerOf(ctx, tx, c.editorID, c.ledgerID)
	if err != nil {
		return err
	}
	if c.scope.changesPayment() && !isOwner {
		return ErrNotOwner
	}
	// The category may be the booking's old one, which skipped the check in
	// UpdateTransaction; the other bookings choose it anew.
	if err := ensureCategoryFits(ctx, tx, c.ledgerID, c.categoryID, TypeExpense, true); err != nil {
		return err
	}

	query := `
		UPDATE transactions SET category_id = ?, updated_at = ?
		WHERE ledger_id = ? AND recurring_payment_id = ? AND id <> ? AND deleted_at IS NULL
		  AND (? OR user_id = ?)`
	args := []any{c.categoryID, timestamp(c.now), c.ledgerID, c.paymentID, c.bookingID, isOwner, c.editorID}
	switch c.scope {
	case ScopeThisEarlier:
		query += ` AND recurring_due_on <= ?`
		args = append(args, c.dueOn)
	case ScopeThisLater:
		query += ` AND recurring_due_on >= ?`
		args = append(args, c.dueOn)
	}
	if _, err := tx.ExecContext(ctx, query, args...); err != nil {
		return err
	}

	if !c.scope.changesPayment() {
		return nil
	}
	_, err = tx.ExecContext(ctx,
		`UPDATE recurring_payment SET category_id = ? WHERE id = ? AND ledger_id = ?`,
		c.categoryID, c.paymentID, c.ledgerID)
	return err
}
