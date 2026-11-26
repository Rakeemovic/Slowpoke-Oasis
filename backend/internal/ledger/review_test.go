package ledger

import (
	"context"
	"errors"
	"testing"
	"time"
)

func reviewOf(t *testing.T, s *Store, userID, today string) WeeklyReview {
	t.Helper()
	r, err := s.WeeklyReview(context.Background(), "l1", userID, today)
	if err != nil {
		t.Fatalf("review: %v", err)
	}
	return r
}

func completeReview(t *testing.T, s *Store, today string) DayRange {
	t.Helper()
	days, err := s.CompleteWeeklyReview(context.Background(), "l1", "u1", today, time.Now())
	if err != nil {
		t.Fatalf("complete on %s: %v", today, err)
	}
	return days
}

// November 2026: the 16th, 23rd and 30th are Mondays.
func TestReviewDays(t *testing.T) {
	tests := []struct {
		name    string
		today   string
		weekday int
		last    string
		want    DayRange // empty if nothing is due
	}{
		{"first one on the day", "2026-11-23", 1, "", DayRange{"2026-11-16", "2026-11-22"}},
		{"first one later in the week", "2026-11-25", 1, "", DayRange{"2026-11-18", "2026-11-24"}},
		{"a week after the last", "2026-11-23", 1, "2026-11-15", DayRange{"2026-11-16", "2026-11-22"}},
		{"only the days since a late one", "2026-11-23", 1, "2026-11-18", DayRange{"2026-11-19", "2026-11-22"}},
		{"left out weeks are not caught up", "2026-11-23", 1, "2026-11-01", DayRange{"2026-11-16", "2026-11-22"}},
		{"done on the day", "2026-11-23", 1, "2026-11-22", DayRange{}},
		{"done, later in the week", "2026-11-29", 1, "2026-11-22", DayRange{}},
		{"done late, same week", "2026-11-26", 1, "2026-11-24", DayRange{}},
		{"missed day stays open", "2026-11-29", 1, "2026-11-15", DayRange{"2026-11-22", "2026-11-28"}},
		{"Sunday as reflection day", "2026-11-29", 7, "2026-11-21", DayRange{"2026-11-22", "2026-11-28"}},
		{"before Sunday comes again", "2026-11-28", 7, "2026-11-21", DayRange{}},
	}
	for _, tt := range tests {
		today, _ := time.Parse(dateLayout, tt.today)
		got, due := reviewDays(today, tt.weekday, tt.last)
		if got != tt.want || due != (tt.want != DayRange{}) {
			t.Errorf("%s: got %+v (due %v), want %+v", tt.name, got, due, tt.want)
		}
	}
}

func TestWeeklyReviewShowsOnlyOwnSpending(t *testing.T) {
	db, s := setup(t)
	mustExec(t, db, `INSERT INTO user (id, name, created_at) VALUES ('u2', 'Bob', '2026-01-01T00:00:00Z')`)
	want := purposeCategory(t, s, "Cinema", PurposeWant)
	own := spend(t, s, want, "2026-11-17", 5000, true)
	spend(t, s, want, "2026-11-10", 2000, false) // the week before
	spend(t, s, want, "2026-11-23", 900, false)  // today is not covered
	spend(t, s, "c-food", "2026-11-18", 1000, false)
	other := baseExpense()
	other.UserID, other.CategoryID, other.BookedOn = "u2", want, "2026-11-17"
	if _, err := s.CreateTransaction(context.Background(), other, time.Now()); err != nil {
		t.Fatal(err)
	}
	moveToPot(t, s, "p-default", "p-holiday", 3000, "2026-11-19")

	r := reviewOf(t, s, "u1", "2026-11-23")
	if !r.Due || r.Covered != (DayRange{"2026-11-16", "2026-11-22"}) || r.Previous != (DayRange{"2026-11-09", "2026-11-15"}) {
		t.Fatalf("review = %+v", r)
	}
	lines := PurposeReport{Lines: r.Lines}
	if l := purposeLine(t, lines, PurposeWant); l.Cents != 5000 || l.SpontaneousCents != 5000 || l.PreviousCents != 2000 || l.SharePercent != nil {
		t.Errorf("want = %+v", l)
	}
	if l := purposeLine(t, lines, PurposeSaving); l.TransferCents != 3000 {
		t.Errorf("saving = %+v", l)
	}
	if l := purposeLine(t, lines, ""); l.Cents != 1000 {
		t.Errorf("none = %+v", l)
	}
	if len(r.Wants) != 1 || r.Wants[0].ID != own {
		t.Errorf("wants = %+v, want only the own one of the week", r.Wants)
	}
	// The other member sees their own booking and none of u1's transfers.
	theirs := reviewOf(t, s, "u2", "2026-11-23")
	if len(theirs.Wants) != 1 || purposeLine(t, PurposeReport{Lines: theirs.Lines}, PurposeSaving).Cents != 0 {
		t.Errorf("review of u2 = %+v", theirs)
	}
}

func TestCompletingClosesTheReviewUntilTheNextDay(t *testing.T) {
	_, s := setup(t)
	if days := completeReview(t, s, "2026-11-25"); days != (DayRange{"2026-11-18", "2026-11-24"}) {
		t.Errorf("covered = %+v", days)
	}
	if reviewOf(t, s, "u1", "2026-11-29").Due {
		t.Error("still due before the next Monday")
	}
	if _, err := s.CompleteWeeklyReview(context.Background(), "l1", "u1", "2026-11-25", time.Now()); !errors.Is(err, ErrReviewNotDue) {
		t.Errorf("second time: err = %v, want ErrReviewNotDue", err)
	}
	if days := completeReview(t, s, "2026-11-30"); days != (DayRange{"2026-11-25", "2026-11-29"}) {
		t.Errorf("next one covers %+v", days)
	}
}

func TestWeeklyReviewUsesTheChosenWeekday(t *testing.T) {
	db, s := setup(t)
	completeReview(t, s, "2026-11-23")
	mustExec(t, db, `UPDATE user SET review_weekday = 4 WHERE id = 'u1'`)
	r := reviewOf(t, s, "u1", "2026-11-26") // a Thursday
	if r.Covered != (DayRange{"2026-11-23", "2026-11-25"}) || r.Previous != (DayRange{"2026-11-20", "2026-11-22"}) {
		t.Errorf("review = %+v", r)
	}
}

func TestWeeklyReviewRejectsABadDate(t *testing.T) {
	_, s := setup(t)
	if _, err := s.WeeklyReview(context.Background(), "l1", "u1", "23.11.2026"); !errors.Is(err, ErrInvalidDate) {
		t.Errorf("err = %v, want ErrInvalidDate", err)
	}
}

func TestLeavingAHouseholdRemovesTheReviews(t *testing.T) {
	db, s := setup(t)
	ctx := context.Background()
	mustExec(t, db, `INSERT INTO user (id, name, created_at) VALUES ('u2', 'Bob', '2026-01-01T00:00:00Z')`)
	mustExec(t, db, `INSERT INTO household (id, name, created_at) VALUES ('h1', 'Flat', '2026-01-01T00:00:00Z')`)
	mustExec(t, db, `INSERT INTO household_member (household_id, user_id, role, joined_at) VALUES
		('h1', 'u1', 'owner', '2026-01-01T00:00:00Z'), ('h1', 'u2', 'member', '2026-01-02T00:00:00Z')`)
	mustExec(t, db, `INSERT INTO ledger (id, name, household_id, created_at) VALUES ('l-flat', 'Flat', 'h1', '2026-01-01T00:00:00Z')`)
	for _, ledgerID := range []string{"l1", "l-flat"} {
		if _, err := s.CompleteWeeklyReview(ctx, ledgerID, "u1", "2026-11-23", time.Now()); err != nil {
			t.Fatal(err)
		}
	}
	if err := s.LeaveHousehold(ctx, "h1", "u1"); err != nil {
		t.Fatal(err)
	}
	var left string
	if err := db.QueryRow(`SELECT group_concat(ledger_id) FROM weekly_review`).Scan(&left); err != nil {
		t.Fatal(err)
	}
	if left != "l1" {
		t.Errorf("reviews left for %q, want only the personal ledger", left)
	}
}
