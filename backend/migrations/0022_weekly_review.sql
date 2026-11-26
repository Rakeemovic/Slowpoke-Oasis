-- The weekly reflection (rules W1-W3).

-- ISO weekday: 1 is Monday, 7 is Sunday. It applies to all ledgers of a user.
ALTER TABLE user ADD COLUMN review_weekday INTEGER NOT NULL DEFAULT 1
    CHECK (review_weekday BETWEEN 1 AND 7);

-- One row per completed reflection; only the covered days are kept.
CREATE TABLE weekly_review (
    ledger_id      TEXT NOT NULL REFERENCES ledger(id) ON DELETE CASCADE,
    user_id        TEXT NOT NULL REFERENCES user(id) ON DELETE CASCADE,
    covered_from   TEXT NOT NULL,
    covered_until  TEXT NOT NULL,
    completed_at   TEXT NOT NULL,
    PRIMARY KEY (ledger_id, user_id, covered_until),
    CHECK (covered_from <= covered_until)
);
