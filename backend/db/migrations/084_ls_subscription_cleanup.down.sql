ALTER TABLE ls_subscriptions ADD COLUMN IF NOT EXISTS org_id UUID NOT NULL DEFAULT gen_random_uuid();
CREATE INDEX IF NOT EXISTS idx_ls_subscriptions_org_id ON ls_subscriptions(org_id);
