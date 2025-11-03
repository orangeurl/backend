-- name: CreateAPIKey :one
INSERT INTO api_keys (
    user_id,
    key_name,
    api_key_hash,
    api_key_prefix,
    permissions,
    rate_limit_tier,
    expires_at
) VALUES (
    $1, $2, $3, $4, $5, $6, $7
) RETURNING *;

-- name: GetAPIKeyByPrefix :one
SELECT * FROM api_keys
WHERE api_key_prefix = $1 AND is_active = TRUE
LIMIT 1;

-- name: GetAPIKeyByID :one
SELECT * FROM api_keys
WHERE id = $1 AND user_id = $2
LIMIT 1;

-- name: ListUserAPIKeys :many
SELECT * FROM api_keys
WHERE user_id = $1
ORDER BY created_at DESC;

-- name: UpdateAPIKeyLastUsed :exec
UPDATE api_keys
SET last_used_at = NOW()
WHERE id = $1;

-- name: UpdateAPIKeyName :one
UPDATE api_keys
SET key_name = $2, updated_at = NOW()
WHERE id = $1 AND user_id = $3
RETURNING *;

-- name: UpdateAPIKeyPermissions :one
UPDATE api_keys
SET permissions = $2, updated_at = NOW()
WHERE id = $1 AND user_id = $3
RETURNING *;

-- name: DeactivateAPIKey :exec
UPDATE api_keys
SET is_active = FALSE, updated_at = NOW()
WHERE id = $1 AND user_id = $2;

-- name: DeleteAPIKey :exec
DELETE FROM api_keys
WHERE id = $1 AND user_id = $2;

-- name: CountUserActiveAPIKeys :one
SELECT COUNT(*) FROM api_keys
WHERE user_id = $1 AND is_active = TRUE;

-- name: CleanupExpiredAPIKeys :exec
UPDATE api_keys
SET is_active = FALSE, updated_at = NOW()
WHERE expires_at IS NOT NULL AND expires_at < NOW() AND is_active = TRUE;
