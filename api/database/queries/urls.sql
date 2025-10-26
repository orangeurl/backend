-- name: CreateURL :one
INSERT INTO urls (user_id, short_id, original_url, expiry, is_active)
VALUES ($1, $2, $3, $4, $5)
RETURNING *;

-- name: GetURLByShortID :one
SELECT * FROM urls WHERE short_id = $1 AND is_active = TRUE;

-- name: GetUserURLs :many
SELECT * FROM urls WHERE user_id = $1 AND is_active = TRUE
ORDER BY created_at DESC;

-- name: UpdateURL :one
UPDATE urls 
SET original_url = $2, expiry = $3, is_active = $4, updated_at = NOW()
WHERE id = $1
RETURNING *;

-- name: DeleteURL :exec
UPDATE urls SET is_active = FALSE WHERE id = $1 AND user_id = $2;

-- name: ListURLs :many
SELECT * FROM urls ORDER BY created_at DESC;

-- name: GetUserURLCount :one
SELECT COUNT(*) FROM urls WHERE user_id = $1 AND is_active = TRUE;

-- name: GetUserURLsWithStats :many
SELECT 
    u.id,
    u.user_id,
    u.short_id,
    u.original_url,
    u.expiry,
    u.is_active,
    u.created_at,
    u.updated_at,
    COUNT(uc.id) as click_count
FROM urls u
LEFT JOIN url_clicks uc ON u.id = uc.url_id
WHERE u.user_id = $1 AND u.is_active = TRUE
GROUP BY u.id
ORDER BY u.created_at DESC;