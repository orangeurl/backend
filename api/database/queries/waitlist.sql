-- name: CreateWaitlistEntry :one
INSERT INTO waitlist (email)
VALUES ($1)
RETURNING *;

-- name: GetWaitlistEntries :many
SELECT * FROM waitlist ORDER BY created_at DESC;

-- name: DeleteWaitlistEntry :exec
DELETE FROM waitlist WHERE id = $1;