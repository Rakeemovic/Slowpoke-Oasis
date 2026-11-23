-- Reserves (budget step 4, rules R10–R14).
--
-- A recurring payment due every 3, 6 or 12 months saves for its next due date
-- in a one-time pot of its own, its reserve. The payment books from it
-- (recurring_payment.pot_id points to it), and the app suggests a rate each
-- payday period.
--
-- pot.reserve_for links a reserve to its payment. The link stays when the
-- payment is archived or becomes monthly, so the pot can be used again when
-- the payment comes back; until then it behaves like any other pot (R10).

ALTER TABLE pot ADD COLUMN reserve_for TEXT REFERENCES recurring_payment(id)
    CHECK (reserve_for IS NULL OR kind = 'one_time');

-- R10: a payment has at most one active reserve.
CREATE UNIQUE INDEX one_active_reserve_per_payment
    ON pot (reserve_for) WHERE reserve_for IS NOT NULL AND archived = 0;

-- Active payments that already save over several months get their reserve
-- now. The name is the payment's name; when another active pot of the ledger
-- has it, or two such payments share it, the start of the payment ID tells
-- them apart. Names stay within the 50 characters of rule T1.
CREATE TEMP TABLE new_reserve AS
SELECT
    lower(hex(randomblob(16))) AS id,
    r.id                       AS payment_id,
    r.ledger_id                AS ledger_id,
    r.amount_cents             AS amount_cents,
    CASE
        WHEN NOT EXISTS (SELECT 1 FROM pot p
                         WHERE p.ledger_id = r.ledger_id AND p.archived = 0
                           AND lower(p.name) = lower(r.name))
         AND NOT EXISTS (SELECT 1 FROM recurring_payment o
                         WHERE o.ledger_id = r.ledger_id AND o.id <> r.id
                           AND o.archived = 0 AND o.interval_months > 1
                           AND lower(o.name) = lower(r.name))
        THEN r.name
        ELSE substr(r.name, 1, 31) || ' (reserve ' || substr(r.id, 1, 8) || ')'
    END                        AS name
FROM recurring_payment r
WHERE r.archived = 0 AND r.interval_months > 1;

INSERT INTO pot (id, ledger_id, name, kind, target_cents, reserve_for)
SELECT id, ledger_id, name, 'one_time', amount_cents, payment_id FROM new_reserve;

UPDATE recurring_payment
SET pot_id = (SELECT n.id FROM new_reserve n WHERE n.payment_id = recurring_payment.id)
WHERE id IN (SELECT payment_id FROM new_reserve);

DROP TABLE new_reserve;
