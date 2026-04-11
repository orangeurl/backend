-- +goose Up
ALTER TABLE users ADD COLUMN is_banned BOOLEAN DEFAULT FALSE;

-- +goose Down
ALTER TABLE users DROP COLUMN IF EXISTS is_banned;
