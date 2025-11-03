-- name: CreateURL :one
INSERT INTO urls (user_id, short_id, original_url, expiry, is_active, password_hash, is_locked)
VALUES ($1, $2, $3, $4, $5, $6, $7)
RETURNING *;

-- name: GetURL :one
SELECT * FROM urls WHERE id = $1;

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

-- name: SetURLPassword :one
UPDATE urls
SET password_hash = $2, is_locked = $3, updated_at = NOW()
WHERE id = $1
RETURNING *;

-- name: RemoveURLPassword :one
UPDATE urls
SET password_hash = NULL, is_locked = FALSE, password_attempts = 0, last_password_attempt = NULL, updated_at = NOW()
WHERE id = $1
RETURNING *;

-- name: IncrementPasswordAttempts :exec
UPDATE urls
SET password_attempts = password_attempts + 1, last_password_attempt = NOW()
WHERE short_id = $1;

-- name: ResetPasswordAttempts :exec
UPDATE urls
SET password_attempts = 0, last_password_attempt = NULL
WHERE short_id = $1;

-- name: GetMonthlyURLCount :one
SELECT COUNT(*) FROM urls
WHERE user_id = $1
    AND is_active = TRUE
    AND DATE_TRUNC('month', created_at) = DATE_TRUNC('month', CURRENT_DATE);

-- name: GetUserURLByOriginalURL :one
SELECT * FROM urls
WHERE user_id = $1
    AND original_url = $2
    AND is_active = TRUE
LIMIT 1;

-- name: DeactivateExpiredURLs :exec
UPDATE urls
SET is_active = FALSE, updated_at = NOW()
WHERE expiry IS NOT NULL
    AND expiry < $1
    AND is_active = TRUE;