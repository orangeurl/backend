-- name: CreateSubscription :one
INSERT INTO subscriptions (user_id, plan_id, status, dodopayments_customer_id)
VALUES ($1, $2, $3, $4)
RETURNING *;

-- name: GetUserSubscription :one
SELECT * FROM subscriptions WHERE user_id = $1;

-- name: UpdateSubscription :one
UPDATE subscriptions 
SET plan_id = $2, status = $3, current_period_start = $4, current_period_end = $5, updated_at = NOW()
WHERE user_id = $1
RETURNING *;    

-- name: ListSubscriptions :many
SELECT * FROM subscriptions ORDER BY created_at DESC;