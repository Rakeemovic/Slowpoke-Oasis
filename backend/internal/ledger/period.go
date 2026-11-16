package ledger

import (
	"context"
	"database/sql"
	"errors"
	"time"
)

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
	first, err := time.Parse("2006-01", month)
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
		var payday int
		err := s.db.QueryRowContext(ctx,
			`SELECT payday_day FROM ledger WHERE id = ?`, ledgerID,
		).Scan(&payday)
		if errors.Is(err, sql.ErrNoRows) {
			return "", "", ErrLedgerNotFound
		}
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
