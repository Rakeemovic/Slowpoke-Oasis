package ledger

import (
	"context"
	"database/sql"
	"errors"
	"testing"
	"time"
)

// rent is a monthly payment on the 20th, created on today (2026-11-16).
func rent() NewRecurringPayment {
	return NewRecurringPayment{
		LedgerID:    "l1",
		Name:        "Rent",
		AmountCents: 80000,
		CategoryID:  "c-food",
		Schedule:    Schedule{IntervalMonths: 1, DueDay: 20, FirstMonth: "2026-01"},
		Today:       today,
	}
}

func mustCreateRecurring(t *testing.T, s *Store, in NewRecurringPayment) string {
	t.Helper()
	id, err := s.CreateRecurringPayment(context.Background(), in, time.Now())
	if err != nil {
		t.Fatalf("create recurring payment: %v", err)
	}
	return id
}

// payOccurrence books the occurrence due on dueOn as an expense.
func payOccurrence(s *Store, paymentID, dueOn string) (string, error) {
	in := baseExpense()
	in.RecurringPaymentID = paymentID
	in.RecurringDueOn = dueOn
	return s.CreateTransaction(context.Background(), in, time.Now())
}

// openDates returns the due dates of the open occurrences on the given day.
func openDates(t *testing.T, s *Store, on string) []string {
	t.Helper()
	items, err := s.OpenOccurrences(context.Background(), "l1", on)
	if err != nil {
		t.Fatalf("open occurrences: %v", err)
	}
	var out []string
	for _, o := range items {
		out = append(out, o.DueOn+" "+string(o.Status))
	}
	return out
}

func assertDates(t *testing.T, got []string, want ...string) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	for i := range got {
		if got[i] != want[i] {
			t.Fatalf("got %v, want %v", got, want)
		}
	}
}

func TestCreateRecurringPaymentUsesDefaultPot(t *testing.T) {
	_, s := setup(t)
	id := mustCreateRecurring(t, s, rent())

	items, err := s.ListRecurringPayments(context.Background(), "l1")
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 || items[0].ID != id || items[0].PotID != "p-default" || items[0].TrackedFrom != today {
		t.Fatalf("unexpected payments: %+v", items)
	}
}

func TestCreateRecurringPaymentValidates(t *testing.T) {
	db, s := setup(t)
	mustExec(t, db, `INSERT INTO pot (id, ledger_id, name, kind, archived) VALUES ('p-old', 'l1', 'Old', 'one_time', 1)`)

	cases := []struct {
		name   string
		change func(*NewRecurringPayment)
		want   error
	}{
		{"income category", func(in *NewRecurringPayment) { in.CategoryID = "c-salary" }, ErrRecurringNeedsExpense},
		{"archived pot", func(in *NewRecurringPayment) { in.PotID = "p-old" }, ErrPotArchived},
		{"empty name", func(in *NewRecurringPayment) { in.Name = " " }, ErrInvalidName},
		{"zero amount", func(in *NewRecurringPayment) { in.AmountCents = 0 }, ErrRecurringAmount},
		{"bad interval", func(in *NewRecurringPayment) { in.IntervalMonths = 2 }, ErrInvalidInterval},
		{"bad today", func(in *NewRecurringPayment) { in.Today = "" }, ErrInvalidDate},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			in := rent()
			c.change(&in)
			if _, err := s.CreateRecurringPayment(context.Background(), in, time.Now()); !errors.Is(err, c.want) {
				t.Fatalf("got %v, want %v", err, c.want)
			}
		})
	}
}

func TestOpenOccurrencesStartAtCreationAndEndWithPeriod(t *testing.T) {
	_, s := setup(t)
	mustCreateRecurring(t, s, rent())

	// Created on 2026-11-16: October and earlier never become open (rule R5).
	assertDates(t, openDates(t, s, today), "2026-11-20 upcoming")
	assertDates(t, openDates(t, s, "2026-11-20"), "2026-11-20 due")
	assertDates(t, openDates(t, s, "2026-12-20"), "2026-11-20 overdue", "2026-12-20 due")
}

func TestOpenOccurrencesFollowPayday(t *testing.T) {
	db, s := setup(t)
	mustExec(t, db, `UPDATE ledger SET payday_day = 25 WHERE id = 'l1'`)
	in := rent()
	in.DueDay = 1
	mustCreateRecurring(t, s, in)

	// The period that contains 2026-11-16 runs to 2026-11-24, so the
	// payment on 2026-12-01 belongs to the next period.
	assertDates(t, openDates(t, s, today))
	assertDates(t, openDates(t, s, "2026-11-25"), "2026-12-01 upcoming")
}

func TestBookingClosesOccurrence(t *testing.T) {
	_, s := setup(t)
	ctx := context.Background()
	id := mustCreateRecurring(t, s, rent())

	txID, err := payOccurrence(s, id, "2026-11-20")
	if err != nil {
		t.Fatalf("book: %v", err)
	}
	assertDates(t, openDates(t, s, today))

	tx, err := s.GetTransaction(ctx, "l1", txID)
	if err != nil {
		t.Fatal(err)
	}
	if tx.RecurringPaymentID != id || tx.RecurringDueOn != "2026-11-20" {
		t.Fatalf("link not returned: %+v", tx)
	}

	if _, err := payOccurrence(s, id, "2026-11-20"); !errors.Is(err, ErrOccurrenceBooked) {
		t.Fatalf("second booking: got %v, want ErrOccurrenceBooked", err)
	}

	// Deleting the booking opens the occurrence again (rule R6) ...
	if err := s.DeleteTransaction(ctx, "l1", txID, "u1", time.Now()); err != nil {
		t.Fatal(err)
	}
	assertDates(t, openDates(t, s, today), "2026-11-20 upcoming")

	// ... and once it is booked again, the old booking cannot come back.
	if _, err := payOccurrence(s, id, "2026-11-20"); err != nil {
		t.Fatalf("book again: %v", err)
	}
	if err := s.RestoreTransaction(ctx, "l1", txID, time.Now()); !errors.Is(err, ErrOccurrenceBooked) {
		t.Fatalf("restore: got %v, want ErrOccurrenceBooked", err)
	}
}

func TestBookingChecksOccurrence(t *testing.T) {
	_, s := setup(t)
	ctx := context.Background()
	id := mustCreateRecurring(t, s, rent())
	archived := mustCreateRecurring(t, s, rent())
	yes := true
	if err := s.UpdateRecurringPayment(ctx, "l1", archived, RecurringPaymentUpdate{Archived: &yes, Today: today}); err != nil {
		t.Fatal(err)
	}

	if _, err := payOccurrence(s, id, "2026-11-21"); !errors.Is(err, ErrNotAnOccurrence) {
		t.Errorf("wrong day: got %v", err)
	}
	if _, err := payOccurrence(s, "nope", "2026-11-20"); !errors.Is(err, ErrRecurringNotFound) {
		t.Errorf("unknown payment: got %v", err)
	}
	if _, err := payOccurrence(s, archived, "2026-11-20"); !errors.Is(err, ErrRecurringArchived) {
		t.Errorf("archived payment: got %v", err)
	}

	in := baseExpense()
	in.RecurringPaymentID = id
	if _, err := s.CreateTransaction(ctx, in, time.Now()); !errors.Is(err, ErrRecurringLinkRequired) {
		t.Errorf("missing due date: got %v", err)
	}
	in.RecurringDueOn = "2026-11-20"
	in.Type, in.CategoryID = TypeIncome, "c-salary"
	if _, err := s.CreateTransaction(ctx, in, time.Now()); !errors.Is(err, ErrRecurringNeedsExpense) {
		t.Errorf("income: got %v", err)
	}
}

func TestLinkedBookingStaysExpense(t *testing.T) {
	_, s := setup(t)
	id := mustCreateRecurring(t, s, rent())
	txID, err := payOccurrence(s, id, "2026-11-20")
	if err != nil {
		t.Fatal(err)
	}
	income, salary := TypeIncome, "c-salary"
	err = s.UpdateTransaction(context.Background(), "l1", txID,
		TransactionPatch{Type: &income, CategoryID: &salary}, time.Now())
	if !errors.Is(err, ErrRecurringNeedsExpense) {
		t.Fatalf("got %v, want ErrRecurringNeedsExpense", err)
	}
}

func TestSkipAndUndo(t *testing.T) {
	_, s := setup(t)
	ctx := context.Background()
	id := mustCreateRecurring(t, s, rent())

	for range 2 { // skipping twice is fine
		if err := s.SkipOccurrence(ctx, "l1", id, "2026-11-20"); err != nil {
			t.Fatalf("skip: %v", err)
		}
	}
	assertDates(t, openDates(t, s, today))
	if _, err := payOccurrence(s, id, "2026-11-20"); !errors.Is(err, ErrOccurrenceSkipped) {
		t.Fatalf("book skipped: got %v", err)
	}

	if err := s.UndoSkip(ctx, "l1", id, "2026-11-20"); err != nil {
		t.Fatalf("undo: %v", err)
	}
	assertDates(t, openDates(t, s, today), "2026-11-20 upcoming")
	if err := s.UndoSkip(ctx, "l1", id, "2026-11-20"); !errors.Is(err, ErrOccurrenceNotSkipped) {
		t.Fatalf("undo twice: got %v", err)
	}

	if _, err := payOccurrence(s, id, "2026-11-20"); err != nil {
		t.Fatal(err)
	}
	if err := s.SkipOccurrence(ctx, "l1", id, "2026-11-20"); !errors.Is(err, ErrOccurrenceBooked) {
		t.Fatalf("skip booked: got %v", err)
	}
	if err := s.SkipOccurrence(ctx, "l1", id, "2026-11-19"); !errors.Is(err, ErrNotAnOccurrence) {
		t.Fatalf("skip wrong day: got %v", err)
	}
}

func TestScheduleChangeRestartsTracking(t *testing.T) {
	_, s := setup(t)
	ctx := context.Background()
	id := mustCreateRecurring(t, s, rent())
	later := "2027-01-10"

	// A new amount keeps the tracking: December stays overdue.
	amount := int64(85000)
	if err := s.UpdateRecurringPayment(ctx, "l1", id, RecurringPaymentUpdate{AmountCents: &amount, Today: later}); err != nil {
		t.Fatal(err)
	}
	assertDates(t, openDates(t, s, later), "2026-11-20 overdue", "2026-12-20 overdue", "2027-01-20 upcoming")

	// A new due day starts tracking anew on that day (rule R5).
	day := 5
	if err := s.UpdateRecurringPayment(ctx, "l1", id, RecurringPaymentUpdate{DueDay: &day, Today: later}); err != nil {
		t.Fatal(err)
	}
	assertDates(t, openDates(t, s, later))
	assertDates(t, openDates(t, s, "2027-02-05"), "2027-02-05 due")
}

func TestUnarchiveRestartsTracking(t *testing.T) {
	_, s := setup(t)
	ctx := context.Background()
	id := mustCreateRecurring(t, s, rent())
	yes, no := true, false
	if err := s.UpdateRecurringPayment(ctx, "l1", id, RecurringPaymentUpdate{Archived: &yes, Today: today}); err != nil {
		t.Fatal(err)
	}
	assertDates(t, openDates(t, s, "2027-01-25"))

	if err := s.UpdateRecurringPayment(ctx, "l1", id, RecurringPaymentUpdate{Archived: &no, Today: "2027-01-25"}); err != nil {
		t.Fatal(err)
	}
	assertDates(t, openDates(t, s, "2027-01-25"))
	assertDates(t, openDates(t, s, "2027-02-20"), "2027-02-20 due")
}

func TestUpdateRecurringPaymentChecksCategory(t *testing.T) {
	db, s := setup(t)
	ctx := context.Background()
	id := mustCreateRecurring(t, s, rent())
	mustExec(t, db, `INSERT INTO category (id, ledger_id, name, archived) VALUES ('c-old', 'l1', 'Old', 1)`)

	for cat, want := range map[string]error{"c-salary": ErrRecurringNeedsExpense, "c-old": ErrCategoryArchived} {
		err := s.UpdateRecurringPayment(ctx, "l1", id, RecurringPaymentUpdate{CategoryID: &cat, Today: today})
		if !errors.Is(err, want) {
			t.Errorf("category %s: got %v, want %v", cat, err, want)
		}
	}
	if err := s.UpdateRecurringPayment(ctx, "other", id, RecurringPaymentUpdate{Today: today}); !errors.Is(err, ErrRecurringNotFound) {
		t.Errorf("other ledger: got %v", err)
	}
}

func TestDeleteCategoryMovesRecurringPayments(t *testing.T) {
	db, s := setup(t)
	ctx := context.Background()
	mustExec(t, db, `INSERT INTO category (id, ledger_id, name) VALUES ('c-rent', 'l1', 'Rent')`)
	in := rent()
	in.CategoryID = "c-rent"
	mustCreateRecurring(t, s, in)

	if _, err := s.DeleteCategory(ctx, "l1", "c-rent", "c-food"); err != nil {
		t.Fatalf("delete category: %v", err)
	}
	items, err := s.ListRecurringPayments(ctx, "l1")
	if err != nil {
		t.Fatal(err)
	}
	if items[0].CategoryID != "c-food" {
		t.Fatalf("payment kept category %s", items[0].CategoryID)
	}
}

func TestRemoveUserDataDeletesRecurringPayments(t *testing.T) {
	db, s := setup(t)
	ctx := context.Background()
	id := mustCreateRecurring(t, s, rent())
	if _, err := payOccurrence(s, id, "2026-11-20"); err != nil {
		t.Fatal(err)
	}
	if err := s.SkipOccurrence(ctx, "l1", id, "2026-12-20"); err != nil {
		t.Fatal(err)
	}

	err := s.inTx(ctx, func(tx *sql.Tx) error { return RemoveUserData(ctx, tx, "u1") })
	if err != nil {
		t.Fatalf("remove user data: %v", err)
	}
	var left int
	if err := db.QueryRow(`SELECT (SELECT count(*) FROM recurring_payment) + (SELECT count(*) FROM recurring_skip)`).Scan(&left); err != nil {
		t.Fatal(err)
	}
	if left != 0 {
		t.Fatalf("%d rows left", left)
	}
}
