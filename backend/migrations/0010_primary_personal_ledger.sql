-- L6: every account gets one personal ledger at registration, its primary
-- ledger. Users may create more personal ledgers later; GET /me names the
-- primary one. created_at only has second precision, so the primary ledger
-- is marked explicitly instead of being "the oldest".

ALTER TABLE ledger ADD COLUMN is_primary INTEGER NOT NULL DEFAULT 0 CHECK (is_primary IN (0, 1));

-- At most one primary ledger per user. Household ledgers never set the flag.
CREATE UNIQUE INDEX ledger_one_primary_per_user ON ledger (user_id) WHERE is_primary = 1;

-- Existing accounts: the oldest personal ledger becomes the primary one.
UPDATE ledger SET is_primary = 1
WHERE id IN (
    SELECT (SELECT l2.id FROM ledger l2
            WHERE l2.user_id = l.user_id
            ORDER BY l2.created_at, l2.rowid LIMIT 1)
    FROM ledger l
    WHERE l.user_id IS NOT NULL
);
