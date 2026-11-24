package ledger

import (
	"context"
	"errors"
	"testing"
	"time"
)

// roundUpOn switches the round-up of a pot on (multiplier > 0) or off.
func roundUpOn(t *testing.T, s *Store, potID string, multiplier int, today string) {
	t.Helper()
	err := s.UpdatePot(context.Background(), "l1", potID, PotUpdate{RoundUpMultiplier: &multiplier, Today: today})
	if err != nil {
		t.Fatalf("set round-up of %s to x%d: %v", potID, multiplier, err)
	}
}

// expenseOf books an expense in the main pot on a day and returns its ID.
func expenseOf(t *testing.T, s *Store, cents int64, day string) string {
	t.Helper()
	return createExpense(t, s, "p-default", day, cents)
}

// roundUpOf returns the round-up transfer of an expense, or false if it has none.
func roundUpOf(t *testing.T, s *Store, expenseID string) (Transaction, bool) {
	t.Helper()
	all, err := s.ListTransactions(context.Background(), ListFilter{LedgerID: "l1"})
	if err != nil {
		t.Fatal(err)
	}
	for _, tx := range all {
		if tx.RoundUpOf == expenseID {
			return tx, true
		}
	}
	return Transaction{}, false
}

func potBalance(t *testing.T, s *Store, potID string) int64 {
	t.Helper()
	pots, err := s.ListPots(context.Background(), "l1")
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range pots {
		if p.ID == potID {
			return p.BalanceCents
		}
	}
	t.Fatalf("no pot %s", potID)
	return 0
}

func TestRoundUpCents(t *testing.T) {
	cases := []struct {
		amount     int64
		multiplier int
		want       int64
	}{
		{975, 1, 25}, {975, 2, 50}, {975, 3, 75}, {975, 5, 125}, // the N26 example: 9,75 €
		{1000, 3, 0}, {100, 1, 0}, // whole euros have nothing to round up
		{1, 1, 99}, {1250, 2, 100}, {101, 1, 99},
	}
	for _, c := range cases {
		if got := roundUpCents(c.amount, c.multiplier); got != c.want {
			t.Errorf("roundUpCents(%d, x%d) = %d, want %d", c.amount, c.multiplier, got, c.want)
		}
	}
}

func TestExpenseOfTheMainPotIsRoundedUpIntoTheCollectingPot(t *testing.T) {
	_, s := setup(t)
	roundUpOn(t, s, "p-holiday", 3, "2026-11-01")

	id := expenseOf(t, s, 975, "2026-11-16")
	r, ok := roundUpOf(t, s, id)
	if !ok {
		t.Fatal("no round-up booked")
	}
	if r.Type != TypeTransfer || r.PotID != "p-default" || r.CounterpartPotID != "p-holiday" || r.AmountCents != 75 ||
		r.BookedOn != "2026-11-16" || r.UserID != "u1" {
		t.Errorf("round-up = %+v, want a transfer of 75 from the main pot into Holiday on the day of the expense", r)
	}
	// The expense plus the spare change leave the main pot.
	if main, holiday := potBalance(t, s, "p-default"), potBalance(t, s, "p-holiday"); main != -975-75 || holiday != 75 {
		t.Errorf("balances: main %d, holiday %d; want -1050, 75", main, holiday)
	}
}

func TestSomeBookingsAreNotRoundedUp(t *testing.T) {
	_, s := setup(t)
	ctx := context.Background()
	rentID := mustCreateRecurring(t, s, rent())
	roundUpOn(t, s, "p-holiday", 1, "2026-11-10")

	payment, err := payOccurrence(s, rentID, "2026-11-20")
	if err != nil {
		t.Fatal(err)
	}
	income := baseExpense()
	income.Type, income.CategoryID, income.AmountCents, income.BookedOn = TypeIncome, "c-salary", 1250, "2026-11-16"
	incomeID, err := s.CreateTransaction(ctx, income, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	cases := map[string]string{
		"a payment of a recurring payment": payment,
		"income":                           incomeID,
		"an expense of another pot":        createExpense(t, s, "p-holiday", "2026-11-16", 1250),
		"an expense before the start day":  expenseOf(t, s, 1250, "2026-11-09"),
		"a whole euro amount":              expenseOf(t, s, 1300, "2026-11-16"),
	}
	for name, id := range cases {
		if r, ok := roundUpOf(t, s, id); ok {
			t.Errorf("%s was rounded up: %+v", name, r)
		}
	}
	// On the start day itself it counts.
	if _, ok := roundUpOf(t, s, expenseOf(t, s, 1250, "2026-11-10")); !ok {
		t.Error("an expense on the start day was not rounded up")
	}
}

func TestChangingTheExpenseChangesItsRoundUp(t *testing.T) {
	_, s := setup(t)
	ctx := context.Background()
	roundUpOn(t, s, "p-holiday", 2, "2026-11-01")
	id := expenseOf(t, s, 1250, "2026-11-16")

	amount, day := int64(1275), "2026-11-17"
	if err := s.UpdateTransaction(ctx, "l1", id, TransactionPatch{AmountCents: &amount, BookedOn: &day}, time.Now()); err != nil {
		t.Fatal(err)
	}
	if r, ok := roundUpOf(t, s, id); !ok || r.AmountCents != 50 || r.BookedOn != day {
		t.Errorf("after the change: %+v (%v), want 50 on %s", r, ok, day)
	}

	// Each change that leaves the rules removes it, and coming back restores it.
	whole := int64(1300)
	steps := []struct {
		name  string
		patch TransactionPatch
		want  int64
	}{
		{"whole euro amount", TransactionPatch{AmountCents: &whole}, 0},
		{"amount with cents again", TransactionPatch{AmountCents: &amount}, 50},
		{"moved to another pot", TransactionPatch{PotID: ptr("p-holiday")}, 0},
		{"moved back", TransactionPatch{PotID: ptr("p-default")}, 50},
		{"turned into income", TransactionPatch{Type: ptr(TypeIncome), CategoryID: ptr("c-salary")}, 0},
	}
	for _, step := range steps {
		if err := s.UpdateTransaction(ctx, "l1", id, step.patch, time.Now()); err != nil {
			t.Fatalf("%s: %v", step.name, err)
		}
		r, ok := roundUpOf(t, s, id)
		if got := r.AmountCents; got != step.want || ok != (step.want > 0) {
			t.Errorf("%s: round-up %d (exists %v), want %d", step.name, got, ok, step.want)
		}
	}
}

func TestRoundUpFollowsTheExpenseWhenDeletedAndRestored(t *testing.T) {
	_, s := setup(t)
	ctx := context.Background()
	roundUpOn(t, s, "p-holiday", 1, "2026-11-01")
	id := expenseOf(t, s, 1250, "2026-11-16")
	now := time.Now()

	if err := s.DeleteTransaction(ctx, "l1", id, "u1", now); err != nil {
		t.Fatal(err)
	}
	if _, ok := roundUpOf(t, s, id); ok {
		t.Error("the round-up stays visible after its expense was deleted")
	}
	if balance := potBalance(t, s, "p-holiday"); balance != 0 {
		t.Errorf("holiday balance %d, want 0", balance)
	}
	// Only the expense is offered for restoring.
	deleted, err := s.ListDeletedTransactions(ctx, "l1", now, 0, 0)
	if err != nil || len(deleted) != 1 || deleted[0].ID != id {
		t.Fatalf("deleted = %+v (%v), want only the expense", deleted, err)
	}

	if err := s.RestoreTransaction(ctx, "l1", id, now); err != nil {
		t.Fatal(err)
	}
	if r, ok := roundUpOf(t, s, id); !ok || r.AmountCents != 50 {
		t.Errorf("after restoring: %+v (%v), want a round-up of 50", r, ok)
	}

	// The cleanup after 30 days removes both for good.
	if err := s.DeleteTransaction(ctx, "l1", id, "u1", now); err != nil {
		t.Fatal(err)
	}
	if _, err := s.PurgeDeletedTransactions(ctx, now.AddDate(0, 0, 31)); err != nil {
		t.Fatal(err)
	}
	all, err := s.ListTransactions(ctx, ListFilter{LedgerID: "l1"})
	if err != nil || len(all) != 0 {
		t.Errorf("after the purge: %+v (%v), want nothing", all, err)
	}
}

func TestARoundUpCannotBeChangedOnItsOwn(t *testing.T) {
	_, s := setup(t)
	ctx := context.Background()
	roundUpOn(t, s, "p-holiday", 1, "2026-11-01")
	r, _ := roundUpOf(t, s, expenseOf(t, s, 1250, "2026-11-16"))

	amount := int64(10)
	if err := s.UpdateTransaction(ctx, "l1", r.ID, TransactionPatch{AmountCents: &amount}, time.Now()); !errors.Is(err, ErrRoundUpTransaction) {
		t.Errorf("update: err = %v, want ErrRoundUpTransaction", err)
	}
	if err := s.DeleteTransaction(ctx, "l1", r.ID, "u1", time.Now()); !errors.Is(err, ErrRoundUpTransaction) {
		t.Errorf("delete: err = %v, want ErrRoundUpTransaction", err)
	}
	if err := s.RestoreTransaction(ctx, "l1", r.ID, time.Now()); !errors.Is(err, ErrRoundUpTransaction) {
		t.Errorf("restore: err = %v, want ErrRoundUpTransaction", err)
	}
}

// The app hides what the server refuses (rule X19), so the permissions of a
// round-up say it cannot be changed, while its expense can.
func TestARoundUpHasNoPermissions(t *testing.T) {
	_, s := setup(t)
	roundUpOn(t, s, "p-holiday", 1, "2026-11-01")
	expense := expenseOf(t, s, 1250, "2026-11-16")
	r, _ := roundUpOf(t, s, expense)

	perms, err := s.Permissions(context.Background(), "u1", []Transaction{{ID: expense}, {ID: r.ID}})
	if err != nil {
		t.Fatal(err)
	}
	if got := perms[r.ID]; got.CanEdit || got.CanDelete {
		t.Errorf("round-up permissions = %+v, want none", got)
	}
	if got := perms[expense]; !got.CanEdit || !got.CanDelete {
		t.Errorf("expense permissions = %+v, want edit and delete", got)
	}
}

func TestOnlyOnePotCollectsAndTheSettingIsChecked(t *testing.T) {
	_, s := setup(t)
	ctx := context.Background()
	other, err := s.CreatePot(ctx, NewPot{LedgerID: "l1", Name: "Bike"})
	if err != nil {
		t.Fatal(err)
	}
	roundUpOn(t, s, "p-holiday", 2, "2026-11-01")
	roundUpOn(t, s, other, 1, "2026-11-05")

	pots, _ := s.ListPots(ctx, "l1")
	for _, p := range pots {
		wantMultiplier, wantFrom := 0, ""
		if p.ID == other {
			wantMultiplier, wantFrom = 1, "2026-11-05"
		}
		if p.RoundUpMultiplier != wantMultiplier || p.RoundUpFrom != wantFrom {
			t.Errorf("%s: round-up x%d from %q, want x%d from %q", p.Name, p.RoundUpMultiplier, p.RoundUpFrom, wantMultiplier, wantFrom)
		}
	}

	// A new multiplier keeps the start day; switching off ends new round-ups.
	roundUpOn(t, s, other, 5, "2026-12-01")
	if pots, _ := s.ListPots(ctx, "l1"); potNamed(pots, "Bike").RoundUpFrom != "2026-11-05" {
		t.Errorf("start day changed with the multiplier: %+v", potNamed(pots, "Bike"))
	}
	old := expenseOf(t, s, 1250, "2026-11-16")
	roundUpOn(t, s, other, 0, "")
	if _, ok := roundUpOf(t, s, expenseOf(t, s, 1250, "2026-11-17")); ok {
		t.Error("a round-up was booked after switching it off")
	}
	if _, ok := roundUpOf(t, s, old); !ok {
		t.Error("switching it off removed an existing round-up")
	}

	bad := 4
	reserve := loadPayment(t, s, mustCreateRecurring(t, s, insurance())).ReservePotID
	yes := 1
	archived := true
	cases := []struct {
		name string
		pot  string
		u    PotUpdate
		want error
	}{
		{"unknown multiplier", other, PotUpdate{RoundUpMultiplier: &bad, Today: "2026-11-01"}, ErrInvalidMultiplier},
		{"missing day", other, PotUpdate{RoundUpMultiplier: &yes}, ErrInvalidDate},
		{"main pot", "p-default", PotUpdate{RoundUpMultiplier: &yes, Today: "2026-11-01"}, ErrRoundUpNotAllowed},
		{"reserve", reserve, PotUpdate{RoundUpMultiplier: &yes, Today: "2026-11-01"}, ErrRoundUpNotAllowed},
	}
	for _, tc := range cases {
		if err := s.UpdatePot(ctx, "l1", tc.pot, tc.u); !errors.Is(err, tc.want) {
			t.Errorf("%s: err = %v, want %v", tc.name, err, tc.want)
		}
	}

	// Archiving the collecting pot ends its round-up; an archived pot cannot start one.
	roundUpOn(t, s, other, 1, "2026-11-20")
	if err := s.UpdatePot(ctx, "l1", other, PotUpdate{Archived: &archived}); err != nil {
		t.Fatal(err)
	}
	if pots, _ := s.ListPots(ctx, "l1"); potNamed(pots, "Bike").RoundUpMultiplier != 0 {
		t.Error("an archived pot still collects round-ups")
	}
	if err := s.UpdatePot(ctx, "l1", other, PotUpdate{RoundUpMultiplier: &yes, Today: "2026-11-20"}); !errors.Is(err, ErrPotArchived) {
		t.Errorf("archived pot: err = %v, want ErrPotArchived", err)
	}
}

func TestHouseholdsHaveNoRoundUp(t *testing.T) {
	_, s := setup(t)
	ctx := context.Background()
	_, ledgerID, err := s.CreateHousehold(ctx, "u1", "Flat", time.Now())
	if err != nil {
		t.Fatal(err)
	}
	id, err := s.CreatePot(ctx, NewPot{LedgerID: ledgerID, Name: "Holiday"})
	if err != nil {
		t.Fatal(err)
	}
	yes := 1
	if err := s.UpdatePot(ctx, ledgerID, id, PotUpdate{RoundUpMultiplier: &yes, Today: "2026-11-01"}); !errors.Is(err, ErrRoundUpNotAllowed) {
		t.Errorf("err = %v, want ErrRoundUpNotAllowed", err)
	}
}

func potNamed(pots []Pot, name string) Pot {
	for _, p := range pots {
		if p.Name == name {
			return p
		}
	}
	return Pot{}
}
