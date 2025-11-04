-- +goose Up
-- Increase webhook secret column size to accommodate generated secrets
-- Generated secrets are "whsec_" + 64 hex chars = 70 characters total
-- Setting to 128 to allow for future flexibility
ALTER TABLE webhooks ALTER COLUMN secret TYPE VARCHAR(128);

-- +goose Down
ALTER TABLE webhooks ALTER COLUMN secret TYPE VARCHAR(64);
