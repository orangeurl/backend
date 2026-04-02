-- name: CreateURL :one
INSERT INTO urls (user_id, short_id, original_url, expiry, is_active, password_hash, is_locked, team_id)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
RETURNING *;

-- name: GetURL :one
SELECT * FROM urls WHERE id = $1;

-- name: GetURLByShortID :one
SELECT * FROM urls WHERE short_id = $1 AND is_active = TRUE;

-- name: GetUserURLs :many
SELECT * FROM urls WHERE user_id = $1 AND is_active = TRUE
ORDER BY created_at DESC;

-- name: GetUserURLCount :one
SELECT COUNT(*) FROM urls WHERE user_id = $1 AND is_active = TRUE AND team_id IS NULL;

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
    AND team_id IS NULL
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

-- name: GetUserAndTeamURLs :many
-- Get URLs owned by user OR shared with their team
SELECT DISTINCT u.* FROM urls u
LEFT JOIN team_members tm ON u.team_id = tm.team_id
WHERE (u.user_id = $1 OR tm.user_id = $1) AND u.is_active = TRUE
ORDER BY u.created_at DESC;

-- name: GetUserAndTeamURLCount :one
-- Count URLs owned by user OR shared with their team
SELECT COUNT(DISTINCT u.id) FROM urls u
LEFT JOIN team_members tm ON u.team_id = tm.team_id
WHERE (u.user_id = $1 OR tm.user_id = $1) AND u.is_active = TRUE;

-- name: GetUserPersonalURLs :many
-- Get ONLY URLs owned by user (NOT shared with team)
SELECT * FROM urls
WHERE user_id = $1 AND is_active = TRUE AND team_id IS NULL
ORDER BY created_at DESC;

-- name: GetUserTeamURLs :many
-- Get ONLY URLs shared with user's team
SELECT DISTINCT u.* FROM urls u
INNER JOIN team_members tm ON u.team_id = tm.team_id
WHERE tm.user_id = $1 AND u.is_active = TRUE
ORDER BY u.created_at DESC;

-- name: DeactivateURLByShortID :exec
-- Admin: Deactivate a URL by short_id (used when blocking abusive URLs)
UPDATE urls SET is_active = FALSE, updated_at = NOW() WHERE short_id = $1;

-- name: AdminGetAllURLs :many
-- Admin: Get all URLs (including inactive) for admin management panel
SELECT u.*, COALESCE(usr.email, '') as owner_email
FROM urls u
LEFT JOIN users usr ON u.user_id = usr.id
ORDER BY u.created_at DESC;

-- name: AdminHardDeleteURL :exec
-- Admin: Permanently delete a URL record from the database
DELETE FROM urls WHERE id = $1;

-- name: AdminDeleteURLClicks :exec
-- Admin: Delete all click analytics for a URL before hard-deleting it
DELETE FROM url_clicks WHERE url_id = $1;

-- name: AdminGetURLByID :one
-- Admin: Get any URL by ID (including inactive ones, no user filter)
SELECT * FROM urls WHERE id = $1;