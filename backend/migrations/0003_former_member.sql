-- Placeholder for members who left a household or deleted their account (rule X12).
-- It has no username and no password hash, so nobody can log in as it.
INSERT INTO user (id, name, created_at)
VALUES ('former-member', 'Ehemaliges Mitglied', '2026-11-16T00:00:00Z');

-- Records which user deleted a transaction (rule X13).
-- Only allowed while the transaction is soft-deleted.
ALTER TABLE transactions ADD COLUMN deleted_by TEXT REFERENCES user(id)
    CHECK (deleted_by IS NULL OR deleted_at IS NOT NULL);
