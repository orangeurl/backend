-- +goose Up
CREATE TABLE monthly_usage(
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID REFERENCES users(id) NOT NULL,
    month VARCHAR(7) NOT NULL, -- Format: YYYY-MM
    url_count INT NOT NULL DEFAULT 0,
    custom_link_count INT NOT NULL DEFAULT 0,
    qr_code_count INT NOT NULL DEFAULT 0,
    created_at TIMESTAMP DEFAULT NOW(),
    updated_at TIMESTAMP DEFAULT NOW(),
    UNIQUE(user_id, month)
);

-- +goose Down
DROP TABLE monthly_usage;


