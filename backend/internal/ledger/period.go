package ledger

import (
	"context"
	"database/sql"
	"errors"
	"time"
)

// monthLayout is the format of a month, as used by monthly plans and reports.
const monthLayout = "2006-01"

// Errors for choosing a period.
var (
	ErrInvalidMonth  = errors.New("month must have the format YYYY-MM")
	ErrInvalidPeriod = errors.New("give either month, or from and to (not both), and from must not be after to")
)

// PeriodFor returns the payday period of a monthly pot for the given month
// (format YYYY-MM). The period starts on the payday of that month and ends the
// day before the payday of the following month. A payday that does not exist
// in a month (e.g. the 30th in February) falls back to the last day of that
// month. The result is inclusive: from and to, format YYYY-MM-DD.
func PeriodFor(month string, payday int) (string, string, error) {
	if !validPayday(payday) {
		return "", "", ErrInvalidPayday
	}
	first, err := time.Parse(monthLayout, month)
	if err != nil {
		return "", "", ErrInvalidMonth
	}
	start := paydayIn(first, payday)
	nextStart := paydayIn(first.AddDate(0, 1, 0), payday)
	return start.Format(dateLayout), nextStart.AddDate(0, 0, -1).Format(dateLayout), nil
}

// paydayIn returns the payday of the month that starts at firstOfMonth.
func paydayIn(firstOfMonth time.Time, payday int) time.Time {
	lastDay := firstOfMonth.AddDate(0, 1, -1).Day()
	return time.Date(firstOfMonth.Year(), firstOfMonth.Month(), min(payday, lastDay), 0, 0, 0, 0, time.UTC)
}

// MonthOf returns the month (YYYY-MM) of the payday period that contains the
// given date (YYYY-MM-DD). Before the payday, a date still belongs to the
// period that started in the previous month.
func MonthOf(date string, payday int) (string, error) {
	if !validPayday(payday) {
		return "", ErrInvalidPayday
	}
	d, err := time.Parse(dateLayout, date)
	if err != nil {
		return "", ErrInvalidDate
	}
	first := time.Date(d.Year(), d.Month(), 1, 0, 0, 0, 0, time.UTC)
	if d.Before(paydayIn(first, payday)) {
		first = first.AddDate(0, -1, 0)
	}
	return first.Format(monthLayout), nil
}

// Today returns date if it is set and valid, else the date of now. The app
// sends its local date, because the server may run in another time zone.
func Today(date string, now time.Time) (string, error) {
	if date == "" {
		return now.Format(dateLayout), nil
	}
	if !validDate(date) {
		return "", ErrInvalidDate
	}
	return date, nil
}

// ledgerPayday returns the payday of a ledger.
func ledgerPayday(ctx context.Context, q queryer, ledgerID string) (int, error) {
	var payday int
	err := q.QueryRowContext(ctx,
		`SELECT payday_day FROM ledger WHERE id = ?`, ledgerID,
	).Scan(&payday)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, ErrLedgerNotFound
	}
	return payday, err
}

// ResolvePeriod turns the query parameters of a report into a date range.
//
// With month, the range is the payday period of that month in the ledger's
// settings. With from and to, the range is taken as given. An empty from or to
// in the result means the range is not limited on that side.
func (s *Store) ResolvePeriod(ctx context.Context, ledgerID, month, from, to string) (string, string, error) {
	if month != "" {
		if from != "" || to != "" {
			return "", "", ErrInvalidPeriod
		}
		payday, err := ledgerPayday(ctx, s.db, ledgerID)
		if err != nil {
			return "", "", err
		}
		return PeriodFor(month, payday)
	}

	if err := validateRange(from, to); err != nil {
		return "", "", err
	}
	if from != "" && to != "" && from > to {
		return "", "", ErrInvalidPeriod
	}
	return from, to, nil
}

// validateRange checks the dates of an inclusive date range. An empty date
// means the range is not limited on that side.
func validateRange(from, to string) error {
	for _, d := range []string{from, to} {
		if d != "" && !validDate(d) {
			return ErrInvalidDate
		}
	}
	return nil
}

// rangeClause returns the SQL condition for an inclusive booked_on range, to
// be appended to a WHERE clause, and its arguments.
func rangeClause(from, to string) (string, []any) {
	clause := ""
	var args []any
	if from != "" {
		clause += ` AND booked_on >= ?`
		args = append(args, from)
	}
	if to != "" {
		clause += ` AND booked_on <= ?`
		args = append(args, to)
	}
	return clause, args
}
