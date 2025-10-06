-- +goose Up
CREATE TABLE url_clicks(
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    url_id UUID REFERENCES urls(id) NOT NULL,
    ip_address INET,
    user_agent TEXT,
    referer TEXT,
    country VARCHAR(255),
    city VARCHAR(255),
    device_type VARCHAR(255),
    browser VARCHAR(255),
    os VARCHAR(255),
    is_bot BOOLEAN DEFAULT FALSE,
    clicked_at TIMESTAMP DEFAULT NOW()
);

-- +goose Down
DROP TABLE url_clicks;
