-- Remove the misleading org_id column from ls_subscriptions
-- (cancellation now resolves org via customer_email → users → org_members)
ALTER TABLE ls_subscriptions DROP COLUMN IF EXISTS org_id;
DROP INDEX IF EXISTS idx_ls_subscriptions_org_id;
