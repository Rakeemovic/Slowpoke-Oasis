package ledger

import (
	"context"
	"errors"
	"time"
)

// Errors for choosing a period.
var (
	ErrInvalidMonth  = errors.New("month must have the format YYYY-MM")
	ErrInvalidPeriod = errors.New("give either month, or from and to (not both), and from must not be after to")
)

const dateLayout = "2006-01-02"

// PeriodFor returns the payday period of a monthly pot for the given month
// (format YYYY-MM). The period starts on the payday of that month and ends the
// day before the payday of the following month. A payday that does not exist
// in a month (e.g. the 30th in February) falls back to the last day of that
// month. The result is inclusive: from and to, format YYYY-MM-DD.
func PeriodFor(month string, payday int) (string, string, error) {
	if payday < 1 || payday > 31 {
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
	day := payday
	if day > lastDay {
		day = lastDay
	}
	return time.Date(firstOfMonth.Year(), firstOfMonth.Month(), day, 0, 0, 0, 0, time.UTC)
}

// ResolvePeriod turns the query parameters of a report into a date range.
//
// With month, the range is the payday period of that month in the ledger's
// settings. With from and to, the range is taken as given. If neither is set,
// the range is empty, which means all dates. Empty results mean "no restriction".
func (s *Store) ResolvePeriod(ctx context.Context, ledgerID, month, from, to string) (string, string, error) {
	if month != "" {
		if from != "" || to != "" {
			return "", "", ErrInvalidPeriod
		}
		var payday int
		if err := s.db.QueryRowContext(ctx,
			`SELECT payday_day FROM ledger WHERE id = ?`, ledgerID,
		).Scan(&payday); err != nil {
			return "", "", err
		}
		return PeriodFor(month, payday)
	}

	for _, d := range []string{from, to} {
		if d == "" {
			continue
		}
		if _, err := time.Parse(dateLayout, d); err != nil {
			return "", "", ErrInvalidDate
		}
	}
	if from != "" && to != "" && from > to {
		return "", "", ErrInvalidPeriod
	}
	return from, to, nil
}
