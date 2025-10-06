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