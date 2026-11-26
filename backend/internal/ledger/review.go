package ledger

import (
	"context"
	"database/sql"
	"errors"
	"time"
)

// reviewMaxDays is the longest time one reflection looks back (rule W2).
// Weeks that were left out are not caught up.
const reviewMaxDays = 7

// ErrReviewNotDue is returned when a reflection is completed although none is open.
var ErrReviewNotDue = errors.New("no weekly review is due")

// DayRange is a span of days, both ends included, as YYYY-MM-DD.
type DayRange struct {
	From  string
	Until string
}

// WeeklyReview is the open reflection of one member about one ledger.
type WeeklyReview struct {
	// Due says whether a reflection is open; the other fields are only set then.
	Due     bool
	Covered DayRange
	// Previous is the span of the same length right before Covered.
	Previous DayRange
	// Lines has the member's own spending like a PurposeReport, without shares.
	Lines []PurposeLine
	// Wants are the member's own expenses of the covered days in categories
	// with the purpose want, which they may mark as spontaneous (rule X20).
	Wants []Transaction
}

// WeeklyReview returns the reflection of a user about a ledger. today is the
// app's local date. The caller checks that the user may see the ledger.
func (s *Store) WeeklyReview(ctx context.Context, ledgerID, userID, today string) (WeeklyReview, error) {
	covered, due, err := openReviewDays(ctx, s.db, ledgerID, userID, today)
	if err != nil || !due {
		return WeeklyReview{}, err
	}
	out := WeeklyReview{Due: true, Covered: covered, Previous: spanBefore(covered)}

	now, err := sumPurposes(ctx, s.db, purposeScope{ledgerID, userID, covered})
	if err != nil {
		return WeeklyReview{}, err
	}
	before, err := sumPurposes(ctx, s.db, purposeScope{ledgerID, userID, out.Previous})
	if err != nil {
		return WeeklyReview{}, err
	}
	out.Lines = purposeLines(now, before)
	out.Wants, err = ownWants(ctx, s.db, purposeScope{ledgerID, userID, covered})
	return out, err
}

// CompleteWeeklyReview records that a user finished the open reflection and
// returns the days it covered. It fails with ErrReviewNotDue if none is open.
func (s *Store) CompleteWeeklyReview(ctx context.Context, ledgerID, userID, today string, now time.Time) (DayRange, error) {
	var covered DayRange
	err := s.inTx(ctx, func(tx *sql.Tx) error {
		days, due, err := openReviewDays(ctx, tx, ledgerID, userID, today)
		if err != nil {
			return err
		}
		if !due {
			return ErrReviewNotDue
		}
		covered = days
		_, err = tx.ExecContext(ctx, `
			INSERT INTO weekly_review (ledger_id, user_id, covered_from, covered_until, completed_at)
			VALUES (?, ?, ?, ?, ?)`,
			ledgerID, userID, days.From, days.Until, timestamp(now),
		)
		return err
	})
	return covered, err
}

// openReviewDays looks up the reflection day of a user and what they reviewed
// last, and returns the days the open reflection covers, if one is open.
func openReviewDays(ctx context.Context, q queryer, ledgerID, userID, today string) (DayRange, bool, error) {
	day, err := time.Parse(dateLayout, today)
	if err != nil {
		return DayRange{}, false, ErrInvalidDate
	}
	var weekday int
	var lastUntil string
	if err := q.QueryRowContext(ctx, `
		SELECT review_weekday,
		       COALESCE((SELECT MAX(covered_until) FROM weekly_review
		                 WHERE ledger_id = ? AND user_id = user.id), '')
		FROM user WHERE id = ?`, ledgerID, userID,
	).Scan(&weekday, &lastUntil); err != nil {
		return DayRange{}, false, err
	}
	days, due := reviewDays(day, weekday, lastUntil)
	return days, due, nil
}

// reviewDays applies rules W1 and W2. A reflection is open from the latest
// reflection day on until the days before it are covered. It covers the days
// since the last one up to yesterday, at most reviewMaxDays. lastUntil is
// empty if the user never completed one.
func reviewDays(today time.Time, weekday int, lastUntil string) (DayRange, bool) {
	// time.Weekday counts Sunday as 0; modulo 7 that equals ISO's 7.
	daysSince := (int(today.Weekday()) - weekday%7 + 7) % 7
	dayBeforeReviewDay := today.AddDate(0, 0, -daysSince-1).Format(dateLayout)
	if lastUntil >= dayBeforeReviewDay {
		return DayRange{}, false
	}
	from := today.AddDate(0, 0, -reviewMaxDays)
	if last, err := time.Parse(dateLayout, lastUntil); err == nil && !last.Before(from) {
		from = last.AddDate(0, 0, 1)
	}
	return DayRange{
		From:  from.Format(dateLayout),
		Until: today.AddDate(0, 0, -1).Format(dateLayout),
	}, true
}

// spanBefore returns the span of the same length that ends the day before r.
func spanBefore(r DayRange) DayRange {
	from, _ := time.Parse(dateLayout, r.From)
	until, _ := time.Parse(dateLayout, r.Until)
	days := int(until.Sub(from).Hours()/24) + 1
	return DayRange{
		From:  from.AddDate(0, 0, -days).Format(dateLayout),
		Until: from.AddDate(0, 0, -1).Format(dateLayout),
	}
}

// ownWants lists the expenses of a user in categories with the purpose want.
func ownWants(ctx context.Context, q queryer, sc purposeScope) ([]Transaction, error) {
	rows, err := q.QueryContext(ctx, `
		SELECT `+transactionColumns+`
		FROM `+transactionTables+`
		JOIN category cat ON cat.id = t.category_id AND cat.ledger_id = t.ledger_id
		WHERE t.ledger_id = ? AND t.user_id = ? AND t.deleted_at IS NULL
		  AND t.type = 'expense' AND cat.purpose = ? AND t.booked_on BETWEEN ? AND ?
		ORDER BY t.booked_on DESC, t.created_at DESC`,
		sc.ledgerID, sc.userID, PurposeWant, sc.days.From, sc.days.Until)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []Transaction
	for rows.Next() {
		var t Transaction
		if err := rows.Scan(t.scanTargets()...); err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}
