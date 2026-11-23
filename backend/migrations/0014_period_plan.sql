-- Monthly plan (budget step 1).
--
-- pot_line stays the plan of one-time pots and becomes the template of
-- monthly pots. Each month of a monthly pot gets its own copy of the template
-- in period_plan, so that changing the plan of one month leaves the others
-- alone. "month" is the month in which the payday period starts (YYYY-MM).

CREATE TABLE period_plan (
    pot_id        TEXT NOT NULL,
    ledger_id     TEXT NOT NULL,
    category_id   TEXT NOT NULL,
    month         TEXT NOT NULL CHECK (month GLOB '[0-9][0-9][0-9][0-9]-[0-1][0-9]'),
    amount_cents  INTEGER NOT NULL CHECK (amount_cents >= 0),                    -- P1
    PRIMARY KEY (pot_id, category_id, month),
    FOREIGN KEY (ledger_id, pot_id) REFERENCES pot (ledger_id, id) ON DELETE CASCADE,
    FOREIGN KEY (ledger_id, category_id) REFERENCES category (ledger_id, id)     -- P2
);

-- P3: a category is planned in at most one monthly pot of a ledger per month.
-- Only monthly pots have rows here, so the database can enforce the rule.
CREATE UNIQUE INDEX period_plan_one_pot_per_category
    ON period_plan (ledger_id, category_id, month);

-- Remembers the months whose plans were created from the templates (P4).
-- Without it, a month whose lines were all removed would be filled again
-- from the template on the next access.
CREATE TABLE period_plan_month (
    ledger_id  TEXT NOT NULL REFERENCES ledger(id) ON DELETE CASCADE,
    month      TEXT NOT NULL CHECK (month GLOB '[0-9][0-9][0-9][0-9]-[0-1][0-9]'),
    PRIMARY KEY (ledger_id, month)
);
