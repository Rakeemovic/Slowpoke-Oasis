-- Recurring payments (budget step 3, rules R1–R8).
--
-- A recurring payment is a reminder, never booked automatically: the app
-- lists its open occurrences, and a tap books one as a normal expense that
-- remembers which occurrence it paid (recurring_payment_id + recurring_due_on).

CREATE TABLE recurring_payment (
    id               TEXT PRIMARY KEY,
    ledger_id        TEXT NOT NULL REFERENCES ledger(id) ON DELETE CASCADE,
    name             TEXT NOT NULL CHECK (length(trim(name)) > 0),
    amount_cents     INTEGER NOT NULL CHECK (amount_cents > 0 AND amount_cents <= 10000000000),  -- R1
    category_id      TEXT NOT NULL,                                                  -- R2
    pot_id           TEXT NOT NULL,
    interval_months  INTEGER NOT NULL CHECK (interval_months IN (1, 3, 6, 12)),      -- R3
    due_day          INTEGER NOT NULL CHECK (due_day BETWEEN 1 AND 31),              -- R3
    first_month      TEXT NOT NULL CHECK (first_month GLOB '[0-9][0-9][0-9][0-9]-[0-1][0-9]'),
    last_month       TEXT CHECK (last_month IS NULL
                                 OR (last_month GLOB '[0-9][0-9][0-9][0-9]-[0-1][0-9]'
                                     AND last_month >= first_month)),             -- R4
    -- R5: occurrences due before this date are never open. It is the day the
    -- payment was created, and moves to the day its schedule last changed.
    tracked_from     TEXT NOT NULL CHECK (tracked_from GLOB '[0-9][0-9][0-9][0-9]-[0-9][0-9]-[0-9][0-9]'),
    archived         INTEGER NOT NULL DEFAULT 0 CHECK (archived IN (0, 1)),
    created_at       TEXT NOT NULL,
    UNIQUE (ledger_id, id),
    FOREIGN KEY (ledger_id, category_id) REFERENCES category (ledger_id, id),
    FOREIGN KEY (ledger_id, pot_id) REFERENCES pot (ledger_id, id)
);

CREATE INDEX recurring_payment_ledger ON recurring_payment (ledger_id);

-- R7: occurrences a member chose not to book (e.g. paid in another way).
CREATE TABLE recurring_skip (
    recurring_payment_id  TEXT NOT NULL REFERENCES recurring_payment(id) ON DELETE CASCADE,
    due_on                TEXT NOT NULL CHECK (due_on GLOB '[0-9][0-9][0-9][0-9]-[0-9][0-9]-[0-9][0-9]'),
    PRIMARY KEY (recurring_payment_id, due_on)
);

-- R6: an expense may pay one occurrence of a recurring payment. Both columns
-- are set together, and only on expenses.
ALTER TABLE transactions ADD COLUMN recurring_payment_id TEXT REFERENCES recurring_payment(id)
    CHECK (recurring_payment_id IS NULL OR type = 'expense');
ALTER TABLE transactions ADD COLUMN recurring_due_on TEXT
    CHECK ((recurring_due_on IS NULL) = (recurring_payment_id IS NULL)
           AND (recurring_due_on IS NULL
                OR recurring_due_on GLOB '[0-9][0-9][0-9][0-9]-[0-9][0-9]-[0-9][0-9]'));

-- R6: at most one booking per occurrence. Deleted bookings do not count, so a
-- deleted booking makes its occurrence open again.
CREATE UNIQUE INDEX transactions_one_booking_per_occurrence
    ON transactions (recurring_payment_id, recurring_due_on)
    WHERE recurring_payment_id IS NOT NULL AND deleted_at IS NULL;
