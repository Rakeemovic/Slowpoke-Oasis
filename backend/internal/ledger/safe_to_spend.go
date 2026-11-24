package ledger

import (
	"context"
	"time"
)

// SafeToSpend is what the main pot can still spend freely until the end of
// the current payday period (rule L8): its balance minus everything that is
// already promised for the period. Income counts only once it is booked.
type SafeToSpend struct {
	AsOf      string // the app's date, YYYY-MM-DD
	PeriodEnd string // last day of the current payday period
	// DaysLeft counts the days from AsOf to PeriodEnd, both included.
	DaysLeft int
	// MainPotCents is the balance of the main pot on AsOf.
	MainPotCents int64
	// RecurringCents and RecurringCount are the open occurrences of monthly
	// payments of the main pot up to PeriodEnd, overdue ones included.
	RecurringCents int64
	RecurringCount int
	// ReserveRatesCents is what the reserves still need to receive this
	// period: the parts of their rates not saved yet.
	ReserveRatesCents int64
	// SavingsCents is what the savings goals still need to receive this
	// period: the parts of their rates not saved yet.
	SavingsCents int64
	// BudgetLeftCents is the rest of the budget of the planned expense
	// categories, without recurring bookings; overspent categories count 0.
	BudgetLeftCents int64
	// AmountCents is MainPotCents minus the four deductions; negative means
	// the period is over-planned.
	AmountCents int64
	// PerDayCents is AmountCents spread over DaysLeft, rounded down; 0 when
	// nothing is left.
	PerDayCents int64
}

// SafeToSpend works out what is safe to spend in a ledger today. today is the
// app's local date and decides the current payday period.
func (s *Store) SafeToSpend(ctx context.Context, ledgerID, today string) (SafeToSpend, error) {
	if !validDate(today) {
		return SafeToSpend{}, ErrInvalidDate
	}
	mainPot, err := resolvePot(ctx, s.db, ledgerID, "")
	if err != nil {
		return SafeToSpend{}, err
	}
	p, err := s.planPeriod(ctx, ledgerID, "", today)
	if err != nil {
		return SafeToSpend{}, err
	}
	out := SafeToSpend{AsOf: today, PeriodEnd: p.To}
	if out.DaysLeft, err = daysFromTo(today, p.To); err != nil {
		return SafeToSpend{}, err
	}

	balance, err := s.LedgerBalance(ctx, ledgerID, today, time.Time{})
	if err != nil {
		return SafeToSpend{}, err
	}
	out.MainPotCents = balance.AvailableCents
	if err := s.addOpenRecurring(ctx, ledgerID, mainPot, today, &out); err != nil {
		return SafeToSpend{}, err
	}
	if err := s.addOpenReserveRates(ctx, ledgerID, today, &out); err != nil {
		return SafeToSpend{}, err
	}
	if err := s.addOpenSavings(ctx, ledgerID, today, &out); err != nil {
		return SafeToSpend{}, err
	}
	if err := s.addBudgetLeft(ctx, ledgerID, mainPot, p, &out); err != nil {
		return SafeToSpend{}, err
	}

	out.AmountCents = out.MainPotCents - out.RecurringCents - out.ReserveRatesCents - out.SavingsCents - out.BudgetLeftCents
	if out.AmountCents > 0 {
		out.PerDayCents = out.AmountCents / int64(out.DaysLeft)
	}
	return out, nil
}

// addOpenRecurring counts the open occurrences of monthly payments from the
// main pot. Payments due every few months are paid from their reserve, and
// payments from other pots do not touch the main pot.
func (s *Store) addOpenRecurring(ctx context.Context, ledgerID, mainPot, today string, out *SafeToSpend) error {
	open, err := s.OpenOccurrences(ctx, ledgerID, today)
	if err != nil {
		return err
	}
	for _, o := range open {
		if o.Payment.IntervalMonths == 1 && o.Payment.PotID == mainPot {
			out.RecurringCount++
			out.RecurringCents += o.Payment.AmountCents
		}
	}
	return nil
}

func (s *Store) addOpenReserveRates(ctx context.Context, ledgerID, today string, out *SafeToSpend) error {
	rates, err := s.ReserveRates(ctx, ledgerID, today)
	if err != nil {
		return err
	}
	for _, r := range rates {
		out.ReserveRatesCents += r.OpenCents
	}
	return nil
}

func (s *Store) addOpenSavings(ctx context.Context, ledgerID, today string, out *SafeToSpend) error {
	goals, err := s.SavingsGoals(ctx, ledgerID, today)
	if err != nil {
		return err
	}
	for _, g := range goals {
		out.SavingsCents += g.OpenCents
	}
	return nil
}

// addBudgetLeft sums what the planned expense categories of the current month
// may still spend. Bookings of recurring payments are left out, as their
// occurrences are counted on their own.
func (s *Store) addBudgetLeft(ctx context.Context, ledgerID, mainPot string, p planPeriod, out *SafeToSpend) error {
	lines, err := categoryLines(ctx, s.db, lineQuery{
		LedgerID: ledgerID, PotID: mainPot, Month: p.Month, From: p.From, To: p.To,
		Source: p.Source, WithoutRecurring: true,
	})
	if err != nil {
		return err
	}
	for _, l := range lines {
		if !l.IsIncome && l.PlannedCents > 0 {
			out.BudgetLeftCents += max(0, l.PlannedCents-l.ActualCents)
		}
	}
	return nil
}

// daysFromTo counts the days from from to to, both included.
func daysFromTo(from, to string) (int, error) {
	a, err := time.Parse(dateLayout, from)
	if err != nil {
		return 0, ErrInvalidDate
	}
	b, err := time.Parse(dateLayout, to)
	if err != nil {
		return 0, ErrInvalidDate
	}
	return int(b.Sub(a).Hours()/24) + 1, nil
}
