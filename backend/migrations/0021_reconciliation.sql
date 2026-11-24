-- Reconciliation (rules X22, L9): the user enters the real balance of the
-- bank account (or of a sub-account, rule T9); a difference can be booked as
-- a correction.
--
-- 1. A new transaction type 'correction': it changes the balance of one pot,
--    has no category and either sign, and so does not count in the budget or
--    the category reports.
--
-- SQLite cannot change a CHECK constraint in place, so the table is rebuilt
-- like in 0007. The new table references itself under its temporary name;
-- the rename at the end updates those references. Referencing "transactions"
-- directly would clear every link (and cascade-delete every round-up) when
-- the old table is dropped, because dropping a table deletes its rows.

CREATE TABLE transactions_new (
    id                    TEXT PRIMARY KEY,
    ledger_id             TEXT NOT NULL REFERENCES ledger(id) ON DELETE CASCADE,
    user_id               TEXT NOT NULL REFERENCES user(id),                     -- X7 is checked by the app
    type                  TEXT NOT NULL CHECK (type IN ('expense', 'income', 'transfer',
                                                        'transfer_out', 'transfer_in',
                                                        'correction')),
    category_id           TEXT,
    pot_id                TEXT NOT NULL,                                         -- X5: the app sets the default pot
    counterpart_pot_id    TEXT,
    amount_cents          INTEGER NOT NULL
                          CHECK (amount_cents <> 0                               -- X1
                                 AND abs(amount_cents) <= 10000000000),          -- X11
    booked_on             TEXT NOT NULL
                          CHECK (booked_on GLOB '[0-9][0-9][0-9][0-9]-[0-9][0-9]-[0-9][0-9]'),  -- X8
    note                  TEXT CHECK (note IS NULL OR length(note) <= 500),      -- X9
    created_at            TEXT NOT NULL,
    updated_at            TEXT NOT NULL,
    deleted_at            TEXT,                                                  -- X10: soft delete
    deleted_by            TEXT REFERENCES user(id),                              -- X13
    linked_transaction_id TEXT REFERENCES transactions_new(id) ON DELETE SET NULL
                          DEFERRABLE INITIALLY DEFERRED,                         -- X14, X16
    is_spontaneous        INTEGER NOT NULL DEFAULT 0
                          CHECK (is_spontaneous IN (0, 1)
                                 AND (is_spontaneous = 0 OR type = 'expense')),  -- X20
    recurring_payment_id  TEXT REFERENCES recurring_payment(id)
                          CHECK (recurring_payment_id IS NULL OR type = 'expense'),  -- R6
    recurring_due_on      TEXT
                          CHECK ((recurring_due_on IS NULL) = (recurring_payment_id IS NULL)
                                 AND (recurring_due_on IS NULL
                                      OR recurring_due_on GLOB '[0-9][0-9][0-9][0-9]-[0-9][0-9]-[0-9][0-9]')),
    round_up_of           TEXT REFERENCES transactions_new(id) ON DELETE CASCADE
                          CHECK (round_up_of IS NULL OR type = 'transfer'),      -- X21
    -- X2, X3, X4, X14, X22: the type determines the sign and the required fields
    CHECK (
        (type = 'expense'      AND amount_cents < 0 AND category_id IS NOT NULL
                               AND counterpart_pot_id IS NULL AND linked_transaction_id IS NULL)
     OR (type = 'income'       AND amount_cents > 0 AND category_id IS NOT NULL
                               AND counterpart_pot_id IS NULL AND linked_transaction_id IS NULL)
     OR (type = 'transfer'     AND amount_cents > 0 AND category_id IS NULL
                               AND counterpart_pot_id IS NOT NULL AND counterpart_pot_id <> pot_id
                               AND linked_transaction_id IS NULL)
     OR (type = 'transfer_out' AND amount_cents < 0 AND category_id IS NULL
                               AND counterpart_pot_id IS NULL)
     OR (type = 'transfer_in'  AND amount_cents > 0 AND category_id IS NULL
                               AND counterpart_pot_id IS NULL)
     OR (type = 'correction'   AND category_id IS NULL
                               AND counterpart_pot_id IS NULL AND linked_transaction_id IS NULL)
    ),
    CHECK (deleted_by IS NULL OR deleted_at IS NOT NULL),                        -- X13
    FOREIGN KEY (ledger_id, pot_id) REFERENCES pot (ledger_id, id),              -- X4: same ledger
    FOREIGN KEY (ledger_id, counterpart_pot_id) REFERENCES pot (ledger_id, id),  -- X4: same ledger
    FOREIGN KEY (ledger_id, category_id) REFERENCES category (ledger_id, id)      -- X6: same ledger
);

INSERT INTO transactions_new (id, ledger_id, user_id, type, category_id, pot_id,
                              counterpart_pot_id, amount_cents, booked_on, note,
                              created_at, updated_at, deleted_at, deleted_by,
                              linked_transaction_id, is_spontaneous,
                              recurring_payment_id, recurring_due_on, round_up_of)
SELECT id, ledger_id, user_id, type, category_id, pot_id,
       counterpart_pot_id, amount_cents, booked_on, note,
       created_at, updated_at, deleted_at, deleted_by,
       linked_transaction_id, is_spontaneous,
       recurring_payment_id, recurring_due_on, round_up_of
FROM transactions;

DROP TABLE transactions;
ALTER TABLE transactions_new RENAME TO transactions;

CREATE INDEX transactions_ledger_date ON transactions (ledger_id, booked_on) WHERE deleted_at IS NULL;
CREATE INDEX transactions_pot ON transactions (pot_id) WHERE deleted_at IS NULL;
CREATE INDEX transactions_linked ON transactions (linked_transaction_id) WHERE linked_transaction_id IS NOT NULL;
CREATE UNIQUE INDEX transactions_one_booking_per_occurrence
    ON transactions (recurring_payment_id, recurring_due_on)
    WHERE recurring_payment_id IS NOT NULL AND deleted_at IS NULL;
CREATE UNIQUE INDEX transactions_one_round_up ON transactions (round_up_of) WHERE round_up_of IS NOT NULL;

-- 2. The last reconciliation of an account (rule L9). pot_id is the default
--    pot for the bank account of the ledger, or a pot that is a separate
--    account at the bank. Only the last one is kept, so the pot is the key.
CREATE TABLE reconciliation (
    pot_id        TEXT PRIMARY KEY,
    ledger_id     TEXT NOT NULL,
    reconciled_on TEXT NOT NULL
                  CHECK (reconciled_on GLOB '[0-9][0-9][0-9][0-9]-[0-9][0-9]-[0-9][0-9]'),
    actual_cents  INTEGER NOT NULL CHECK (abs(actual_cents) <= 10000000000),
    FOREIGN KEY (ledger_id, pot_id) REFERENCES pot (ledger_id, id) ON DELETE CASCADE
);
