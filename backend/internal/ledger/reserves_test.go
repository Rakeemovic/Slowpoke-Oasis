package ledger

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

// insurance is due every three months on the 15th, from February 2027 on.
func insurance() NewRecurringPayment {
	in := rent()
	in.Name = "Car insurance"
	in.AmountCents = 120000
	in.Schedule = Schedule{IntervalMonths: 3, DueDay: 15, FirstMonth: "2027-02"}
	return in
}

// loadPayment reads a payment of ledger l1.
func loadPayment(t *testing.T, s *Store, id string) RecurringPayment {
	t.Helper()
	p, err := loadRecurringPayment(context.Background(), s.db, "l1", id)
	if err != nil {
		t.Fatalf("load payment: %v", err)
	}
	return p
}

// moveToPot books a transfer between two pots of ledger l1.
func moveToPot(t *testing.T, s *Store, from, to string, cents int64, on string) string {
	t.Helper()
	in := baseExpense()
	in.Type, in.CategoryID = TypeTransfer, ""
	in.PotID, in.CounterpartPotID = from, to
	in.AmountCents, in.BookedOn = cents, on
	id, err := s.CreateTransaction(context.Background(), in, time.Now())
	if err != nil {
		t.Fatalf("move: %v", err)
	}
	return id
}

// expenseFrom returns an expense of the given amount from a pot.
func expenseFrom(potID string, cents int64) NewTransaction {
	in := baseExpense()
	in.PotID, in.AmountCents = potID, cents
	return in
}

func updatePayment(s *Store, id string, u RecurringPaymentUpdate) error {
	u.Today = today
	return s.UpdateRecurringPayment(context.Background(), "l1", id, u)
}

func TestPaymentDueEveryFewMonthsGetsReserve(t *testing.T) {
	db, s := setup(t)
	id := mustCreateRecurring(t, s, insurance())

	p := loadPayment(t, s, id)
	if p.ReservePotID == "" || p.PotID != p.ReservePotID {
		t.Fatalf("pot = %q, reserve = %q; want the payment to book from its reserve", p.PotID, p.ReservePotID)
	}
	var name, kind, reserveFor string
	var target int64
	err := db.QueryRow(`SELECT name, kind, target_cents, reserve_for FROM pot WHERE id = ?`, p.PotID).
		Scan(&name, &kind, &target, &reserveFor)
	if err != nil {
		t.Fatal(err)
	}
	if name != "Car insurance" || kind != "one_time" || target != 120000 || reserveFor != id {
		t.Fatalf("reserve = %q %q %d %q", name, kind, target, reserveFor)
	}
}

func TestMonthlyPaymentHasNoReserve(t *testing.T) {
	_, s := setup(t)
	p := loadPayment(t, s, mustCreateRecurring(t, s, rent()))
	if p.ReservePotID != "" || p.PotID != "p-default" {
		t.Fatalf("pot = %q, reserve = %q", p.PotID, p.ReservePotID)
	}
}

func TestReserveNameAvoidsActivePots(t *testing.T) {
	_, s := setup(t)
	in := insurance()
	in.Name = "holiday" // the pot "Holiday" exists
	first := loadPayment(t, s, mustCreateRecurring(t, s, in))
	second := loadPayment(t, s, mustCreateRecurring(t, s, in))

	pots := potNames(t, s)
	if pots[first.PotID] != "holiday (reserve)" || pots[second.PotID] != "holiday (reserve 2)" {
		t.Fatalf("names = %q, %q", pots[first.PotID], pots[second.PotID])
	}
}

func TestReserveNameFitsLimit(t *testing.T) {
	long := strings.Repeat("x", 50)
	for n := 1; n <= 3; n++ {
		if got := reserveName(long, n); len(got) > maxNameLength {
			t.Errorf("reserveName(n=%d) has %d characters", n, len(got))
		}
	}
	if got := reserveName(long, 2); !strings.HasSuffix(got, " (reserve)") {
		t.Errorf("got %q", got)
	}
}

func potNames(t *testing.T, s *Store) map[string]string {
	t.Helper()
	pots, err := s.ListPots(context.Background(), "l1")
	if err != nil {
		t.Fatal(err)
	}
	out := map[string]string{}
	for _, p := range pots {
		out[p.ID] = p.Name
	}
	return out
}

func TestReservePotCannotBeChosen(t *testing.T) {
	_, s := setup(t)
	in := insurance()
	in.PotID = "p-holiday"
	if _, err := s.CreateRecurringPayment(context.Background(), in, time.Now()); !errors.Is(err, ErrReservePotFixed) {
		t.Fatalf("create with pot: got %v, want ErrReservePotFixed", err)
	}

	id := mustCreateRecurring(t, s, insurance())
	other := "p-holiday"
	if err := updatePayment(s, id, RecurringPaymentUpdate{PotID: &other}); !errors.Is(err, ErrReservePotFixed) {
		t.Fatalf("change pot: got %v, want ErrReservePotFixed", err)
	}

	// Another payment must not book from this reserve.
	reserve := loadPayment(t, s, id).PotID
	monthly := rent()
	monthly.PotID = reserve
	if _, err := s.CreateRecurringPayment(context.Background(), monthly, time.Now()); !errors.Is(err, ErrPotIsReserve) {
		t.Fatalf("monthly payment from reserve: got %v, want ErrPotIsReserve", err)
	}
}

func TestReserveFollowsAmount(t *testing.T) {
	db, s := setup(t)
	id := mustCreateRecurring(t, s, insurance())
	amount := int64(150000)
	if err := updatePayment(s, id, RecurringPaymentUpdate{AmountCents: &amount}); err != nil {
		t.Fatal(err)
	}
	var target int64
	if err := db.QueryRow(`SELECT target_cents FROM pot WHERE reserve_for = ?`, id).Scan(&target); err != nil {
		t.Fatal(err)
	}
	if target != amount {
		t.Fatalf("target = %d, want %d", target, amount)
	}
}

func TestMonthlyAgainKeepsReserveForLater(t *testing.T) {
	_, s := setup(t)
	id := mustCreateRecurring(t, s, insurance())
	reserve := loadPayment(t, s, id).PotID

	monthly, quarterly := 1, 3
	if err := updatePayment(s, id, RecurringPaymentUpdate{IntervalMonths: &monthly}); err != nil {
		t.Fatal(err)
	}
	p := loadPayment(t, s, id)
	if p.PotID != "p-default" || p.ReservePotID != reserve {
		t.Fatalf("monthly: pot = %q, reserve = %q; want default pot and the old reserve", p.PotID, p.ReservePotID)
	}

	if err := updatePayment(s, id, RecurringPaymentUpdate{IntervalMonths: &quarterly}); err != nil {
		t.Fatal(err)
	}
	if p := loadPayment(t, s, id); p.PotID != reserve {
		t.Fatalf("quarterly again: pot = %q, want the old reserve %q", p.PotID, reserve)
	}
}

func TestArchivedReserveIsReplaced(t *testing.T) {
	_, s := setup(t)
	ctx := context.Background()
	id := mustCreateRecurring(t, s, insurance())
	reserve := loadPayment(t, s, id).PotID

	yes, no := true, false
	if err := s.UpdatePot(ctx, "l1", reserve, PotUpdate{Archived: &yes}); !errors.Is(err, ErrReserveInUse) {
		t.Fatalf("archive active reserve: got %v, want ErrReserveInUse", err)
	}
	if err := updatePayment(s, id, RecurringPaymentUpdate{Archived: &yes}); err != nil {
		t.Fatal(err)
	}
	if err := s.UpdatePot(ctx, "l1", reserve, PotUpdate{Archived: &yes}); err != nil {
		t.Fatalf("archive reserve of archived payment: %v", err)
	}

	if err := updatePayment(s, id, RecurringPaymentUpdate{Archived: &no}); err != nil {
		t.Fatal(err)
	}
	p := loadPayment(t, s, id)
	if p.PotID == reserve || p.PotID != p.ReservePotID {
		t.Fatalf("pot = %q, reserve = %q; want a new reserve", p.PotID, p.ReservePotID)
	}
}

func TestReserveNeverBelowZero(t *testing.T) {
	_, s := setup(t)
	ctx := context.Background()
	reserve := loadPayment(t, s, mustCreateRecurring(t, s, insurance())).PotID

	if _, err := s.CreateTransaction(ctx, expenseFrom(reserve, 100), time.Now()); !errors.Is(err, ErrReserveShort) {
		t.Fatalf("expense from empty reserve: got %v, want ErrReserveShort", err)
	}

	moveID := moveToPot(t, s, "p-default", reserve, 100, "2026-11-10")
	expenseID, err := s.CreateTransaction(ctx, expenseFrom(reserve, 100), time.Now())
	if err != nil {
		t.Fatalf("expense covered by the reserve: %v", err)
	}

	more := int64(101)
	if err := s.UpdateTransaction(ctx, "l1", expenseID, TransactionPatch{AmountCents: &more}, time.Now()); !errors.Is(err, ErrReserveShort) {
		t.Fatalf("raise expense: got %v, want ErrReserveShort", err)
	}
	if err := s.DeleteTransaction(ctx, "l1", moveID, "u1", time.Now()); !errors.Is(err, ErrReserveShort) {
		t.Fatalf("delete the money in the reserve: got %v, want ErrReserveShort", err)
	}

	// Deleting the expense first frees the transfer.
	if err := s.DeleteTransaction(ctx, "l1", expenseID, "u1", time.Now()); err != nil {
		t.Fatal(err)
	}
	if err := s.DeleteTransaction(ctx, "l1", moveID, "u1", time.Now()); err != nil {
		t.Fatal(err)
	}
	if err := s.RestoreTransaction(ctx, "l1", expenseID, time.Now()); !errors.Is(err, ErrReserveShort) {
		t.Fatalf("restore expense: got %v, want ErrReserveShort", err)
	}
}

func TestTopUpMovesWhatIsMissing(t *testing.T) {
	_, s := setup(t)
	ctx := context.Background()
	reserve := loadPayment(t, s, mustCreateRecurring(t, s, insurance())).PotID
	moveToPot(t, s, "p-default", reserve, 3000, "2026-11-10")

	_, toppedUp, err := s.CreateTransactionWithTopUp(ctx, expenseFrom(reserve, 10000), time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if toppedUp != 7000 {
		t.Fatalf("topped up %d, want 7000", toppedUp)
	}
	balances := potBalances(t, s)
	if balances[reserve] != 0 || balances["p-default"] != -10000 {
		t.Fatalf("balances = %v", balances)
	}

	// Enough in the reserve: nothing moves.
	moveToPot(t, s, "p-default", reserve, 500, "2026-11-10")
	if _, toppedUp, err = s.CreateTransactionWithTopUp(ctx, expenseFrom(reserve, 500), time.Now()); err != nil || toppedUp != 0 {
		t.Fatalf("topped up %d, err %v; want 0, nil", toppedUp, err)
	}

	if _, _, err := s.CreateTransactionWithTopUp(ctx, expenseFrom("p-holiday", 100), time.Now()); !errors.Is(err, ErrTopUpNeedsReserve) {
		t.Fatalf("top up a normal pot: got %v, want ErrTopUpNeedsReserve", err)
	}
}

func potBalances(t *testing.T, s *Store) map[string]int64 {
	t.Helper()
	pots, err := s.ListPots(context.Background(), "l1")
	if err != nil {
		t.Fatal(err)
	}
	out := map[string]int64{}
	for _, p := range pots {
		out[p.ID] = p.BalanceCents
	}
	return out
}

func TestReserveRates(t *testing.T) {
	_, s := setup(t)
	ctx := context.Background()
	reserve := loadPayment(t, s, mustCreateRecurring(t, s, insurance())).PotID

	rates := func(on string) ReserveRate {
		t.Helper()
		got, err := s.ReserveRates(ctx, "l1", on)
		if err != nil || len(got) != 1 {
			t.Fatalf("rates on %s: %v, %v", on, got, err)
		}
		return got[0]
	}

	// November to February are four periods: 1,200.00 / 4.
	r := rates(today)
	if r.DueOn != "2027-02-15" || r.Periods != 4 || r.RateCents != 30000 || r.OpenCents != 30000 {
		t.Fatalf("before saving: %+v", r)
	}

	moveToPot(t, s, "p-default", reserve, 10000, "2026-11-10")
	r = rates(today)
	if r.RateCents != 30000 || r.SavedCents != 10000 || r.OpenCents != 20000 || r.BalanceCents != 10000 {
		t.Fatalf("after saving part: %+v", r)
	}

	// December: what is missing (1,100.00) spreads over three periods.
	r = rates("2026-12-05")
	if r.Periods != 3 || r.RateCents != 36667 || r.SavedCents != 0 {
		t.Fatalf("next period: %+v", r)
	}

	// After paying the occurrence, the reserve saves for the next one.
	moveToPot(t, s, "p-default", reserve, 110000, "2027-02-01")
	in := expenseFrom(reserve, 120000)
	in.RecurringPaymentID, in.RecurringDueOn = r.Payment.ID, "2027-02-15"
	if _, err := s.CreateTransaction(ctx, in, time.Now()); err != nil {
		t.Fatal(err)
	}
	r = rates("2027-02-20")
	if r.DueOn != "2027-05-15" || r.Periods != 4 || r.RateCents != 30000 {
		t.Fatalf("after paying: %+v", r)
	}
}
