-- name: CreateSubscription :one
INSERT INTO subscriptions (user_id, plan_id, status, dodopayments_customer_id, billing_interval, current_period_start, current_period_end)
VALUES ($1, $2, $3, $4, $5, $6, $7)
RETURNING *;

-- name: GetUserSubscription :one
SELECT * FROM subscriptions WHERE user_id = $1;

-- name: GetSubscriptionByDodoCustomerId :one
SELECT * FROM subscriptions WHERE dodopayments_customer_id = $1;

-- name: GetSubscriptionByDodoSubscriptionId :one
SELECT * FROM subscriptions WHERE dodopayments_subscription_id = $1;

-- name: UpdateSubscription :one
UPDATE subscriptions 
SET plan_id = $2, status = $3, current_period_start = $4, current_period_end = $5, updated_at = NOW()
WHERE user_id = $1
RETURNING *;

-- name: UpdateSubscriptionWithBilling :one
UPDATE subscriptions 
SET plan_id = $2, 
    status = $3, 
    current_period_start = $4, 
    current_period_end = $5, 
    billing_interval = $6,
    dodopayments_subscription_id = $7,
    updated_at = NOW()
WHERE user_id = $1
RETURNING *;

-- name: ListSubscriptions :many
SELECT * FROM subscriptions ORDER BY created_at DESC;

-- name: GetSubscriptionsForRenewal :many
-- Get all active subscriptions where the current period has ended
SELECT s.*, u.email, u.clerk_id
FROM subscriptions s
JOIN users u ON s.user_id = u.id
WHERE s.status = 'active' 
  AND s.current_period_end IS NOT NULL 
  AND s.current_period_end <= NOW();

-- name: ResetSubscriptionPeriod :one
-- Reset subscription for a new billing period
UPDATE subscriptions 
SET current_period_start = $2,
    current_period_end = $3,
    urls_created_this_period = 0,
    url_usage_reset_at = NOW(),
    failed_payment_count = 0,
    updated_at = NOW()
WHERE id = $1
RETURNING *;

-- name: IncrementUrlUsage :exec
-- Increment URL usage count for the current period
UPDATE subscriptions 
SET urls_created_this_period = urls_created_this_period + 1,
    updated_at = NOW()
WHERE user_id = $1;

-- name: GetSubscriptionUsage :one
-- Get current period URL usage
SELECT urls_created_this_period, url_usage_reset_at, current_period_end
FROM subscriptions 
WHERE user_id = $1;

-- name: DowngradeToFree :one
-- Downgrade subscription to free tier on payment failure
UPDATE subscriptions 
SET plan_id = 'free',
    status = 'cancelled',
    failed_payment_count = failed_payment_count + 1,
    last_payment_failure_at = NOW(),
    updated_at = NOW()
WHERE id = $1
RETURNING *;

-- name: RecordPaymentFailure :exec
-- Record a payment failure without downgrading (for retry logic)
UPDATE subscriptions 
SET failed_payment_count = failed_payment_count + 1,
    last_payment_failure_at = NOW(),
    updated_at = NOW()
WHERE id = $1;

-- name: UpdateSubscriptionStatus :exec
UPDATE subscriptions 
SET status = $2, updated_at = NOW()
WHERE id = $1;

-- name: GetActiveSubscriptionsCount :one
SELECT COUNT(*) FROM subscriptions WHERE status = 'active' AND plan_id != 'free';