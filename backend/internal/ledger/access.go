package ledger

import "context"

// CanAccessLedger reports whether a user may read and change a ledger.
// A personal ledger is accessible to its owner. A household ledger is
// accessible to every member of that household.
func (s *Store) CanAccessLedger(ctx context.Context, userID, ledgerID string) (bool, error) {
	var allowed bool
	err := s.db.QueryRowContext(ctx, `
		SELECT EXISTS (
			SELECT 1
			FROM ledger l
			LEFT JOIN household_member m
			       ON m.household_id = l.household_id AND m.user_id = ?
			WHERE l.id = ?
			  AND (l.user_id = ? OR m.user_id IS NOT NULL)
		)`,
		userID, ledgerID, userID,
	).Scan(&allowed)
	return allowed, err
}
