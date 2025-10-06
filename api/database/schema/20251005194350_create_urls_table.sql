-- +goose Up
CREATE TABLE urls(
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID REFERENCES users(id) NOT NULL,
    short_id VARCHAR(255) NOT NULL,
    original_url VARCHAR(255) NOT NULL,
    expiry TIMESTAMP,
    is_active BOOLEAN DEFAULT TRUE,
    created_at TIMESTAMP DEFAULT NOW(),
    updated_at TIMESTAMP DEFAULT NOW()
);

-- +goose Down
DROP TABLE urls;
