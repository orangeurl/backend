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

-- name: GetURLAnalyticsWithLimit :many
SELECT * FROM url_clicks WHERE url_id = $1
ORDER BY clicked_at DESC
LIMIT $2;

-- name: GetUserAnalyticsWithDateRange :many
SELECT uc.* FROM url_clicks uc
JOIN urls u ON uc.url_id = u.id
WHERE u.user_id = $1 AND uc.clicked_at >= $2 AND uc.clicked_at <= $3
ORDER BY uc.clicked_at DESC;

-- name: GetURLClickCount :one
SELECT COUNT(*) as click_count FROM url_clicks WHERE url_id = $1;

-- name: GetUserTotalClicks :one
SELECT COUNT(*) as total_clicks FROM url_clicks uc
JOIN urls u ON uc.url_id = u.id
WHERE u.user_id = $1;

-- name: GetURLClicksByCountry :many
SELECT country, COUNT(*) as click_count 
FROM url_clicks 
WHERE url_id = $1 AND country IS NOT NULL
GROUP BY country
ORDER BY click_count DESC;

-- name: GetUserClicksByCountry :many
SELECT uc.country, COUNT(*) as click_count 
FROM url_clicks uc
JOIN urls u ON uc.url_id = u.id
WHERE u.user_id = $1 AND uc.country IS NOT NULL
GROUP BY uc.country
ORDER BY click_count DESC;

-- name: GetURLClicksByDevice :many
SELECT device_type, COUNT(*) as click_count 
FROM url_clicks 
WHERE url_id = $1 AND device_type IS NOT NULL
GROUP BY device_type
ORDER BY click_count DESC;

-- name: GetUserClicksByDevice :many
SELECT uc.device_type, COUNT(*) as click_count 
FROM url_clicks uc
JOIN urls u ON uc.url_id = u.id
WHERE u.user_id = $1 AND uc.device_type IS NOT NULL
GROUP BY uc.device_type
ORDER BY click_count DESC;

-- name: GetURLClicksByBrowser :many
SELECT browser, COUNT(*) as click_count 
FROM url_clicks 
WHERE url_id = $1 AND browser IS NOT NULL
GROUP BY browser
ORDER BY click_count DESC;

-- name: GetUserClicksByBrowser :many
SELECT uc.browser, COUNT(*) as click_count 
FROM url_clicks uc
JOIN urls u ON uc.url_id = u.id
WHERE u.user_id = $1 AND uc.browser IS NOT NULL
GROUP BY uc.browser
ORDER BY click_count DESC;

-- name: GetURLClicksByOS :many
SELECT os, COUNT(*) as click_count 
FROM url_clicks 
WHERE url_id = $1 AND os IS NOT NULL
GROUP BY os
ORDER BY click_count DESC;

-- name: GetUserClicksByOS :many
SELECT uc.os, COUNT(*) as click_count 
FROM url_clicks uc
JOIN urls u ON uc.url_id = u.id
WHERE u.user_id = $1 AND uc.os IS NOT NULL
GROUP BY uc.os
ORDER BY click_count DESC;

-- name: GetURLClicksOverTime :many
SELECT DATE(clicked_at) as date, COUNT(*) as click_count
FROM url_clicks
WHERE url_id = $1 AND clicked_at >= $2
GROUP BY DATE(clicked_at)
ORDER BY date DESC;

-- name: GetUserClicksOverTime :many
SELECT DATE(uc.clicked_at) as date, COUNT(*) as click_count
FROM url_clicks uc
JOIN urls u ON uc.url_id = u.id
WHERE u.user_id = $1 AND uc.clicked_at >= $2
GROUP BY DATE(uc.clicked_at)
ORDER BY date DESC;

-- name: GetUserTopURLs :many
SELECT u.id, u.short_id, u.original_url, COUNT(uc.id) as click_count
FROM urls u
LEFT JOIN url_clicks uc ON u.id = uc.url_id
WHERE u.user_id = $1 AND u.is_active = TRUE
GROUP BY u.id, u.short_id, u.original_url
ORDER BY click_count DESC
LIMIT $2;

-- name: GetURLReferrers :many
SELECT referer, COUNT(*) as click_count
FROM url_clicks
WHERE url_id = $1 AND referer IS NOT NULL AND referer != ''
GROUP BY referer
ORDER BY click_count DESC
LIMIT $2;

-- name: GetUserBotClicksPercentage :one
SELECT 
    COUNT(CASE WHEN uc.is_bot = TRUE THEN 1 END)::FLOAT / NULLIF(COUNT(*), 0) * 100 as bot_percentage
FROM url_clicks uc
JOIN urls u ON uc.url_id = u.id
WHERE u.user_id = $1;

-- name: DeleteOldClicksByDate :exec
DELETE FROM url_clicks
WHERE url_id IN (
    SELECT id FROM urls WHERE user_id = $1
) AND clicked_at < $2;