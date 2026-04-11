-- +goose Up
-- Admin-managed blocked domains list
CREATE TABLE blocked_domains(
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    domain VARCHAR(255) NOT NULL UNIQUE,
    include_subdomains BOOLEAN NOT NULL DEFAULT TRUE,
    block_reason TEXT NOT NULL DEFAULT 'Blocked by admin',
    blocked_by UUID REFERENCES users(id),
    created_at TIMESTAMP DEFAULT NOW()
);

CREATE INDEX idx_blocked_domains_domain ON blocked_domains(domain);

-- +goose Down
DROP INDEX IF EXISTS idx_blocked_domains_domain;
DROP TABLE blocked_domains;
