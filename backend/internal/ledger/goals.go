package ledger

import (
	"context"
	"sort"
	"time"
)

// SavingsGoal is what a pot with a savings goal should receive in the
// current payday period (rule T8). A pot saves towards a target amount by a
// date, or a fixed rate per period with an optional target. A pot with only
// a target amount has no rate and is not a SavingsGoal.
type SavingsGoal struct {
	Pot Pot
	// TargetMonth is the month of the payday period the target date lies in;
	// Periods counts the periods until then, the current one included. Both
	// are empty for a fixed rate.
	TargetMonth string
	Periods     int
	// SavedCents is what transfers in this period moved into the pot, as far
	// as it is still there (as for reserves, rule R14).
	SavedCents int64
	// RateCents is the rate of this period: for a target date what is missing
	// without this period's savings, divided by Periods and rounded up to the
	// cent; for a fixed rate the rate, but never more than the target lacks.
	RateCents int64
	// OpenCents is the part of the rate not saved yet (never below zero).
	OpenCents int64
	// Reached means the pot holds its target amount.
	Reached bool
	// DatePassed means the target date lies before today: the rest is due now.
	DatePassed bool
}

// hasRate reports whether a pot saves a rate: it has a target date or a fixed
// rate. The main pot and reserves never do (rule T7).
func (p Pot) hasRate() bool {
	return !p.IsDefault && p.ReserveFor == "" && !p.Archived && (p.TargetDate != "" || p.MonthlyRateCents > 0)
}

// SavingsGoals returns the rates of the current payday period for the pots
// of a ledger that save towards a goal, ordered by name. today is the app's
// local date. Like reserve rates, they correct themselves: whatever was
// saved too little or too much changes the rates of the following periods.
func (s *Store) SavingsGoals(ctx context.Context, ledgerID, today string) ([]SavingsGoal, error) {
	cur, err := currentPeriod(ctx, s.db, ledgerID, today)
	if err != nil {
		return nil, err
	}
	pots, err := s.ListPots(ctx, ledgerID)
	if err != nil {
		return nil, err
	}

	out := []SavingsGoal{}
	for _, p := range pots {
		if !p.hasRate() {
			continue
		}
		g, err := s.savingsGoal(ctx, p, cur, today)
		if err != nil {
			return nil, err
		}
		out = append(out, g)
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Pot.Name < out[j].Pot.Name })
	return out, nil
}

func (s *Store) savingsGoal(ctx context.Context, p Pot, cur period, today string) (SavingsGoal, error) {
	g := SavingsGoal{Pot: p, Reached: p.TargetCents > 0 && p.BalanceCents >= p.TargetCents}
	_, moved, err := potMovements(ctx, s.db, p.ID, cur.From, cur.To)
	if err != nil {
		return SavingsGoal{}, err
	}
	g.SavedCents = min(moved, p.BalanceCents)
	// What the pot held before this period's savings; the rate is planned
	// from there, so saving during the period does not shrink it.
	basis := p.BalanceCents - g.SavedCents

	if p.TargetDate != "" {
		if g.TargetMonth, err = MonthOf(p.TargetDate, cur.Payday); err != nil {
			return SavingsGoal{}, err
		}
		// A passed date leaves one period: the rest is due now.
		g.Periods = periodsUntil(cur.Month, g.TargetMonth)
		g.DatePassed = p.TargetDate < today
		g.RateCents = reserveRate(p.TargetCents, basis, g.Periods)
	} else {
		g.RateCents = fixedRate(p.MonthlyRateCents, p.TargetCents, basis)
	}
	g.OpenCents = max(0, g.RateCents-g.SavedCents)
	return g, nil
}

// fixedRate is a fixed rate, limited to what the target still lacks when the
// pot has a target.
func fixedRate(rate, target, basis int64) int64 {
	if target == 0 {
		return rate
	}
	return min(rate, max(0, target-basis))
}

// SavingsLine is the rate of a savings goal in a month of the budget
// (rule P11). PlannedCents is nil in past months, whose rates were never
// stored; future months are a preview.
type SavingsLine struct {
	PotID        string
	Name         string
	PlannedCents *int64
	ActualCents  int64
	Preview      bool
}

// savingsLines returns the savings goals of a month of the budget: the real
// rates for the current month, a preview for future months, and for past
// months only what was transferred.
func (s *Store) savingsLines(ctx context.Context, ledgerID, today string, p planPeriod) ([]SavingsLine, error) {
	if p.Month < p.Current {
		return s.pastSavingsLines(ctx, ledgerID, p)
	}
	goals, err := s.SavingsGoals(ctx, ledgerID, today)
	if err != nil {
		return nil, err
	}
	out := []SavingsLine{}
	for _, g := range goals {
		line := SavingsLine{PotID: g.Pot.ID, Name: g.Pot.Name, Preview: p.Month > p.Current}
		rate := g.RateCents
		if line.Preview {
			var ok bool
			if rate, ok, err = previewRate(g, p.Month, p.Current); err != nil {
				return nil, err
			}
			if !ok {
				continue
			}
		} else {
			line.ActualCents = g.SavedCents
		}
		if rate == 0 && line.ActualCents == 0 {
			continue
		}
		line.PlannedCents = &rate
		out = append(out, line)
	}
	return out, nil
}

// previewRate assumes that the current rate is saved as planned in every
// period until month. A target date keeps its rate until the target month
// and has none afterwards; a fixed rate ends once the target is reached.
func previewRate(g SavingsGoal, month, current string) (int64, bool, error) {
	if g.TargetMonth != "" {
		return g.RateCents, g.RateCents > 0 && month <= g.TargetMonth, nil
	}
	a, err := time.Parse(monthLayout, current)
	if err != nil {
		return 0, false, err
	}
	b, err := time.Parse(monthLayout, month)
	if err != nil {
		return 0, false, err
	}
	projected := g.Pot.BalanceCents - g.SavedCents + g.RateCents*int64(monthsBetween(a, b))
	rate := fixedRate(g.Pot.MonthlyRateCents, g.Pot.TargetCents, projected)
	return rate, rate > 0, nil
}

// pastSavingsLines shows what moved into the goal pots in a past month.
func (s *Store) pastSavingsLines(ctx context.Context, ledgerID string, p planPeriod) ([]SavingsLine, error) {
	pots, err := s.ListPots(ctx, ledgerID)
	if err != nil {
		return nil, err
	}
	out := []SavingsLine{}
	for _, pot := range pots {
		if pot.IsDefault || pot.ReserveFor != "" || (pot.TargetCents == 0 && pot.MonthlyRateCents == 0) {
			continue
		}
		_, saved, err := potMovements(ctx, s.db, pot.ID, p.From, p.To)
		if err != nil {
			return nil, err
		}
		if saved != 0 {
			out = append(out, SavingsLine{PotID: pot.ID, Name: pot.Name, ActualCents: saved})
		}
	}
	return out, nil
}
