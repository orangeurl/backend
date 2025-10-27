-- name: CreateURL :one
INSERT INTO urls (user_id, short_id, original_url, expiry, is_active)
VALUES ($1, $2, $3, $4, $5)
RETURNING *;

-- name: GetURLByShortID :one
SELECT * FROM urls WHERE short_id = $1 AND is_active = TRUE;

-- name: GetUserURLs :many
SELECT * FROM urls WHERE user_id = $1 AND is_active = TRUE
ORDER BY created_at DESC;

-- name: GetUserURLCount :one
SELECT COUNT(*) FROM urls WHERE user_id = $1 AND is_active = TRUE;

-- name: UpdateURL :one
UPDATE urls 
SET original_url = $2, expiry = $3, is_active = $4, updated_at = NOW()
WHERE id = $1
RETURNING *;

-- name: DeleteURL :exec
UPDATE urls SET is_active = FALSE WHERE id = $1 AND user_id = $2;

-- name: ListURLs :many
SELECT * FROM urls ORDER BY created_at DESC;