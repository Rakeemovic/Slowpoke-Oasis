-- Members of a household are either owners or regular members.
-- Owners may add and remove other members.
ALTER TABLE household_member ADD COLUMN role TEXT NOT NULL DEFAULT 'member'
    CHECK (role IN ('owner', 'member'));
