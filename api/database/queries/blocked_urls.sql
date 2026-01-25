-- name: BlockURL :one
-- Admin: Block a URL by short_id (for abuse reports)
INSERT INTO blocked_urls (short_id, original_url, block_reason, blocked_by, abuse_report_ref)
VALUES ($1, $2, $3, $4, $5)
ON CONFLICT (short_id) DO UPDATE SET
    block_reason = EXCLUDED.block_reason,
    abuse_report_ref = EXCLUDED.abuse_report_ref,
    blocked_by = EXCLUDED.blocked_by,
    created_at = NOW()
RETURNING *;

-- name: UnblockURL :exec
-- Admin: Unblock a URL
DELETE FROM blocked_urls WHERE short_id = $1;

-- name: IsURLBlocked :one
-- Check if a URL is blocked (used during resolution)
SELECT EXISTS(SELECT 1 FROM blocked_urls WHERE short_id = $1) AS is_blocked;

-- name: GetBlockedURL :one
-- Get blocked URL details
SELECT * FROM blocked_urls WHERE short_id = $1;

-- name: ListBlockedURLs :many
-- List all blocked URLs (admin view)
SELECT * FROM blocked_urls ORDER BY created_at DESC;

-- name: GetBlockedURLCount :one
-- Count total blocked URLs
SELECT COUNT(*) FROM blocked_urls;
