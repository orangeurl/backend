-- name: CreateWebhook :one
INSERT INTO webhooks (
    user_id,
    url,
    events,
    secret,
    is_active
) VALUES (
    $1, $2, $3, $4, $5
) RETURNING *;

-- name: GetWebhookByID :one
SELECT * FROM webhooks
WHERE id = $1 AND user_id = $2
LIMIT 1;

-- name: ListUserWebhooks :many
SELECT * FROM webhooks
WHERE user_id = $1
ORDER BY created_at DESC;

-- name: ListActiveWebhooksByEvent :many
SELECT * FROM webhooks
WHERE is_active = TRUE AND events && $1;

-- name: ListActiveWebhooksByEventAndUser :many
SELECT * FROM webhooks
WHERE is_active = TRUE AND events && $1 AND user_id = $2;

-- name: UpdateWebhook :one
UPDATE webhooks
SET url = $2, events = $3, updated_at = NOW()
WHERE id = $1 AND user_id = $4
RETURNING *;

-- name: ToggleWebhookStatus :one
UPDATE webhooks
SET is_active = $2, updated_at = NOW()
WHERE id = $1 AND user_id = $3
RETURNING *;

-- name: DeleteWebhook :exec
DELETE FROM webhooks
WHERE id = $1 AND user_id = $2;

-- name: CountUserActiveWebhooks :one
SELECT COUNT(*) FROM webhooks
WHERE user_id = $1 AND is_active = TRUE;

-- Webhook Deliveries

-- name: CreateWebhookDelivery :one
INSERT INTO webhook_deliveries (
    webhook_id,
    event_type,
    payload,
    status
) VALUES (
    $1, $2, $3, $4
) RETURNING *;

-- name: UpdateWebhookDelivery :exec
UPDATE webhook_deliveries
SET status = $2, attempts = $3, response_code = $4, response_body = $5, last_attempt_at = NOW()
WHERE id = $1;

-- name: GetPendingWebhookDeliveries :many
SELECT * FROM webhook_deliveries
WHERE status = 'pending' AND attempts < 3
ORDER BY created_at ASC
LIMIT $1;

-- name: ListWebhookDeliveries :many
SELECT * FROM webhook_deliveries
WHERE webhook_id = $1
ORDER BY created_at DESC
LIMIT $2 OFFSET $3;

-- name: GetWebhookDeliveryStats :one
SELECT
    COUNT(*) as total,
    COUNT(*) FILTER (WHERE status = 'success') as successful,
    COUNT(*) FILTER (WHERE status = 'failed') as failed,
    COUNT(*) FILTER (WHERE status = 'pending') as pending
FROM webhook_deliveries
WHERE webhook_id = $1;
