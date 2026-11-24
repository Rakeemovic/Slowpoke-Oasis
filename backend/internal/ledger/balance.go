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
	// SeparateAccounts are the pots that are real sub-accounts at the bank
	// (rule T9) with their balance, archived ones included. SeparateCents is
	// their sum.
	SeparateAccounts []SeparateAccount
	SeparateCents    int64
	// EstimatedCents is the estimated account balance: TotalCents minus the
	// separate accounts, whose money is not on the main account, and minus the
	// unbooked payments, because the bank has most likely paid them already.
	EstimatedCents int64
	// DefaultPotID identifies the bank account of the ledger when reconciling.
	DefaultPotID string
	// Reconciled is the last reconciliation of the bank account, or nil
	// (rule L9).
	Reconciled *Reconciliation
}

// SeparateAccount is a pot that is a real sub-account at the bank.
type SeparateAccount struct {
	PotID        string
	Name         string
	BalanceCents int64
	// Reconciled is the last reconciliation of the sub-account, or nil.
	Reconciled *Reconciliation
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
	b, err := ledgerBalance(ctx, s.db, ledgerID, asOf)
	if err != nil {
		return Balance{}, err
	}
	return b, s.addReconciliations(ctx, ledgerID, &b)
}

// addReconciliations fills in the last reconciliation of the bank account
// and of each separate account.
func (s *Store) addReconciliations(ctx context.Context, ledgerID string, b *Balance) error {
	last, err := lastReconciliations(ctx, s.db, ledgerID)
	if err != nil {
		return err
	}
	b.Reconciled = last[b.DefaultPotID]
	for i := range b.SeparateAccounts {
		b.SeparateAccounts[i].Reconciled = last[b.SeparateAccounts[i].PotID]
	}
	return nil
}

// ledgerBalance is LedgerBalance on q for a valid asOf, so it also works
// inside a database transaction.
func ledgerBalance(ctx context.Context, q queryer, ledgerID, asOf string) (Balance, error) {
	// Pot balances follow the same rules as in ListPots. A transfer between
	// two pots of the ledger leaves the total unchanged, but moves money out
	// of (pot_id) or into (counterpart_pot_id) the default pot. Starting from
	// the default pot gives one row even for a ledger without transactions.
	b := Balance{AsOf: asOf}
	err := q.QueryRowContext(ctx, `
		SELECT
		  d.id, d.name,
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
	).Scan(&b.DefaultPotID, &b.DefaultPotName, &b.TotalCents, &b.AvailableCents)
	if err != nil {
		return Balance{}, err
	}
	if b.SeparateAccounts, err = separateAccounts(ctx, q, ledgerID, asOf); err != nil {
		return Balance{}, err
	}
	for _, a := range b.SeparateAccounts {
		b.SeparateCents += a.BalanceCents
	}
	if err := addUnbookedPayments(ctx, q, ledgerID, &b); err != nil {
		return Balance{}, err
	}
	return b, nil
}

// separateAccounts returns the sub-account pots of a ledger with their
// balance on asOf, ordered by name. The balance counts like the default
// pot's in LedgerBalance.
func separateAccounts(ctx context.Context, q queryer, ledgerID, asOf string) ([]SeparateAccount, error) {
	rows, err := q.QueryContext(ctx, `
		SELECT p.id, p.name,
		  COALESCE(SUM(CASE
		    WHEN t.type <> 'transfer' AND t.pot_id = p.id THEN t.amount_cents
		    WHEN t.type = 'transfer' AND t.pot_id = p.id THEN -t.amount_cents
		    WHEN t.type = 'transfer' AND t.counterpart_pot_id = p.id THEN t.amount_cents
		    ELSE 0 END), 0)
		FROM pot p
		LEFT JOIN transactions t
		  ON t.ledger_id = p.ledger_id AND t.deleted_at IS NULL AND t.booked_on <= ?
		WHERE p.ledger_id = ? AND p.is_separate_account = 1
		GROUP BY p.id
		ORDER BY p.name`, asOf, ledgerID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := []SeparateAccount{}
	for rows.Next() {
		var a SeparateAccount
		if err := rows.Scan(&a.PotID, &a.Name, &a.BalanceCents); err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

// addUnbookedPayments fills in the unbooked recurring payments and the
// estimated balance. Upcoming occurrences do not count: the bank has not
// paid them yet.
func addUnbookedPayments(ctx context.Context, q queryer, ledgerID string, b *Balance) error {
	open, err := openOccurrences(ctx, q, ledgerID, b.AsOf)
	if err != nil {
		return err
	}
	for _, o := range open {
		if o.Status != StatusUpcoming {
			b.UnbookedCount++
			b.UnbookedCents += o.Payment.AmountCents
		}
	}
	b.EstimatedCents = b.TotalCents - b.SeparateCents - b.UnbookedCents
	return nil
}
