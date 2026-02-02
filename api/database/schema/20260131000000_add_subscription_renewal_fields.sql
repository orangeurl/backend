-- +goose Up
-- Add billing interval to track monthly vs annual subscriptions
ALTER TABLE subscriptions ADD COLUMN billing_interval VARCHAR(20) DEFAULT 'monthly';

-- Add URL usage tracking for monthly resets
ALTER TABLE subscriptions ADD COLUMN urls_created_this_period INT DEFAULT 0;

-- Add last reset date to track when URL count was last reset
ALTER TABLE subscriptions ADD COLUMN url_usage_reset_at TIMESTAMP DEFAULT NOW();

-- Add payment failure tracking
ALTER TABLE subscriptions ADD COLUMN failed_payment_count INT DEFAULT 0;
ALTER TABLE subscriptions ADD COLUMN last_payment_failure_at TIMESTAMP;

-- Create index for efficient renewal queries
CREATE INDEX idx_subscriptions_period_end ON subscriptions(current_period_end) WHERE status = 'active';
CREATE INDEX idx_subscriptions_status ON subscriptions(status);

-- +goose Down
DROP INDEX IF EXISTS idx_subscriptions_status;
DROP INDEX IF EXISTS idx_subscriptions_period_end;
ALTER TABLE subscriptions DROP COLUMN IF EXISTS last_payment_failure_at;
ALTER TABLE subscriptions DROP COLUMN IF EXISTS failed_payment_count;
ALTER TABLE subscriptions DROP COLUMN IF EXISTS url_usage_reset_at;
ALTER TABLE subscriptions DROP COLUMN IF EXISTS urls_created_this_period;
ALTER TABLE subscriptions DROP COLUMN IF EXISTS billing_interval;
