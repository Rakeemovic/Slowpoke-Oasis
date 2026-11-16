package ledger

import (
	"context"
	"database/sql"
)

// FormerMemberID is the placeholder user that replaces a departed member in
// household transactions (rule X12). It is created by migration 0003.
const FormerMemberID = "former-member"

// ReleaseHouseholdTransactionsTx hands the household transactions of a user
// over to the placeholder. Amounts, dates and notes stay unchanged. It must
// run before the user row is deleted, because transactions reference the user.
func ReleaseHouseholdTransactionsTx(ctx context.Context, tx *sql.Tx, userID string) error {
	if _, err := tx.ExecContext(ctx,
		`UPDATE transactions SET user_id = ? WHERE user_id = ?`, FormerMemberID, userID,
	); err != nil {
		return err
	}
	_, err := tx.ExecContext(ctx,
		`UPDATE transactions SET deleted_by = ? WHERE deleted_by = ?`, FormerMemberID, userID,
	)
	return err
}
