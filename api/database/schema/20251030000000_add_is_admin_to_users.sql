-- +goose Up
ALTER TABLE users
  ADD COLUMN IF NOT EXISTS is_admin BOOLEAN DEFAULT FALSE;

-- Create index for faster admin lookups
CREATE INDEX IF NOT EXISTS idx_users_is_admin ON users(is_admin) WHERE is_admin = TRUE;

-- +goose Down
DROP INDEX IF EXISTS idx_users_is_admin;
ALTER TABLE users
  DROP COLUMN IF EXISTS is_admin;
