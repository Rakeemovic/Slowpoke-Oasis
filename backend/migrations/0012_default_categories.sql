-- K4: new ledgers start with default categories (see defaultCategories in
-- internal/ledger/setup.go). Existing ledgers without any category get them
-- too, so their owners can book right away. Ledgers that already have
-- categories, even only archived ones, stay as they are: their users chose
-- their own list.
--
-- This list is a snapshot; later changes to defaultCategories only affect
-- ledgers created afterwards.

CREATE TEMP TABLE default_category (name TEXT NOT NULL, is_income INTEGER NOT NULL);
INSERT INTO default_category (name, is_income) VALUES
    ('Salary', 1),
    ('Other income', 1),
    ('Groceries', 0),
    ('Rent', 0),
    ('Utilities', 0),
    ('Transport', 0),
    ('Insurance', 0),
    ('Health', 0),
    ('Eating out', 0),
    ('Leisure', 0),
    ('Clothing', 0),
    ('Subscriptions', 0),
    ('Household', 0),
    ('Other', 0);

-- IDs use the same format as newID in Go: 32 lower-case hex characters.
INSERT INTO category (id, ledger_id, name, is_income)
SELECT lower(hex(randomblob(16))), l.id, d.name, d.is_income
FROM ledger l CROSS JOIN default_category d
WHERE NOT EXISTS (SELECT 1 FROM category c WHERE c.ledger_id = l.id);

DROP TABLE default_category;
