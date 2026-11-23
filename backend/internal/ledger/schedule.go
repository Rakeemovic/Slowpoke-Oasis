package ledger

import (
	"errors"
	"time"
)

// Errors for the schedule of a recurring payment.
var (
	ErrInvalidInterval   = errors.New("interval_months must be 1, 3, 6 or 12")
	ErrInvalidDueDay     = errors.New("due_day must be a day between 1 and 31")
	ErrInvalidMonthRange = errors.New("last_month must not be before first_month")
)

// Schedule says when a recurring payment is due (rules R3, R4): every
// IntervalMonths months on DueDay, starting in FirstMonth and ending with
// LastMonth (empty: no end). Months have the format YYYY-MM. A due day that a
// month does not have falls back to the month's last day.
type Schedule struct {
	IntervalMonths int
	DueDay         int
	FirstMonth     string
	LastMonth      string
}

// validate checks the schedule (rules R3, R4).
func (s Schedule) validate() error {
	switch s.IntervalMonths {
	case 1, 3, 6, 12:
	default:
		return ErrInvalidInterval
	}
	if s.DueDay < 1 || s.DueDay > 31 {
		return ErrInvalidDueDay
	}
	if _, err := time.Parse(monthLayout, s.FirstMonth); err != nil {
		return ErrInvalidMonth
	}
	if s.LastMonth != "" {
		if _, err := time.Parse(monthLayout, s.LastMonth); err != nil {
			return ErrInvalidMonth
		}
		if s.LastMonth < s.FirstMonth {
			return ErrInvalidMonthRange
		}
	}
	return nil
}

// sameDates reports whether both schedules have the same due dates. Only the
// end may differ: ending a payment earlier or later does not move its dates.
func (s Schedule) sameDates(o Schedule) bool {
	return s.IntervalMonths == o.IntervalMonths && s.DueDay == o.DueDay && s.FirstMonth == o.FirstMonth
}

// DueDates returns the due dates (YYYY-MM-DD) from from to to, both
// inclusive, oldest first. The schedule must be valid.
func (s Schedule) DueDates(from, to string) []string {
	first, _ := time.Parse(monthLayout, s.FirstMonth)
	fromDate, err1 := time.Parse(dateLayout, from)
	toDate, err2 := time.Parse(dateLayout, to)
	if err1 != nil || err2 != nil {
		return nil
	}

	// Skip the months before from in whole intervals instead of walking
	// through them: a due date always lies in its own month.
	month := first
	if gap := monthsBetween(first, fromDate); gap > 0 {
		month = first.AddDate(0, gap/s.IntervalMonths*s.IntervalMonths, 0)
	}

	var dates []string
	for ; s.LastMonth == "" || month.Format(monthLayout) <= s.LastMonth; month = month.AddDate(0, s.IntervalMonths, 0) {
		due := dayIn(month, s.DueDay)
		if due.After(toDate) {
			break
		}
		if !due.Before(fromDate) {
			dates = append(dates, due.Format(dateLayout))
		}
	}
	return dates
}

// IsDueOn reports whether date (YYYY-MM-DD) is one of the due dates.
func (s Schedule) IsDueOn(date string) bool {
	dates := s.DueDates(date, date)
	return len(dates) == 1
}

// monthsBetween returns the number of whole calendar months from the month of
// a to the month of b (negative if b is earlier).
func monthsBetween(a, b time.Time) int {
	return (b.Year()-a.Year())*12 + int(b.Month()) - int(a.Month())
}

// addMonths moves a valid date (YYYY-MM-DD) by n months. A day the target
// month lacks overflows into the next month, which is fine for range ends.
func addMonths(date string, n int) string {
	d, _ := time.Parse(dateLayout, date)
	return d.AddDate(0, n, 0).Format(dateLayout)
}
