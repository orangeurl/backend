-- +goose Up
CREATE TABLE api_keys (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID REFERENCES users(id) ON DELETE CASCADE NOT NULL,
    key_name VARCHAR(255) NOT NULL,
    api_key_hash VARCHAR(255) UNIQUE NOT NULL,
    api_key_prefix VARCHAR(20) NOT NULL,
    permissions JSONB DEFAULT '["urls:read", "urls:write", "analytics:read"]'::jsonb,
    rate_limit_tier VARCHAR(50) NOT NULL DEFAULT 'free',
    last_used_at TIMESTAMP,
    is_active BOOLEAN DEFAULT TRUE,
    expires_at TIMESTAMP,
    created_at TIMESTAMP DEFAULT NOW(),
    updated_at TIMESTAMP DEFAULT NOW()
);

-- Create index for fast prefix lookup
CREATE INDEX idx_api_keys_prefix ON api_keys(api_key_prefix);
CREATE INDEX idx_api_keys_user_id ON api_keys(user_id);
CREATE INDEX idx_api_keys_active ON api_keys(is_active) WHERE is_active = TRUE;

-- +goose Down
DROP INDEX IF EXISTS idx_api_keys_active;
DROP INDEX IF EXISTS idx_api_keys_user_id;
DROP INDEX IF EXISTS idx_api_keys_prefix;
DROP TABLE api_keys;
