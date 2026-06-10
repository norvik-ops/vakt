DROP INDEX IF EXISTS idx_polar_subscriptions_renewal_token;

ALTER TABLE polar_subscriptions
    DROP COLUMN IF EXISTS renewal_token,
    DROP COLUMN IF EXISTS license_key;
