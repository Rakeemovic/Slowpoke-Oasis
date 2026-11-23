-- The default pot is called "Main" from now on (it holds the money of the
-- account, see docs/data-model.md). Only default pots that still carry the
-- old standard name are renamed; a name the user chose stays. A ledger that
-- already has an active pot called "Main" keeps "Monthly" (rule T1).
UPDATE pot SET name = 'Main'
WHERE is_default = 1
  AND name = 'Monthly'
  AND NOT EXISTS (
    SELECT 1 FROM pot other
    WHERE other.ledger_id = pot.ledger_id
      AND other.id <> pot.id
      AND other.archived = 0
      AND lower(other.name) = 'main'
  );
