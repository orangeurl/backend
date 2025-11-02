-- name: CreateClick :one
INSERT INTO url_clicks (url_id, ip_address, user_agent, referer, country, city, device_type, browser, os, is_bot)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
RETURNING *;

-- name: GetURLAnalytics :many
SELECT * FROM url_clicks WHERE url_id = $1
ORDER BY clicked_at DESC;

-- name: GetUserAnalytics :many
SELECT uc.* FROM url_clicks uc
JOIN urls u ON uc.url_id = u.id
WHERE u.user_id = $1
ORDER BY uc.clicked_at DESC;

-- name: GetURLClicksByBrowser :many
SELECT 
    browser,
    COUNT(*) as clicks
FROM url_clicks
WHERE url_id = $1 AND browser IS NOT NULL
GROUP BY browser
ORDER BY clicks DESC;

-- name: GetURLClicksByDate :many
SELECT 
    DATE(clicked_at) as date,
    COUNT(*) as clicks
FROM url_clicks
WHERE url_id = $1
GROUP BY DATE(clicked_at)
ORDER BY date DESC;

-- name: GetURLClicksByDevice :many
SELECT 
    device_type,
    COUNT(*) as clicks
FROM url_clicks
WHERE url_id = $1 AND device_type IS NOT NULL
GROUP BY device_type
ORDER BY clicks DESC;

-- name: GetUserClickStats :one
SELECT 
    COUNT(*) as total_clicks,
    COUNT(DISTINCT uc.url_id) as unique_urls_clicked,
    COUNT(DISTINCT uc.country) as unique_countries,
    COUNT(DISTINCT uc.city) as unique_cities
FROM url_clicks uc
JOIN urls u ON uc.url_id = u.id
WHERE u.user_id = $1;

-- name: GetUserClicksByBrowser :many
SELECT 
    browser,
    COUNT(*) as clicks
FROM url_clicks uc
JOIN urls u ON uc.url_id = u.id
WHERE u.user_id = $1 AND uc.browser IS NOT NULL
GROUP BY browser
ORDER BY clicks DESC
LIMIT 10;

-- name: GetUserClicksByCountry :many
SELECT 
    country,
    COUNT(*) as clicks
FROM url_clicks uc
JOIN urls u ON uc.url_id = u.id
WHERE u.user_id = $1 AND uc.country IS NOT NULL
GROUP BY country
ORDER BY clicks DESC
LIMIT 10;

-- name: GetUserClicksByDevice :many
SELECT 
    device_type,
    COUNT(*) as clicks
FROM url_clicks uc
JOIN urls u ON uc.url_id = u.id
WHERE u.user_id = $1 AND uc.device_type IS NOT NULL
GROUP BY device_type
ORDER BY clicks DESC;

-- name: GetUserClicksByDate :many
SELECT
    DATE(clicked_at) as date,
    COUNT(*) as clicks
FROM url_clicks uc
JOIN urls u ON uc.url_id = u.id
WHERE u.user_id = $1
GROUP BY DATE(clicked_at)
ORDER BY date DESC
LIMIT 30;

-- name: GetURLClicksByReferrerAndDate :many
SELECT
    DATE(clicked_at) as date,
    referer,
    COUNT(*) as clicks
FROM url_clicks
WHERE url_id = $1
GROUP BY DATE(clicked_at), referer
ORDER BY date DESC, clicks DESC;

-- name: DeleteOldAnalytics :exec
DELETE FROM url_clicks
WHERE url_id IN (
    SELECT id FROM urls WHERE user_id = $1
)
AND clicked_at < $2;

-- name: GetMonthlyClickStats :one
SELECT
    COUNT(*) as monthly_clicks
FROM url_clicks uc
JOIN urls u ON uc.url_id = u.id
WHERE u.user_id = $1
    AND DATE_TRUNC('month', uc.clicked_at) = DATE_TRUNC('month', CURRENT_DATE);