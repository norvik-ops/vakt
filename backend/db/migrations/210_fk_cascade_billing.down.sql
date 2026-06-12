ALTER TABLE license_keys DROP CONSTRAINT IF EXISTS fk_license_keys_org;
ALTER TABLE ls_revoked_subscriptions DROP CONSTRAINT IF EXISTS fk_ls_revoked_subscriptions_org;
ALTER TABLE notification_alert_state DROP CONSTRAINT IF EXISTS fk_notification_alert_state_org;
