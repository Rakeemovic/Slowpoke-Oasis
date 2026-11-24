-- Savings goals of pots (rules T6, T7): besides a target amount with an
-- optional date, a pot can save a fixed rate per payday period. A fixed rate
-- and a target date exclude each other: with a date, the rate is calculated.
ALTER TABLE pot ADD COLUMN monthly_rate_cents INTEGER
    CHECK (monthly_rate_cents IS NULL OR (monthly_rate_cents > 0 AND target_date IS NULL));
