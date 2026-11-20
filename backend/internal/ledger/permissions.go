package ledger

import (
	"context"
	"database/sql"
	"errors"
)

// ErrNotAllowed is returned when a user may see a transaction but may not
// change it.
var ErrNotAllowed = errors.New("you are not allowed to change this transaction")

// TransactionAction is a change to a transaction that needs a permission check.
type TransactionAction int

const (
	// ActionEdit changes the fields of a transaction.
	ActionEdit TransactionAction = iota
	// ActionDelete soft-deletes a transaction. Restoring one uses the same rule.
	ActionDelete
)

// AuthorizeTransactionChange checks whether a user may perform an action on a
// transaction of a ledger. The rules are:
//
//   - Edit: the author may edit, and so may the owner of the ledger. In a
//     household the owner is the household owner. A personal ledger is
//     always owned by its user.
//   - Delete and restore: only the author may delete a transaction. The owner
//     may also delete bookings whose author has left, because nobody else
//     could otherwise remove them.
//
// It returns ErrTransactionNotFound if the transaction is not in the ledger,
// and ErrNotAllowed if the user may see the transaction but not change it.
func (s *Store) AuthorizeTransactionChange(ctx context.Context, userID, ledgerID, id string, action TransactionAction) error {
	var author, role string
	var personalOwner bool
	err := s.db.QueryRowContext(ctx, `
		SELECT t.user_id,
		       COALESCE(l.user_id = ?, 0),
		       COALESCE((SELECT m.role FROM household_member m
		                 WHERE m.household_id = l.household_id AND m.user_id = ?), '')
		FROM transactions t
		JOIN ledger l ON l.id = t.ledger_id
		WHERE t.id = ? AND t.ledger_id = ?`,
		userID, userID, id, ledgerID,
	).Scan(&author, &personalOwner, &role)
	if errors.Is(err, sql.ErrNoRows) {
		return ErrTransactionNotFound
	}
	if err != nil {
		return err
	}

	if !mayChange(action, userID, author, personalOwner || role == RoleOwner) {
		return ErrNotAllowed
	}
	return nil
}

// mayChange holds the rules described at AuthorizeTransactionChange for one
// transaction. isOwner says whether userID owns the transaction's ledger.
// Keeping them in one place lets the list flags (Permissions) never disagree
// with the check that guards the actual change.
func mayChange(action TransactionAction, userID, author string, isOwner bool) bool {
	isAuthor := author == userID
	switch action {
	case ActionEdit:
		return isAuthor || isOwner
	case ActionDelete:
		return isAuthor || (isOwner && author == FormerMemberID)
	}
	return false
}
