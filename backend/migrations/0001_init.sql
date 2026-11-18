-- Slowpoke-Oasis: initial schema (SQLite)
-- Rules are documented in docs/data-model.md and docs/validation-rules.md.
-- Foreign keys must be enabled per connection: PRAGMA foreign_keys = ON;

PRAGMA foreign_keys = ON;

-- ---------------------------------------------------------------
-- Users and households
-- ---------------------------------------------------------------

CREATE TABLE user (
    id          TEXT PRIMARY KEY,
    name        TEXT NOT NULL CHECK (length(trim(name)) > 0),
    created_at  TEXT NOT NULL
);

CREATE TABLE household (
    id          TEXT PRIMARY KEY,
    name        TEXT NOT NULL CHECK (length(trim(name)) > 0),
    created_at  TEXT NOT NULL
);

CREATE TABLE household_member (
    household_id  TEXT NOT NULL REFERENCES household(id) ON DELETE CASCADE,
    user_id       TEXT NOT NULL REFERENCES user(id) ON DELETE CASCADE,
    PRIMARY KEY (household_id, user_id)
);

-- ---------------------------------------------------------------
-- Ledger (personal or household)
-- ---------------------------------------------------------------

CREATE TABLE ledger (
    id            TEXT PRIMARY KEY,
    name          TEXT NOT NULL CHECK (length(trim(name)) > 0),
    user_id       TEXT REFERENCES user(id) ON DELETE CASCADE,
    household_id  TEXT REFERENCES household(id) ON DELETE CASCADE,
    payday_day    INTEGER NOT NULL DEFAULT 1 CHECK (payday_day BETWEEN 1 AND 31),  -- L2
    currency      TEXT NOT NULL DEFAULT 'EUR' CHECK (length(currency) = 3),        -- L3 (allowed list is enforced by the app)
    created_at    TEXT NOT NULL,
    -- L1: exactly one of user_id and household_id is set
    CHECK ((user_id IS NULL) <> (household_id IS NULL)),
    UNIQUE (id)
);

-- ---------------------------------------------------------------
-- Categories
-- ---------------------------------------------------------------

CREATE TABLE category (
    id          TEXT PRIMARY KEY,
    ledger_id   TEXT NOT NULL REFERENCES ledger(id) ON DELETE CASCADE,
    name        TEXT NOT NULL CHECK (length(trim(name)) BETWEEN 1 AND 50),       -- K1
    is_income   INTEGER NOT NULL DEFAULT 0 CHECK (is_income IN (0, 1)),         -- K3
    archived    INTEGER NOT NULL DEFAULT 0 CHECK (archived IN (0, 1)),          -- K2
    UNIQUE (ledger_id, id)                                                      -- target for composite foreign keys
);

-- K1: names are unique per ledger, case-insensitive
CREATE UNIQUE INDEX category_name_per_ledger ON category (ledger_id, lower(name));

-- ---------------------------------------------------------------
-- Pots (budgets / savings goals)
-- ---------------------------------------------------------------

CREATE TABLE pot (
    id            TEXT PRIMARY KEY,
    ledger_id     TEXT NOT NULL REFERENCES ledger(id) ON DELETE CASCADE,
    name          TEXT NOT NULL CHECK (length(trim(name)) > 0),
    kind          TEXT NOT NULL CHECK (kind IN ('monthly', 'one_time')),
    is_default    INTEGER NOT NULL DEFAULT 0 CHECK (is_default IN (0, 1)),
    target_cents  INTEGER CHECK (target_cents IS NULL OR target_cents > 0),      -- T3
    target_date   TEXT,                                                          -- ISO date, see check below
    archived      INTEGER NOT NULL DEFAULT 0 CHECK (archived IN (0, 1)),
    -- T4: a target date requires a target amount
    CHECK (target_date IS NULL OR target_cents IS NOT NULL),
    -- T2: the default pot is a monthly pot and is never archived
    CHECK (is_default = 0 OR (kind = 'monthly' AND archived = 0)),
    UNIQUE (ledger_id, id)                                                       -- target for composite foreign keys
);

-- L4: exactly one default pot per ledger
CREATE UNIQUE INDEX one_default_pot_per_ledger ON pot (ledger_id) WHERE is_default = 1;

-- T1: names are unique among active pots of a ledger
CREATE UNIQUE INDEX pot_name_active_per_ledger ON pot (ledger_id, lower(name)) WHERE archived = 0;

-- T2: the default pot cannot be deleted
CREATE TRIGGER pot_default_no_delete
BEFORE DELETE ON pot
WHEN OLD.is_default = 1
BEGIN
    SELECT RAISE(ABORT, 'The default pot cannot be deleted');
END;

-- Planned amount per category within a pot
CREATE TABLE pot_line (
    pot_id        TEXT NOT NULL,
    ledger_id     TEXT NOT NULL,   -- added so P2 can be enforced by the database
    category_id   TEXT NOT NULL,
    amount_cents  INTEGER NOT NULL CHECK (amount_cents >= 0),                    -- P1
    PRIMARY KEY (pot_id, category_id),
    FOREIGN KEY (ledger_id, pot_id) REFERENCES pot (ledger_id, id) ON DELETE CASCADE,
    FOREIGN KEY (ledger_id, category_id) REFERENCES category (ledger_id, id)     -- P2
);

-- ---------------------------------------------------------------
-- Transactions
-- Table is named "transactions" because TRANSACTION is a reserved word in SQLite.
-- ---------------------------------------------------------------

CREATE TABLE transactions (
    id                  TEXT PRIMARY KEY,
    ledger_id           TEXT NOT NULL REFERENCES ledger(id) ON DELETE CASCADE,
    user_id             TEXT NOT NULL REFERENCES user(id),                       -- X7 is checked by the app
    type                TEXT NOT NULL CHECK (type IN ('expense', 'income', 'transfer')),
    category_id         TEXT,
    pot_id              TEXT NOT NULL,                                           -- X5: the app sets the default pot
    counterpart_pot_id  TEXT,
    amount_cents        INTEGER NOT NULL
                        CHECK (amount_cents <> 0                                 -- X1
                               AND abs(amount_cents) <= 10000000000),            -- X11
    booked_on           TEXT NOT NULL
                        CHECK (booked_on GLOB '[0-9][0-9][0-9][0-9]-[0-9][0-9]-[0-9][0-9]'),  -- X8
    note                TEXT CHECK (note IS NULL OR length(note) <= 500),        -- X9
    created_at          TEXT NOT NULL,
    updated_at          TEXT NOT NULL,
    deleted_at          TEXT,                                                    -- X10: soft delete
    -- X2, X3, X4: the type determines the sign and the required fields
    CHECK (
        (type = 'expense'  AND amount_cents < 0 AND category_id IS NOT NULL AND counterpart_pot_id IS NULL)
     OR (type = 'income'   AND amount_cents > 0 AND category_id IS NOT NULL AND counterpart_pot_id IS NULL)
     OR (type = 'transfer' AND amount_cents > 0 AND category_id IS NULL
                           AND counterpart_pot_id IS NOT NULL AND counterpart_pot_id <> pot_id)
    ),
    FOREIGN KEY (ledger_id, pot_id) REFERENCES pot (ledger_id, id),              -- X4: same ledger
    FOREIGN KEY (ledger_id, counterpart_pot_id) REFERENCES pot (ledger_id, id),  -- X4: same ledger
    FOREIGN KEY (ledger_id, category_id) REFERENCES category (ledger_id, id)      -- X6: same ledger
);

-- Common queries: ledger list by date and pot totals, ignoring soft-deleted rows
CREATE INDEX transactions_ledger_date ON transactions (ledger_id, booked_on) WHERE deleted_at IS NULL;
CREATE INDEX transactions_pot ON transactions (pot_id) WHERE deleted_at IS NULL;

-- Background job (X10): permanently remove rows soft-deleted more than 30 days ago.
-- Example: DELETE FROM transactions WHERE deleted_at IS NOT NULL AND deleted_at < datetime('now', '-30 days');
