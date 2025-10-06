-- name: CreateUser :one
INSERT INTO users (clerk_id, email, first_name, last_name, avatar_url)
VALUES ($1, $2, $3, $4, $5)
RETURNING *;

-- name: GetUserByClerkID :one
SELECT * FROM users WHERE clerk_id = $1;

-- name: GetUserByID :one
SELECT * FROM users WHERE id = $1;

-- name: GetUserByEmail :one
SELECT * FROM users WHERE email = $1;

-- name: UpdateUser :one
UPDATE users 
SET email = $2, first_name = $3, last_name = $4, avatar_url = $5, updated_at = NOW()
WHERE clerk_id = $1
RETURNING *;

-- name: UpdateUserSubscriptionTier :exec
UPDATE users SET subscription_tier = $2, updated_at = NOW()
WHERE id = $1;

-- name: DeleteUser :exec
DELETE FROM users WHERE clerk_id = $1;

-- name: ListUsers :many
SELECT * FROM users ORDER BY created_at DESC;