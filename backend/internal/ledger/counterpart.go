package ledger

import (
	"context"
	"strings"
)

// Ledger kinds, as named in API responses.
const (
	KindPersonal  = "personal"
	KindHousehold = "household"
)

// Counterpart is the ledger on the other side of a transfer between two
// ledgers (rule X17). LedgerID and Name are only set when the viewer may
// access that ledger, so another member's personal ledger stays hidden.
type Counterpart struct {
	Kind     string
	LedgerID string
	Name     string
}

// Counterparts returns the other side of each transfer leg among txIDs, keyed
// by transaction ID. Transactions without a linked leg are left out: normal
// bookings, and legs whose other ledger was deleted (rule X16).
func (s *Store) Counterparts(ctx context.Context, viewerID string, txIDs []string) (map[string]Counterpart, error) {
	out := make(map[string]Counterpart)
	if len(txIDs) == 0 {
		return out, nil
	}

	// The access check matches CanAccessLedger. "IS ?" compares NULL safely,
	// so a household ledger (user_id NULL) gives false instead of NULL.
	placeholders := strings.TrimSuffix(strings.Repeat("?,", len(txIDs)), ",")
	args := []any{viewerID, viewerID}
	for _, id := range txIDs {
		args = append(args, id)
	}
	rows, err := s.db.QueryContext(ctx, `
		SELECT t.id,
		       CASE WHEN ol.household_id IS NULL THEN '`+KindPersonal+`' ELSE '`+KindHousehold+`' END,
		       ol.id, ol.name,
		       (ol.user_id IS ? OR m.user_id IS NOT NULL)
		FROM transactions t
		JOIN transactions o ON o.id = t.linked_transaction_id
		JOIN ledger ol ON ol.id = o.ledger_id
		LEFT JOIN household_member m ON m.household_id = ol.household_id AND m.user_id = ?
		WHERE t.id IN (`+placeholders+`)`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var (
			txID, ledgerID, name string
			c                    Counterpart
			visible              bool
		)
		if err := rows.Scan(&txID, &c.Kind, &ledgerID, &name, &visible); err != nil {
			return nil, err
		}
		if visible {
			c.LedgerID, c.Name = ledgerID, name
		}
		out[txID] = c
	}
	return out, rows.Err()
}
