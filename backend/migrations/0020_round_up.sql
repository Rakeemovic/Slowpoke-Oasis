-- Round-up (rules T10, X21): a pot can collect the spare change of expenses
-- from the main pot. round_up_multiplier (1, 2, 3 or 5) switches it on,
-- round_up_from is the first booking day that is rounded up. Only one pot per
-- ledger collects, and neither the main pot nor a reserve does.
ALTER TABLE pot ADD COLUMN round_up_multiplier INTEGER
    CHECK (round_up_multiplier IS NULL OR (round_up_multiplier IN (1, 2, 3, 5)
                                           AND is_default = 0 AND reserve_for IS NULL));
ALTER TABLE pot ADD COLUMN round_up_from TEXT
    CHECK ((round_up_from IS NULL) = (round_up_multiplier IS NULL));
CREATE UNIQUE INDEX pot_one_round_up_per_ledger ON pot (ledger_id) WHERE round_up_multiplier IS NOT NULL;

-- The transfer into the collecting pot points at the expense it rounds up.
-- It is removed together with the expense, and each expense has at most one.
ALTER TABLE transactions ADD COLUMN round_up_of TEXT
    REFERENCES transactions(id) ON DELETE CASCADE
    CHECK (round_up_of IS NULL OR type = 'transfer');
CREATE UNIQUE INDEX transactions_one_round_up ON transactions (round_up_of) WHERE round_up_of IS NOT NULL;
