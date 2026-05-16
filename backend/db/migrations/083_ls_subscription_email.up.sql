-- Copyright (c) 2026 NorvikOps. All rights reserved.
-- SPDX-License-Identifier: Elastic-2.0

-- Add customer_email to ls_subscriptions so cancellation/refund handlers can
-- resolve the real org_id via users → org_members instead of using the bogus
-- gen_random_uuid() placeholder that was stored previously.
ALTER TABLE ls_subscriptions
    ADD COLUMN IF NOT EXISTS customer_email TEXT NOT NULL DEFAULT '';

CREATE INDEX IF NOT EXISTS idx_ls_subscriptions_email ON ls_subscriptions (customer_email);
