-- Add estimated_refresh_interval column to Feed table.

SET DATABASE TO Goliath;

ALTER TABLE Feed ADD COLUMN IF NOT EXISTS estimated_refresh_interval INT DEFAULT 600;
