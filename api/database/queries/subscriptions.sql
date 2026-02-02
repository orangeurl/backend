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

-- name: UpdateSubscriptionSetDPIDs :one
UPDATE subscriptions
SET
  dodopayments_subscription_id = COALESCE($2, dodopayments_subscription_id),
  dodopayments_customer_id = COALESCE($3, dodopayments_customer_id),
  updated_at = NOW()
WHERE user_id = $1
RETURNING *;

-- name: ListSubscriptions :many
SELECT * FROM subscriptions ORDER BY created_at DESC;

-- name: GetSubscriptionsForRenewal :many
SELECT * FROM subscriptions 
WHERE status = 'active' 
AND current_period_end <= NOW() + INTERVAL '1 hour'
ORDER BY current_period_end ASC;

-- name: ResetSubscriptionPeriod :one
UPDATE subscriptions 
SET 
    urls_created_this_period = 0,
    url_usage_reset_at = NOW(),
    current_period_start = $2,
    current_period_end = $3,
    updated_at = NOW()
WHERE user_id = $1
RETURNING *;

-- name: IncrementUrlUsage :one
UPDATE subscriptions 
SET urls_created_this_period = urls_created_this_period + 1, updated_at = NOW()
WHERE user_id = $1
RETURNING *;

-- name: GetSubscriptionUsage :one
SELECT urls_created_this_period, url_usage_reset_at, billing_interval, current_period_end 
FROM subscriptions 
WHERE user_id = $1;

-- name: DowngradeToFree :one
UPDATE subscriptions 
SET 
    plan_id = 'free',
    status = 'cancelled',
    updated_at = NOW()
WHERE user_id = $1
RETURNING *;

-- name: RecordPaymentFailure :one
UPDATE subscriptions 
SET 
    failed_payment_count = failed_payment_count + 1,
    last_payment_failure_at = NOW(),
    updated_at = NOW()
WHERE user_id = $1
RETURNING *;

-- name: UpdateSubscriptionStatus :one
UPDATE subscriptions 
SET status = $2, updated_at = NOW()
WHERE user_id = $1
RETURNING *;

-- name: GetActiveSubscriptionsCount :one
SELECT COUNT(*) as count FROM subscriptions WHERE status = 'active';

-- name: UpdateSubscriptionWithBilling :one
UPDATE subscriptions 
SET 
    plan_id = $2,
    status = $3,
    billing_interval = $4,
    current_period_start = $5,
    current_period_end = $6,
    urls_created_this_period = 0,
    url_usage_reset_at = NOW(),
    failed_payment_count = 0,
    updated_at = NOW()
WHERE user_id = $1
RETURNING *;