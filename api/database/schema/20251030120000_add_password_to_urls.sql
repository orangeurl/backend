-- +goose Up
-- Add password protection fields to urls table
ALTER TABLE urls ADD COLUMN password_hash TEXT;
ALTER TABLE urls ADD COLUMN is_locked BOOLEAN DEFAULT FALSE;
ALTER TABLE urls ADD COLUMN password_attempts INTEGER DEFAULT 0;
ALTER TABLE urls ADD COLUMN last_password_attempt TIMESTAMP;

-- Create index for faster lookups on locked URLs
CREATE INDEX idx_urls_is_locked ON urls(is_locked) WHERE is_locked = TRUE;

-- +goose Down
-- Remove password protection fields
DROP INDEX IF EXISTS idx_urls_is_locked;
ALTER TABLE urls DROP COLUMN IF EXISTS last_password_attempt;
ALTER TABLE urls DROP COLUMN IF EXISTS password_attempts;
ALTER TABLE urls DROP COLUMN IF EXISTS is_locked;
ALTER TABLE urls DROP COLUMN IF EXISTS password_hash;
