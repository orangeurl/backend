-- Bio Pages CRUD Operations

-- name: CreateBioPage :one
INSERT INTO bio_pages (
    user_id,
    username,
    display_name,
    bio,
    avatar_url,
    favicon_url,
    theme,
    custom_colors,
    background_type,
    background_value,
    button_style,
    font_family,
    social_links,
    links,
    seo_title,
    seo_description,
    og_image_url,
    is_published
) VALUES (
    $1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18
) RETURNING *;

-- name: GetBioPageByUsername :one
SELECT * FROM bio_pages
WHERE username = $1 AND is_published = true;

-- name: GetBioPageByUsernameForEdit :one
SELECT * FROM bio_pages
WHERE username = $1 AND user_id = $2;

-- name: GetUserBioPages :many
SELECT * FROM bio_pages
WHERE user_id = $1
ORDER BY created_at DESC;

-- name: UpdateBioPage :one
UPDATE bio_pages
SET
    display_name = COALESCE($2, display_name),
    bio = COALESCE($3, bio),
    avatar_url = COALESCE($4, avatar_url),
    favicon_url = COALESCE($5, favicon_url),
    theme = COALESCE($6, theme),
    custom_colors = COALESCE($7, custom_colors),
    background_type = COALESCE($8, background_type),
    background_value = COALESCE($9, background_value),
    button_style = COALESCE($10, button_style),
    font_family = COALESCE($11, font_family),
    social_links = COALESCE($12, social_links),
    links = COALESCE($13, links),
    seo_title = COALESCE($14, seo_title),
    seo_description = COALESCE($15, seo_description),
    og_image_url = COALESCE($16, og_image_url),
    is_published = COALESCE($17, is_published),
    updated_at = NOW()
WHERE username = $1 AND user_id = $18
RETURNING *;

-- name: DeleteBioPage :exec
DELETE FROM bio_pages
WHERE username = $1 AND user_id = $2;

-- name: CheckUsernameAvailability :one
SELECT EXISTS(
    SELECT 1 FROM bio_pages WHERE username = $1
) AS exists;

-- name: PublishBioPage :one
UPDATE bio_pages
SET
    is_published = true,
    last_published_at = NOW()
WHERE username = $1 AND user_id = $2
RETURNING *;

-- name: UnpublishBioPage :one
UPDATE bio_pages
SET is_published = false
WHERE username = $1 AND user_id = $2
RETURNING *;

-- name: IncrementBioPageViews :exec
UPDATE bio_pages
SET total_views = total_views + 1
WHERE id = $1;

-- name: IncrementBioPageClicks :exec
UPDATE bio_pages
SET total_clicks = total_clicks + 1
WHERE id = $1;

-- Analytics Queries

-- name: TrackBioPageView :one
INSERT INTO bio_page_views (
    bio_page_id,
    ip_address,
    user_agent,
    referer,
    country,
    city,
    device,
    browser,
    os,
    is_bot
) VALUES (
    $1, $2, $3, $4, $5, $6, $7, $8, $9, $10
) RETURNING *;

-- name: TrackBioLinkClick :one
INSERT INTO bio_link_clicks (
    bio_page_id,
    link_id,
    ip_address,
    user_agent,
    referer,
    country,
    city,
    device,
    browser,
    os,
    is_bot
) VALUES (
    $1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11
) RETURNING *;

-- name: GetBioPageAnalytics :one
SELECT
    COUNT(DISTINCT v.id) as total_views,
    COUNT(DISTINCT c.id) as total_clicks,
    COUNT(DISTINCT v.country) as unique_countries,
    COUNT(DISTINCT v.device) as unique_devices
FROM bio_pages bp
LEFT JOIN bio_page_views v ON v.bio_page_id = bp.id
LEFT JOIN bio_link_clicks c ON c.bio_page_id = bp.id
WHERE bp.username = $1 AND bp.user_id = $2;

-- name: GetBioPageViewsByDate :many
SELECT
    DATE(viewed_at) as date,
    COUNT(*) as views
FROM bio_page_views
WHERE bio_page_id = $1
    AND viewed_at >= $2
    AND viewed_at <= $3
GROUP BY DATE(viewed_at)
ORDER BY date DESC;

-- name: GetBioLinkClicksByLink :many
SELECT
    link_id,
    COUNT(*) as clicks
FROM bio_link_clicks
WHERE bio_page_id = $1
    AND clicked_at >= $2
    AND clicked_at <= $3
GROUP BY link_id
ORDER BY clicks DESC;

-- name: GetBioPageDeviceStats :many
SELECT
    device,
    COUNT(*) as count
FROM bio_page_views
WHERE bio_page_id = $1
GROUP BY device
ORDER BY count DESC;

-- name: GetBioPageCountryStats :many
SELECT
    country,
    COUNT(*) as count
FROM bio_page_views
WHERE bio_page_id = $1
GROUP BY country
ORDER BY count DESC
LIMIT 10;

-- name: GetBioPageBrowserStats :many
SELECT
    browser,
    COUNT(*) as count
FROM bio_page_views
WHERE bio_page_id = $1
GROUP BY browser
ORDER BY count DESC;

-- name: GetBioPageRecentViews :many
SELECT *
FROM bio_page_views
WHERE bio_page_id = $1
ORDER BY viewed_at DESC
LIMIT $2;

-- name: GetBioPageRecentClicks :many
SELECT *
FROM bio_link_clicks
WHERE bio_page_id = $1
ORDER BY clicked_at DESC
LIMIT $2;
