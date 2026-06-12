-- 210_fk_cascade_billing.up.sql
-- Add ON DELETE CASCADE to org_id columns in subscription/licensing tables.
-- These tables had no FK to organizations, so deleting a demo org left orphan rows.
-- Pattern: clean up orphans first, then add the constraint.

-- notification_alert_state
DELETE FROM notification_alert_state WHERE org_id NOT IN (SELECT id FROM organizations);
ALTER TABLE notification_alert_state
    ADD CONSTRAINT fk_notification_alert_state_org
    FOREIGN KEY (org_id) REFERENCES organizations(id) ON DELETE CASCADE;

-- ls_subscriptions
DELETE FROM ls_subscriptions WHERE org_id NOT IN (SELECT id FROM organizations);
ALTER TABLE ls_subscriptions
    ADD CONSTRAINT fk_ls_subscriptions_org
    FOREIGN KEY (org_id) REFERENCES organizations(id) ON DELETE CASCADE;

-- ls_revoked_subscriptions
DELETE FROM ls_revoked_subscriptions WHERE org_id NOT IN (SELECT id FROM organizations);
ALTER TABLE ls_revoked_subscriptions
    ADD CONSTRAINT fk_ls_revoked_subscriptions_org
    FOREIGN KEY (org_id) REFERENCES organizations(id) ON DELETE CASCADE;

-- license_keys
DELETE FROM license_keys WHERE org_id NOT IN (SELECT id FROM organizations);
ALTER TABLE license_keys
    ADD CONSTRAINT fk_license_keys_org
    FOREIGN KEY (org_id) REFERENCES organizations(id) ON DELETE CASCADE;
