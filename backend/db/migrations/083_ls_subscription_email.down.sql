-- Copyright (c) 2026 NorvikOps. All rights reserved.
-- SPDX-License-Identifier: Elastic-2.0

DROP INDEX IF EXISTS idx_ls_subscriptions_email;
ALTER TABLE ls_subscriptions DROP COLUMN IF EXISTS customer_email;
