package ledger

import (
	"errors"
	"testing"
)

func TestPeriodForFollowsPayday(t *testing.T) {
	cases := []struct {
		month    string
		payday   int
		from, to string
	}{
		{"2026-11", 23, "2026-11-23", "2026-12-22"},
		{"2026-11", 1, "2026-11-01", "2026-11-30"},
		{"2026-02", 30, "2026-02-28", "2026-03-29"},
		{"2026-12", 31, "2026-12-31", "2027-01-30"},
	}
	for _, c := range cases {
		from, to, err := PeriodFor(c.month, c.payday)
		if err != nil {
			t.Errorf("PeriodFor(%s, %d): %v", c.month, c.payday, err)
			continue
		}
		if from != c.from || to != c.to {
			t.Errorf("PeriodFor(%s, %d) = %s..%s, want %s..%s", c.month, c.payday, from, to, c.from, c.to)
		}
	}
}

func TestResolvePeriodRejectsConflictingInput(t *testing.T) {
	_, s := setup(t)
	ctx := t.Context()

	if _, _, err := s.ResolvePeriod(ctx, "l1", "2026-11", "2026-11-01", ""); !errors.Is(err, ErrInvalidPeriod) {
		t.Errorf("month with from: err = %v, want ErrInvalidPeriod", err)
	}
	if _, _, err := s.ResolvePeriod(ctx, "l1", "", "2026-12-01", "2026-11-01"); !errors.Is(err, ErrInvalidPeriod) {
		t.Errorf("from after to: err = %v, want ErrInvalidPeriod", err)
	}
	if _, _, err := s.ResolvePeriod(ctx, "l1", "2026-13", "", ""); !errors.Is(err, ErrInvalidMonth) {
		t.Errorf("month 2026-13: err = %v, want ErrInvalidMonth", err)
	}
	from, to, err := s.ResolvePeriod(ctx, "l1", "", "", "")
	if err != nil || from != "" || to != "" {
		t.Errorf("no range: %q..%q, %v; want empty range", from, to, err)
	}
}
