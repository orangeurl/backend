-- ============================================
-- IMMEDIATE ACTION: Block abusive URL vPg9uc
-- Run this script in your PostgreSQL database
-- ============================================

-- 1. First, create the blocked_urls table if it doesn't exist yet
-- (Run the migration first, or use this as a fallback)
CREATE TABLE IF NOT EXISTS blocked_urls(
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    short_id VARCHAR(255) NOT NULL UNIQUE,
    original_url TEXT,
    block_reason TEXT NOT NULL,
    blocked_by UUID REFERENCES users(id),
    abuse_report_ref TEXT,
    created_at TIMESTAMP DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_blocked_urls_short_id ON blocked_urls(short_id);

-- 2. Block the reported URL
INSERT INTO blocked_urls (short_id, block_reason, abuse_report_ref)
VALUES (
    'vPg9uc',
    'Abuse report - Phishing/Malware redirect to malicious domain',
    'Porkbun abuse report - January 2026'
)
ON CONFLICT (short_id) DO UPDATE SET
    block_reason = EXCLUDED.block_reason,
    abuse_report_ref = EXCLUDED.abuse_report_ref,
    created_at = NOW();

-- 3. Deactivate the URL in the urls table
UPDATE urls SET is_active = FALSE, updated_at = NOW() WHERE short_id = 'vPg9uc';

-- 4. Verify the block was successful
SELECT * FROM blocked_urls WHERE short_id = 'vPg9uc';

-- ============================================
-- IMPORTANT: Also run this command in Redis
-- to immediately stop redirects:
-- 
-- redis-cli DEL vPg9uc
-- ============================================
