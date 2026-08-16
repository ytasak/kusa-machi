-- Profile pictures.
--
-- Only the "a photo exists, and when it changed" marker lives in the database.
-- The bytes are files under PHOTO_DIR, laid out one directory per game date so
-- the daily cleanup can drop yesterday's pictures by removing a directory.
ALTER TABLE personas ADD COLUMN photo_updated_at TIMESTAMPTZ NULL;
