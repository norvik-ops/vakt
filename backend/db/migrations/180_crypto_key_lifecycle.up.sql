-- S67-6: Kryptographie-Schlüssel-Lifecycle (ISO 27001 A.8.24)
-- Central register for cryptographic assets with rotation tracking.

CREATE TABLE IF NOT EXISTS ck_crypto_keys (
    id                     UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id                 UUID        NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    name                   TEXT        NOT NULL,
    key_type               TEXT        NOT NULL CHECK (key_type IN (
        'symmetric', 'asymmetric', 'certificate', 'hmac', 'signing', 'other'
    )),
    algorithm              TEXT        NOT NULL,
    key_length             INTEGER,
    purpose                TEXT        NOT NULL,
    location               TEXT,
    rotation_interval_days INTEGER,
    last_rotation_date     DATE,
    next_rotation_due      DATE,
    expiry_date            DATE,
    is_weak_algorithm      BOOLEAN     NOT NULL DEFAULT false,
    notes                  TEXT,
    created_at             TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at             TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_ck_crypto_keys_org      ON ck_crypto_keys (org_id);
CREATE INDEX IF NOT EXISTS idx_ck_crypto_keys_rotation ON ck_crypto_keys (org_id, next_rotation_due)
    WHERE rotation_interval_days IS NOT NULL;
