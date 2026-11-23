package ledger

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"time"
)

// Errors for keeping reserves above zero (rule R12).
var (
	ErrReserveShort      = errors.New("this would take the reserve below zero; top it up first")
	ErrTopUpNeedsReserve = errors.New("only an expense from the reserve of a recurring payment can be topped up")
)

// potBalanceSQL is the balance of pot p over all bookings that are not
// deleted: income, expenses and transfers between ledgers count with their
// sign; a transfer between two pots of the ledger takes money out of its pot
// (pot_id) and puts it into its counterpart pot.
const potBalanceSQL = `(
	(SELECT COALESCE(SUM(t.amount_cents), 0) FROM transactions t
	  WHERE t.pot_id = p.id AND t.deleted_at IS NULL
	    AND t.type IN ('expense', 'income', 'transfer_out', 'transfer_in'))
	- (SELECT COALESCE(SUM(t.amount_cents), 0) FROM transactions t
	  WHERE t.pot_id = p.id AND t.type = 'transfer' AND t.deleted_at IS NULL)
	+ (SELECT COALESCE(SUM(t.amount_cents), 0) FROM transactions t
	  WHERE t.counterpart_pot_id = p.id AND t.type = 'transfer' AND t.deleted_at IS NULL)
)`

// reserveGuard remembers the balances of the active reserves among some pots
// before a change, so that check can reject the change if it takes one of
// them below zero (rule R12).
type reserveGuard map[string]int64

// guardReserves starts a reserveGuard for the given pots. Empty IDs and pots
// that are not active reserves are ignored.
func guardReserves(ctx context.Context, q queryer, potIDs ...string) (reserveGuard, error) {
	return activeReserveBalances(ctx, q, potIDs)
}

// check rejects the change if a reserve is below zero now and lower than
// before. A reserve that was below zero already (e.g. a pot that became a
// reserve again) may still be topped up.
func (g reserveGuard) check(ctx context.Context, q queryer) error {
	if len(g) == 0 {
		return nil
	}
	ids := make([]string, 0, len(g))
	for id := range g {
		ids = append(ids, id)
	}
	after, err := activeReserveBalances(ctx, q, ids)
	if err != nil {
		return err
	}
	for id, balance := range after {
		if balance < 0 && balance < g[id] {
			return ErrReserveShort
		}
	}
	return nil
}

// activeReserveBalances returns the balance of each active reserve among the
// given pots, by pot ID.
func activeReserveBalances(ctx context.Context, q queryer, potIDs []string) (map[string]int64, error) {
	args := []any{}
	for _, id := range potIDs {
		if id != "" {
			args = append(args, id)
		}
	}
	out := map[string]int64{}
	if len(args) == 0 {
		return out, nil
	}
	placeholders := strings.TrimSuffix(strings.Repeat("?, ", len(args)), ", ")
	rows, err := q.QueryContext(ctx, `
		SELECT p.id, `+potBalanceSQL+`
		FROM pot p JOIN recurring_payment r ON `+activeReserveCondition+`
		WHERE p.id IN (`+placeholders+`)`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var id string
		var balance int64
		if err := rows.Scan(&id, &balance); err != nil {
			return nil, err
		}
		out[id] = balance
	}
	return out, rows.Err()
}

// transactionPots returns the pots of the given transaction of a ledger and,
// for a transfer between ledgers, of its other leg: the pots whose balance
// changes when it is deleted or restored.
func transactionPots(ctx context.Context, q queryer, ledgerID, id string) ([]string, error) {
	rows, err := q.QueryContext(ctx, `
		SELECT pot_id, COALESCE(counterpart_pot_id, '') FROM transactions
		WHERE `+selfAndLinkedLeg, id, ledgerID, id, ledgerID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var pots []string
	for rows.Next() {
		var pot, counterpart string
		if err := rows.Scan(&pot, &counterpart); err != nil {
			return nil, err
		}
		pots = append(pots, pot, counterpart)
	}
	return pots, rows.Err()
}

// topUpReserve prepares an expense from a reserve (rule R13): if the reserve
// holds less than the expense, it moves the missing amount from the default
// pot into it, booked on the same day by the same person. It returns the
// amount moved, 0 if the reserve holds enough.
func topUpReserve(ctx context.Context, tx *sql.Tx, in NewTransaction, potID string, now time.Time) (int64, error) {
	if in.Type != TypeExpense {
		return 0, ErrTopUpNeedsReserve
	}
	balances, err := activeReserveBalances(ctx, tx, []string{potID})
	if err != nil {
		return 0, err
	}
	balance, ok := balances[potID]
	if !ok {
		return 0, ErrTopUpNeedsReserve
	}
	missing := in.AmountCents - balance
	if missing <= 0 {
		return 0, nil
	}
	defaultPot, err := resolvePot(ctx, tx, in.LedgerID, "")
	if err != nil {
		return 0, err
	}
	topUp := NewTransaction{
		LedgerID: in.LedgerID, UserID: in.UserID, Type: TypeTransfer,
		PotID: defaultPot, CounterpartPotID: potID,
		AmountCents: missing, BookedOn: in.BookedOn,
	}
	if _, err := insertTransaction(ctx, tx, topUp, now); err != nil {
		return 0, err
	}
	return missing, nil
}
