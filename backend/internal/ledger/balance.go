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
	// DefaultPotName names the pot behind AvailableCents, usually "Main".
	DefaultPotName string
	// TotalCents is the balance of all pots together, archived ones included.
	TotalCents int64
	// UnbookedCount and UnbookedCents are the recurring payments that are due
	// or overdue on AsOf but neither booked nor skipped.
	UnbookedCount int
	UnbookedCents int64
	// EstimatedCents is the estimated account balance: TotalCents minus the
	// unbooked payments, because the bank has most likely paid them already.
	EstimatedCents int64
}

// LedgerBalance returns the balance of a ledger, counting only transactions
// booked on or before asOf, and estimates the account balance from it.
// Planned transactions after that day do not count yet. An empty asOf means
// today in the server's time zone.
func (s *Store) LedgerBalance(ctx context.Context, ledgerID, asOf string, now time.Time) (Balance, error) {
	asOf, err := Today(asOf, now)
	if err != nil {
		return Balance{}, err
	}

	// Pot balances follow the same rules as in ListPots. A transfer between
	// two pots of the ledger leaves the total unchanged, but moves money out
	// of (pot_id) or into (counterpart_pot_id) the default pot. Starting from
	// the default pot gives one row even for a ledger without transactions.
	b := Balance{AsOf: asOf}
	err = s.db.QueryRowContext(ctx, `
		SELECT
		  d.name,
		  COALESCE(SUM(CASE WHEN t.type <> 'transfer' THEN t.amount_cents ELSE 0 END), 0),
		  COALESCE(SUM(CASE
		    WHEN t.type <> 'transfer' AND t.pot_id = d.id THEN t.amount_cents
		    WHEN t.type = 'transfer' AND t.pot_id = d.id THEN -t.amount_cents
		    WHEN t.type = 'transfer' AND t.counterpart_pot_id = d.id THEN t.amount_cents
		    ELSE 0 END), 0)
		FROM pot d
		LEFT JOIN transactions t
		  ON t.ledger_id = d.ledger_id AND t.deleted_at IS NULL AND t.booked_on <= ?
		WHERE d.ledger_id = ? AND d.is_default = 1
		GROUP BY d.id`,
		asOf, ledgerID,
	).Scan(&b.DefaultPotName, &b.TotalCents, &b.AvailableCents)
	if err != nil {
		return Balance{}, err
	}
	if err := s.addUnbookedPayments(ctx, ledgerID, &b); err != nil {
		return Balance{}, err
	}
	return b, nil
}

// addUnbookedPayments fills in the unbooked recurring payments and the
// estimated balance. Upcoming occurrences do not count: the bank has not
// paid them yet.
func (s *Store) addUnbookedPayments(ctx context.Context, ledgerID string, b *Balance) error {
	open, err := s.OpenOccurrences(ctx, ledgerID, b.AsOf)
	if err != nil {
		return err
	}
	for _, o := range open {
		if o.Status != StatusUpcoming {
			b.UnbookedCount++
			b.UnbookedCents += o.Payment.AmountCents
		}
	}
	b.EstimatedCents = b.TotalCents - b.UnbookedCents
	return nil
}
