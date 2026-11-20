-- Password resets by email are replaced by a recovery code (rules X15, A4).
-- The code is shown once at registration; only its Argon2id hash is stored.
-- Accounts created before this migration have none until an operator resets
-- their password on the server.
ALTER TABLE user ADD COLUMN recovery_hash TEXT;

-- Email addresses are no longer collected. Existing ones are removed, together
-- with the reset codes that were sent to them.
DROP TABLE password_reset;
DROP INDEX user_email_unique;
ALTER TABLE user DROP COLUMN email;
