-- Email address for password resets (rule X15). New accounts must give one, which
-- the application enforces. The column stays nullable because the placeholder
-- user "former-member" has no address.
ALTER TABLE user ADD COLUMN email TEXT;

-- Email addresses are unique, ignoring case.
CREATE UNIQUE INDEX user_email_unique ON user (lower(email)) WHERE email IS NOT NULL;

-- Password reset codes. Like session tokens, only their SHA-256 hash is stored.
-- A code is used up by deleting it, so no "used" flag is needed.
CREATE TABLE password_reset (
    token_hash  TEXT PRIMARY KEY,
    user_id     TEXT NOT NULL REFERENCES user(id) ON DELETE CASCADE,
    created_at  TEXT NOT NULL,
    expires_at  TEXT NOT NULL
);

CREATE INDEX password_reset_user ON password_reset (user_id);
