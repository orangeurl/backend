-- +goose Up
ALTER TABLE subscriptions
  ADD COLUMN IF NOT EXISTS billing_interval VARCHAR(20) DEFAULT 'monthly',
  ADD COLUMN IF NOT EXISTS cancel_at_period_end BOOLEAN DEFAULT FALSE,
  ADD COLUMN IF NOT EXISTS currency VARCHAR(10) DEFAULT 'USD';

CREATE UNIQUE INDEX IF NOT EXISTS idx_subscriptions_dp_sub_id
  ON subscriptions (dodopayments_subscription_id)
  WHERE dodopayments_subscription_id IS NOT NULL;

CREATE INDEX IF NOT EXISTS idx_subscriptions_dp_customer_id
  ON subscriptions (dodopayments_customer_id);

-- +goose Down
ALTER TABLE subscriptions
  DROP COLUMN IF EXISTS billing_interval,
  DROP COLUMN IF EXISTS cancel_at_period_end,
  DROP COLUMN IF EXISTS currency;

DROP INDEX IF EXISTS idx_subscriptions_dp_sub_id;
DROP INDEX IF EXISTS idx_subscriptions_dp_customer_id;


