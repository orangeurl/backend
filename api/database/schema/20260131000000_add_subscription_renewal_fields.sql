-- +goose Up
-- +goose StatementBegin

-- Add new columns for subscription renewal tracking
ALTER TABLE subscriptions 
ADD COLUMN IF NOT EXISTS billing_interval VARCHAR(20) DEFAULT 'month',
ADD COLUMN IF NOT EXISTS urls_created_this_period INTEGER DEFAULT 0,
ADD COLUMN IF NOT EXISTS url_usage_reset_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
ADD COLUMN IF NOT EXISTS failed_payment_count INTEGER DEFAULT 0,
ADD COLUMN IF NOT EXISTS last_payment_failure_at TIMESTAMP WITH TIME ZONE;

-- Add index for efficient renewal queries
CREATE INDEX IF NOT EXISTS idx_subscriptions_period_end ON subscriptions(current_period_end) WHERE status = 'active';
CREATE INDEX IF NOT EXISTS idx_subscriptions_status ON subscriptions(status);

-- Initialize url_usage_reset_at for existing subscriptions
UPDATE subscriptions 
SET url_usage_reset_at = COALESCE(current_period_start, created_at)
WHERE url_usage_reset_at IS NULL;

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

DROP INDEX IF EXISTS idx_subscriptions_period_end;
DROP INDEX IF EXISTS idx_subscriptions_status;

ALTER TABLE subscriptions 
DROP COLUMN IF EXISTS billing_interval,
DROP COLUMN IF EXISTS urls_created_this_period,
DROP COLUMN IF EXISTS url_usage_reset_at,
DROP COLUMN IF EXISTS failed_payment_count,
DROP COLUMN IF EXISTS last_payment_failure_at;

-- +goose StatementEnd
