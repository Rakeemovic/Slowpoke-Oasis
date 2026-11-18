-- Authentication: login names, password hashes and login sessions.
-- Session tokens are never stored; only their SHA-256 hash is kept.

ALTER TABLE user ADD COLUMN username TEXT;
ALTER TABLE user ADD COLUMN password_hash TEXT;

-- Login names are unique, ignoring case. Existing users without a login name are allowed.
CREATE UNIQUE INDEX user_username_unique ON user (lower(username)) WHERE username IS NOT NULL;

CREATE TABLE session (
    token_hash  TEXT PRIMARY KEY,
    user_id     TEXT NOT NULL REFERENCES user(id) ON DELETE CASCADE,
    created_at  TEXT NOT NULL,
    expires_at  TEXT NOT NULL
);

CREATE INDEX session_user ON session (user_id);
