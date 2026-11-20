-- L5: a household has exactly one ledger. GET /households returns it as
-- ledger_id, so a second ledger would make that answer ambiguous.
-- Personal ledgers have household_id NULL; SQLite treats NULLs as distinct,
-- so they are not affected by this index.
CREATE UNIQUE INDEX ledger_one_per_household ON ledger (household_id);
