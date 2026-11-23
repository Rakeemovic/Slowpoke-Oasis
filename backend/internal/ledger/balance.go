package ledger

import (
	"context"
	"time"
)

// Balance is what a ledger holds on a given day (rule L7).
type Balance struct {
	AsOf string // YYYY-MM-DD, inclusive
	// AvailableCents is the balance of the default pot: money that is not set
	// aside in another pot.
	AvailableCents int64
	// TotalCents is the balance of all pots together, archived ones included.
	TotalCents int64
}

// LedgerBalance returns the balance of a ledger, counting only transactions
// booked on or before asOf. Planned transactions after that day do not count
// yet. An empty asOf means today in the server's time zone.
func (s *Store) LedgerBalance(ctx context.Context, ledgerID, asOf string, now time.Time) (Balance, error) {
	asOf, err := Today(asOf, now)
	if err != nil {
		return Balance{}, err
	}

	// Pot balances follow the same rules as in ListPots. A transfer between
	// two pots of the ledger leaves the total unchanged, but moves money out
	// of (pot_id) or into (counterpart_pot_id) the default pot.
	b := Balance{AsOf: asOf}
	err = s.db.QueryRowContext(ctx, `
		SELECT
		  COALESCE(SUM(CASE WHEN t.type <> 'transfer' THEN t.amount_cents ELSE 0 END), 0),
		  COALESCE(SUM(CASE
		    WHEN t.type <> 'transfer' AND t.pot_id = d.id THEN t.amount_cents
		    WHEN t.type = 'transfer' AND t.pot_id = d.id THEN -t.amount_cents
		    WHEN t.type = 'transfer' AND t.counterpart_pot_id = d.id THEN t.amount_cents
		    ELSE 0 END), 0)
		FROM transactions t
		JOIN pot d ON d.ledger_id = t.ledger_id AND d.is_default = 1
		WHERE t.ledger_id = ? AND t.deleted_at IS NULL AND t.booked_on <= ?`,
		ledgerID, asOf,
	).Scan(&b.TotalCents, &b.AvailableCents)
	return b, err
}
