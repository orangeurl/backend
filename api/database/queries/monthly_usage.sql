-- name: GetMonthlyUsage :one
SELECT * FROM monthly_usage 
WHERE user_id = $1 AND month = $2;

-- name: IncrementURLCount :one
INSERT INTO monthly_usage (user_id, month, url_count)
VALUES ($1, $2, 1)
ON CONFLICT (user_id, month) 
DO UPDATE SET url_count = monthly_usage.url_count + 1, updated_at = NOW()
RETURNING *;

-- name: IncrementCustomLinkCount :one
INSERT INTO monthly_usage (user_id, month, custom_link_count)
VALUES ($1, $2, 1)
ON CONFLICT (user_id, month)
DO UPDATE SET custom_link_count = monthly_usage.custom_link_count + 1, updated_at = NOW()
RETURNING *;

-- name: IncrementQRCodeCount :one
INSERT INTO monthly_usage (user_id, month, qr_code_count)
VALUES ($1, $2, 1)
ON CONFLICT (user_id, month)
DO UPDATE SET qr_code_count = monthly_usage.qr_code_count + 1, updated_at = NOW()
RETURNING *;

