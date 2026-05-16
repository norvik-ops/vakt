-- Copyright (c) 2026 NorvikOps. All rights reserved.
-- SPDX-License-Identifier: Elastic-2.0

-- LemonSqueezy subscription tracking: maps ls_subscription_id → org_id for cancellation handling.
CREATE TABLE IF NOT EXISTS ls_subscriptions (
    id                BIGSERIAL    PRIMARY KEY,
    org_id            UUID         NOT NULL,
    ls_subscription_id TEXT        NOT NULL UNIQUE,
    tier              TEXT         NOT NULL DEFAULT 'pro',
    status            TEXT         NOT NULL DEFAULT 'active',  -- active | cancelled | expired | refunded
    created_at        TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at        TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_ls_subscriptions_org_id ON ls_subscriptions (org_id);

-- Revocation blocklist: orgs whose subscription was cancelled/refunded lose Pro features.
CREATE TABLE IF NOT EXISTS ls_revoked_subscriptions (
    org_id     UUID        PRIMARY KEY,
    reason     TEXT        NOT NULL DEFAULT 'cancelled',
    revoked_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- License keys stored via the /api/v1/license/activate endpoint.
CREATE TABLE IF NOT EXISTS license_keys (
    id           BIGSERIAL   PRIMARY KEY,
    org_id       UUID        NOT NULL UNIQUE,
    key_value    TEXT        NOT NULL,
    activated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    activated_by UUID        NULL  -- user_id who activated it
);
