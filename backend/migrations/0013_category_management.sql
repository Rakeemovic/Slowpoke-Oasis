-- Category management (rules K2, K5–K9) and spontaneous expenses (rule X20).

-- K5: an expense category may say what kind of spending it is. Income
-- categories have no purpose. Existing categories stay without one; only new
-- ledgers get the default purposes (see defaultCategories in setup.go).
ALTER TABLE category ADD COLUMN purpose TEXT
    CHECK (purpose IS NULL OR (purpose IN ('need', 'want', 'saving') AND is_income = 0));

-- K6: "Other" and "Other income" catch the transactions of deleted categories.
-- They are recognised by this flag, not by their name, so users may rename
-- them. A fallback category is never archived.
ALTER TABLE category ADD COLUMN is_fallback INTEGER NOT NULL DEFAULT 0
    CHECK (is_fallback IN (0, 1) AND (is_fallback = 0 OR archived = 0));

-- At most one fallback category per ledger and kind (expense, income).
CREATE UNIQUE INDEX category_one_fallback_per_kind ON category (ledger_id, is_income) WHERE is_fallback = 1;

-- Existing ledgers: the active categories named like the fallback categories
-- become them. Ledgers without one get it when they first delete a category.
UPDATE category SET is_fallback = 1
WHERE archived = 0
  AND ((is_income = 0 AND lower(name) = 'other')
    OR (is_income = 1 AND lower(name) = 'other income'));

-- X20: the author may mark an expense as spontaneous (an unplanned purchase).
ALTER TABLE transactions ADD COLUMN is_spontaneous INTEGER NOT NULL DEFAULT 0
    CHECK (is_spontaneous IN (0, 1) AND (is_spontaneous = 0 OR type = 'expense'));
