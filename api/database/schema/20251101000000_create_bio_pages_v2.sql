-- +goose Up
-- Bio Pages Table with all features
CREATE TABLE bio_pages (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,

    -- Profile Info
    username VARCHAR(50) UNIQUE NOT NULL,  -- URL slug
    display_name VARCHAR(100) NOT NULL,
    bio TEXT,  -- Markdown supported
    avatar_url TEXT,
    favicon_url TEXT,

    -- Customization
    theme VARCHAR(20) DEFAULT 'light',  -- 'light', 'dark', 'custom'
    custom_colors JSONB DEFAULT '{
        "primary": "#FF6B35",
        "secondary": "#4ECDC4",
        "background": "#FFFFFF",
        "text": "#000000"
    }',
    background_type VARCHAR(20) DEFAULT 'gradient',  -- 'gradient', 'solid', 'image'
    background_value TEXT,  -- gradient name, color code, or image URL
    button_style VARCHAR(20) DEFAULT 'rounded',  -- 'rounded', 'outline', 'glow', 'square'
    font_family VARCHAR(50) DEFAULT 'Inter',  -- Google Fonts

    -- Social Links (predefined platforms)
    social_links JSONB DEFAULT '{}',
    -- Example: {"instagram": "username", "twitter": "handle", "github": "user"}

    -- Custom Links with full features
    links JSONB DEFAULT '[]',
    -- Example: [{
    --   "id": "uuid",
    --   "title": "My Website",
    --   "url": "https://...",
    --   "thumbnail": "https://...",
    --   "icon": "globe",
    --   "status": "active",  -- 'active' | 'hidden'
    --   "schedule_start": "2024-01-01T00:00:00Z",
    --   "schedule_end": "2024-12-31T23:59:59Z",
    --   "order": 0,
    --   "click_count": 0
    -- }]

    -- SEO & Meta
    seo_title VARCHAR(100),
    seo_description VARCHAR(200),
    og_image_url TEXT,

    -- Settings
    is_published BOOLEAN DEFAULT false,
    subdomain VARCHAR(50) UNIQUE,  -- For custom subdomain (premium)

    -- Analytics
    total_views INTEGER DEFAULT 0,
    total_clicks INTEGER DEFAULT 0,

    -- Timestamps
    created_at TIMESTAMP DEFAULT NOW(),
    updated_at TIMESTAMP DEFAULT NOW(),
    last_published_at TIMESTAMP,

    -- Constraints
    CONSTRAINT valid_username CHECK (username ~ '^[a-zA-Z0-9_-]{3,50}$'),
    CONSTRAINT valid_theme CHECK (theme IN ('light', 'dark', 'custom')),
    CONSTRAINT valid_background_type CHECK (background_type IN ('gradient', 'solid', 'image')),
    CONSTRAINT valid_button_style CHECK (button_style IN ('rounded', 'outline', 'glow', 'square'))
);

-- Bio Page Link Clicks (for per-link analytics)
CREATE TABLE bio_link_clicks (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    bio_page_id UUID NOT NULL REFERENCES bio_pages(id) ON DELETE CASCADE,
    link_id VARCHAR(100) NOT NULL,  -- Matches link.id in JSONB

    -- Analytics Data
    ip_address VARCHAR(45),
    user_agent TEXT,
    referer TEXT,
    country VARCHAR(2),
    city VARCHAR(100),
    device VARCHAR(50),
    browser VARCHAR(50),
    os VARCHAR(50),
    is_bot BOOLEAN DEFAULT false,

    clicked_at TIMESTAMP DEFAULT NOW()
);

-- Bio Page Views (for page analytics)
CREATE TABLE bio_page_views (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    bio_page_id UUID NOT NULL REFERENCES bio_pages(id) ON DELETE CASCADE,

    -- Analytics Data
    ip_address VARCHAR(45),
    user_agent TEXT,
    referer TEXT,
    country VARCHAR(2),
    city VARCHAR(100),
    device VARCHAR(50),
    browser VARCHAR(50),
    os VARCHAR(50),
    is_bot BOOLEAN DEFAULT false,

    viewed_at TIMESTAMP DEFAULT NOW()
);

-- Indexes for performance
CREATE INDEX idx_bio_pages_user_id ON bio_pages(user_id);
CREATE INDEX idx_bio_pages_username ON bio_pages(username);
CREATE INDEX idx_bio_pages_subdomain ON bio_pages(subdomain);
CREATE INDEX idx_bio_pages_published ON bio_pages(is_published);
CREATE INDEX idx_bio_link_clicks_page_id ON bio_link_clicks(bio_page_id);
CREATE INDEX idx_bio_link_clicks_link_id ON bio_link_clicks(link_id);
CREATE INDEX idx_bio_link_clicks_date ON bio_link_clicks(clicked_at);
CREATE INDEX idx_bio_page_views_page_id ON bio_page_views(bio_page_id);
CREATE INDEX idx_bio_page_views_date ON bio_page_views(viewed_at);

-- Function to update updated_at timestamp
-- +goose StatementBegin
CREATE OR REPLACE FUNCTION update_bio_page_updated_at()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;
-- +goose StatementEnd

-- +goose StatementBegin
CREATE TRIGGER bio_page_updated_at_trigger
BEFORE UPDATE ON bio_pages
FOR EACH ROW
EXECUTE FUNCTION update_bio_page_updated_at();
-- +goose StatementEnd

-- +goose Down
DROP TRIGGER IF EXISTS bio_page_updated_at_trigger ON bio_pages;
DROP FUNCTION IF EXISTS update_bio_page_updated_at;
DROP TABLE IF EXISTS bio_page_views;
DROP TABLE IF EXISTS bio_link_clicks;
DROP TABLE IF EXISTS bio_pages;
