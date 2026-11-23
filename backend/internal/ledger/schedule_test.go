package ledger

import (
	"errors"
	"slices"
	"testing"
)

func TestScheduleDueDates(t *testing.T) {
	cases := []struct {
		name     string
		schedule Schedule
		from, to string
		want     []string
	}{
		{"monthly", Schedule{1, 3, "2026-10", ""}, "2026-10-01", "2026-12-31",
			[]string{"2026-10-03", "2026-11-03", "2026-12-03"}},
		{"range limits are inclusive", Schedule{1, 3, "2026-01", ""}, "2026-11-03", "2026-12-03",
			[]string{"2026-11-03", "2026-12-03"}},
		{"due day falls back to the last day", Schedule{1, 31, "2027-01", ""}, "2027-01-01", "2027-04-30",
			[]string{"2027-01-31", "2027-02-28", "2027-03-31", "2027-04-30"}},
		{"quarterly counts from the first month", Schedule{3, 15, "2025-02", ""}, "2026-01-01", "2026-12-31",
			[]string{"2026-02-15", "2026-05-15", "2026-08-15", "2026-11-15"}},
		{"yearly", Schedule{12, 1, "2020-06", ""}, "2026-01-01", "2027-12-31",
			[]string{"2026-06-01", "2027-06-01"}},
		{"ends with the last month", Schedule{1, 10, "2026-10", "2026-11"}, "2026-01-01", "2027-12-31",
			[]string{"2026-10-10", "2026-11-10"}},
		{"nothing before the first month", Schedule{1, 10, "2026-12", ""}, "2026-01-01", "2026-11-30", nil},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := c.schedule.DueDates(c.from, c.to); !slices.Equal(got, c.want) {
				t.Errorf("DueDates(%s, %s) = %v, want %v", c.from, c.to, got, c.want)
			}
		})
	}
}

func TestScheduleIsDueOn(t *testing.T) {
	s := Schedule{3, 31, "2026-02", ""}
	for date, want := range map[string]bool{
		"2026-02-28": true,  // falls back to the last day of February
		"2026-05-31": true,  // three months later
		"2026-03-31": false, // not in the rhythm
		"2026-05-30": false, // wrong day
		"2025-11-30": false, // before the first month
	} {
		if got := s.IsDueOn(date); got != want {
			t.Errorf("IsDueOn(%s) = %v, want %v", date, got, want)
		}
	}
}

func TestScheduleValidate(t *testing.T) {
	cases := []struct {
		schedule Schedule
		want     error
	}{
		{Schedule{1, 1, "2026-11", ""}, nil},
		{Schedule{12, 31, "2026-11", "2026-11"}, nil},
		{Schedule{2, 1, "2026-11", ""}, ErrInvalidInterval},
		{Schedule{1, 0, "2026-11", ""}, ErrInvalidDueDay},
		{Schedule{1, 32, "2026-11", ""}, ErrInvalidDueDay},
		{Schedule{1, 1, "2026-13", ""}, ErrInvalidMonth},
		{Schedule{1, 1, "2026-11", "nope"}, ErrInvalidMonth},
		{Schedule{1, 1, "2026-11", "2026-10"}, ErrInvalidMonthRange},
	}
	for _, c := range cases {
		if err := c.schedule.validate(); !errors.Is(err, c.want) {
			t.Errorf("validate(%+v) = %v, want %v", c.schedule, err, c.want)
		}
	}
}

func TestNextDueOn(t *testing.T) {
	p := RecurringPayment{Schedule: Schedule{3, 20, "2026-08", "2027-02"}}
	for today, want := range map[string]string{
		"2026-11-16": "2026-11-20",
		"2026-11-20": "2026-11-20",
		"2026-11-21": "2027-02-20",
		"2027-02-21": "",           // ended
		"2026-01-01": "2026-08-20", // long before the first month
	} {
		if got := p.NextDueOn(today); got != want {
			t.Errorf("NextDueOn(%s) = %q, want %q", today, got, want)
		}
	}
}
