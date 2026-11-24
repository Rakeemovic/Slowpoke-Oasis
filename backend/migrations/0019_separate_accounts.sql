-- A pot can be a real sub-account at the bank (rule T9). Its money is not on
-- the main account, so the estimated account balance leaves it out. The main
-- pot and the reserves of recurring payments are never sub-accounts.
ALTER TABLE pot ADD COLUMN is_separate_account INTEGER NOT NULL DEFAULT 0
    CHECK (is_separate_account IN (0, 1) AND (is_separate_account = 0 OR (is_default = 0 AND reserve_for IS NULL)));
