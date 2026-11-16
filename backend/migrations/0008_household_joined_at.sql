-- When a member joined a household. Owner succession picks the longest-standing
-- member by this column instead of the internal row order. Memberships that
-- existed before this migration get the epoch, which sorts them first.
ALTER TABLE household_member ADD COLUMN joined_at TEXT NOT NULL DEFAULT '1970-01-01T00:00:00Z';
