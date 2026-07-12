ALTER TABLE billing_quote_requests
    ADD COLUMN IF NOT EXISTS renewal_token UUID NOT NULL DEFAULT gen_random_uuid();
CREATE UNIQUE INDEX IF NOT EXISTS idx_billing_quote_requests_renewal_token
    ON billing_quote_requests (renewal_token);

DROP INDEX IF EXISTS idx_billing_quote_requests_portal_token;
ALTER TABLE billing_quote_requests DROP COLUMN IF EXISTS portal_token_hash;

DROP INDEX IF EXISTS idx_billing_licenses_renewal_token;
ALTER TABLE billing_licenses
    DROP COLUMN IF EXISTS renewal_token,
    DROP COLUMN IF EXISTS last_seen_at,
    DROP COLUMN IF EXISTS revoked_at;
