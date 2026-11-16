-- A transfer between two ledgers has one leg in each ledger (rule X14). When
-- one ledger is deleted (account deletion, or the last member leaving a
-- household), the leg in the other ledger must stay, because the money did
-- arrive there or leave from there (rule X16).
--
-- Until now the surviving leg kept pointing at the deleted one, so the foreign
-- key made the deletion fail. Now the link is cleared instead: ON DELETE SET
-- NULL, and a transfer_out or transfer_in leg may exist without a link.
--
-- SQLite cannot change a constraint in place, so the table is rebuilt. The new
-- table references itself under its temporary name; the rename at the end
-- updates that reference. Referencing "transactions" directly would clear
-- every existing link when the old table is dropped, because dropping a table
-- deletes its rows and so triggers ON DELETE SET NULL.

CREATE TABLE transactions_new (
    id                    TEXT PRIMARY KEY,
    ledger_id             TEXT NOT NULL REFERENCES ledger(id) ON DELETE CASCADE,
    user_id               TEXT NOT NULL REFERENCES user(id),                     -- X7 is checked by the app
    type                  TEXT NOT NULL CHECK (type IN ('expense', 'income', 'transfer',
                                                        'transfer_out', 'transfer_in')),
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
    -- X2, X3, X4, X14: the type determines the sign and the required fields
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
    ),
    CHECK (deleted_by IS NULL OR deleted_at IS NOT NULL),                        -- X13
    FOREIGN KEY (ledger_id, pot_id) REFERENCES pot (ledger_id, id),              -- X4: same ledger
    FOREIGN KEY (ledger_id, counterpart_pot_id) REFERENCES pot (ledger_id, id),  -- X4: same ledger
    FOREIGN KEY (ledger_id, category_id) REFERENCES category (ledger_id, id)      -- X6: same ledger
);

INSERT INTO transactions_new (id, ledger_id, user_id, type, category_id, pot_id,
                              counterpart_pot_id, amount_cents, booked_on, note,
                              created_at, updated_at, deleted_at, deleted_by,
                              linked_transaction_id)
SELECT id, ledger_id, user_id, type, category_id, pot_id,
       counterpart_pot_id, amount_cents, booked_on, note,
       created_at, updated_at, deleted_at, deleted_by,
       linked_transaction_id
FROM transactions;

DROP TABLE transactions;
ALTER TABLE transactions_new RENAME TO transactions;

-- Common queries: ledger list by date and pot totals, ignoring soft-deleted rows
CREATE INDEX transactions_ledger_date ON transactions (ledger_id, booked_on) WHERE deleted_at IS NULL;
CREATE INDEX transactions_pot ON transactions (pot_id) WHERE deleted_at IS NULL;
CREATE INDEX transactions_linked ON transactions (linked_transaction_id) WHERE linked_transaction_id IS NOT NULL;
