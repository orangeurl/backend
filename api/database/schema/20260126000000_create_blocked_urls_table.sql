-- +goose Up
-- Migration for tracking blocked/abusive URLs
CREATE TABLE blocked_urls(
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    short_id VARCHAR(255) NOT NULL UNIQUE,
    original_url TEXT,
    block_reason TEXT NOT NULL,
    blocked_by UUID REFERENCES users(id),
    abuse_report_ref TEXT,
    created_at TIMESTAMP DEFAULT NOW()
);

-- Index for fast lookups when resolving URLs
CREATE INDEX idx_blocked_urls_short_id ON blocked_urls(short_id);

-- +goose Down
DROP INDEX IF EXISTS idx_blocked_urls_short_id;
DROP TABLE blocked_urls;
