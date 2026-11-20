package ledger

import (
	"context"
	"strings"
)

// Permission tells a viewer what they may do with a transaction, so the app
// can hide actions that the server would refuse anyway (rule X19).
type Permission struct {
	CanEdit   bool
	CanDelete bool
}

// Permissions returns the Permission of userID for each transaction among
// items, keyed by transaction ID, with a single query. The answers match
// AuthorizeTransactionChange and, for the legs of a transfer between ledgers,
// the check of both legs that deleting such a leg needs.
func (s *Store) Permissions(ctx context.Context, userID string, items []Transaction) (map[string]Permission, error) {
	out := make(map[string]Permission, len(items))
	if len(items) == 0 {
		return out, nil
	}

	placeholders := strings.TrimSuffix(strings.Repeat("?,", len(items)), ",")
	args := []any{userID, userID, userID, userID}
	for _, t := range items {
		args = append(args, t.ID)
	}
	// "IS ?" compares NULL safely: a household ledger (user_id NULL) and a
	// missing partner ledger both give false instead of NULL.
	rows, err := s.db.QueryContext(ctx, `
		SELECT t.id, t.type, t.user_id, `+ownsLedger("l")+`,
		       COALESCE(p.user_id, ''), `+ownsLedger("pl")+`
		FROM transactions t
		JOIN ledger l ON l.id = t.ledger_id
		LEFT JOIN transactions p ON p.id = t.linked_transaction_id
		LEFT JOIN ledger pl ON pl.id = p.ledger_id
		WHERE t.id IN (`+placeholders+`)`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var (
			id, author, partnerAuthor string
			txType                    TxType
			owner, partnerOwner       bool
		)
		if err := rows.Scan(&id, &txType, &author, &owner, &partnerAuthor, &partnerOwner); err != nil {
			return nil, err
		}
		linked := txType == TypeTransferOut || txType == TypeTransferIn
		// A leg whose other ledger was deleted has no partner (rule X16).
		partnerOK := partnerAuthor == "" || mayChange(ActionDelete, userID, partnerAuthor, partnerOwner)
		out[id] = Permission{
			CanEdit:   !linked && mayChange(ActionEdit, userID, author, owner),
			CanDelete: partnerOK && mayChange(ActionDelete, userID, author, owner),
		}
	}
	return out, rows.Err()
}

// ownsLedger is an SQL expression that is true when the user (two "?"
// arguments) owns the ledger with the given alias: a personal ledger by its
// user, a household ledger by the household owner.
func ownsLedger(alias string) string {
	return `(` + alias + `.user_id IS ? OR EXISTS (
		SELECT 1 FROM household_member m
		WHERE m.household_id = ` + alias + `.household_id AND m.user_id = ? AND m.role = '` + RoleOwner + `'))`
}
