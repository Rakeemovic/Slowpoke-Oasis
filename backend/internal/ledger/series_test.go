package ledger

import (
	"context"
	"database/sql"
	"errors"
	"testing"
	"time"
)

// seriesSetup creates the rent payment with bookings for September to
// November 2026 in c-food, and a category c-rent to move them to. It returns
// the payment and the bookings by due date.
func seriesSetup(t *testing.T) (*sql.DB, *Store, string, map[string]string) {
	t.Helper()
	db, s := setup(t)
	mustExec(t, db, `INSERT INTO category (id, ledger_id, name) VALUES ('c-rent', 'l1', 'Rent')`)
	paymentID := mustCreateRecurring(t, s, rent())
	bookings := map[string]string{}
	for _, due := range []string{"2026-09-20", "2026-10-20", "2026-11-20"} {
		id, err := payOccurrence(s, paymentID, due)
		if err != nil {
			t.Fatalf("book %s: %v", due, err)
		}
		bookings[due] = id
	}
	return db, s, paymentID, bookings
}

// categoriesOf returns the category of each booking by due date, and that of the payment.
func categoriesOf(t *testing.T, db *sql.DB, paymentID string) (map[string]string, string) {
	t.Helper()
	rows, err := db.Query(`SELECT recurring_due_on, category_id FROM transactions WHERE recurring_payment_id = ?`, paymentID)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	out := map[string]string{}
	for rows.Next() {
		var due, cat string
		if err := rows.Scan(&due, &cat); err != nil {
			t.Fatal(err)
		}
		out[due] = cat
	}
	var paymentCategory string
	if err := db.QueryRow(`SELECT category_id FROM recurring_payment WHERE id = ?`, paymentID).Scan(&paymentCategory); err != nil {
		t.Fatal(err)
	}
	return out, paymentCategory
}

func TestCategoryScopes(t *testing.T) {
	cases := []struct {
		scope   CategoryScope
		rent    []string // due dates whose booking moves to c-rent
		payment string   // category of the payment afterwards
	}{
		{"", []string{"2026-10-20"}, "c-food"},
		{ScopeThis, []string{"2026-10-20"}, "c-food"},
		{ScopeThisEarlier, []string{"2026-09-20", "2026-10-20"}, "c-food"},
		{ScopeThisLater, []string{"2026-10-20", "2026-11-20"}, "c-rent"},
		{ScopeWholeSeries, []string{"2026-09-20", "2026-10-20", "2026-11-20"}, "c-rent"},
	}
	for _, c := range cases {
		t.Run(string(c.scope), func(t *testing.T) {
			db, s, paymentID, bookings := seriesSetup(t)
			cat := "c-rent"
			patch := TransactionPatch{CategoryID: &cat, CategoryScope: c.scope, EditorID: "u1"}
			if err := s.UpdateTransaction(context.Background(), "l1", bookings["2026-10-20"], patch, time.Now()); err != nil {
				t.Fatalf("update: %v", err)
			}

			got, paymentCategory := categoriesOf(t, db, paymentID)
			for due, category := range got {
				want := "c-food"
				for _, r := range c.rent {
					if r == due {
						want = "c-rent"
					}
				}
				if category != want {
					t.Errorf("booking %s: category %s, want %s", due, category, want)
				}
			}
			if paymentCategory != c.payment {
				t.Errorf("payment category %s, want %s", paymentCategory, c.payment)
			}
		})
	}
}

func TestCategoryScopeLeavesDeletedBookings(t *testing.T) {
	db, s, paymentID, bookings := seriesSetup(t)
	ctx := context.Background()
	if err := s.DeleteTransaction(ctx, "l1", bookings["2026-09-20"], "u1", time.Now()); err != nil {
		t.Fatal(err)
	}
	cat := "c-rent"
	patch := TransactionPatch{CategoryID: &cat, CategoryScope: ScopeWholeSeries, EditorID: "u1"}
	if err := s.UpdateTransaction(ctx, "l1", bookings["2026-11-20"], patch, time.Now()); err != nil {
		t.Fatal(err)
	}
	got, _ := categoriesOf(t, db, paymentID)
	if got["2026-09-20"] != "c-food" || got["2026-10-20"] != "c-rent" {
		t.Fatalf("categories = %v", got)
	}
}

func TestCategoryScopeIsChecked(t *testing.T) {
	_, s, _, bookings := seriesSetup(t)
	ctx := context.Background()
	plain, err := s.CreateTransaction(ctx, baseExpense(), time.Now())
	if err != nil {
		t.Fatal(err)
	}
	rentCat, salary := "c-rent", "c-salary"
	note := "x"

	cases := []struct {
		name  string
		id    string
		patch TransactionPatch
		want  error
	}{
		{"booking without series", plain, TransactionPatch{CategoryID: &rentCat, CategoryScope: ScopeWholeSeries}, ErrInvalidCategoryScope},
		{"no category", bookings["2026-10-20"], TransactionPatch{Note: &note, CategoryScope: ScopeWholeSeries}, ErrInvalidCategoryScope},
		{"unknown scope", bookings["2026-10-20"], TransactionPatch{CategoryID: &rentCat, CategoryScope: "some"}, ErrInvalidCategoryScope},
		{"income category", bookings["2026-10-20"], TransactionPatch{CategoryID: &salary, CategoryScope: ScopeWholeSeries}, ErrCategoryKindMismatch},
	}
	for _, c := range cases {
		c.patch.EditorID = "u1"
		if err := s.UpdateTransaction(ctx, "l1", c.id, c.patch, time.Now()); !errors.Is(err, c.want) {
			t.Errorf("%s: got %v, want %v", c.name, err, c.want)
		}
	}
}
