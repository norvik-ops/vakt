-- 211_fk_cascade_control_tracking.up.sql
-- Add ON DELETE CASCADE to org_id columns in compliance control tracking tables.

-- ck_control_changelog
DELETE FROM ck_control_changelog WHERE org_id NOT IN (SELECT id FROM organizations);
ALTER TABLE ck_control_changelog
    ADD CONSTRAINT fk_ck_control_changelog_org
    FOREIGN KEY (org_id) REFERENCES organizations(id) ON DELETE CASCADE;

-- ck_access_review_campaigns
DELETE FROM ck_access_review_campaigns WHERE org_id NOT IN (SELECT id FROM organizations);
ALTER TABLE ck_access_review_campaigns
    ADD CONSTRAINT fk_ck_access_review_campaigns_org
    FOREIGN KEY (org_id) REFERENCES organizations(id) ON DELETE CASCADE;

-- ck_control_exceptions
DELETE FROM ck_control_exceptions WHERE org_id NOT IN (SELECT id FROM organizations);
ALTER TABLE ck_control_exceptions
    ADD CONSTRAINT fk_ck_control_exceptions_org
    FOREIGN KEY (org_id) REFERENCES organizations(id) ON DELETE CASCADE;

-- ck_evidence_history
DELETE FROM ck_evidence_history WHERE org_id NOT IN (SELECT id FROM organizations);
ALTER TABLE ck_evidence_history
    ADD CONSTRAINT fk_ck_evidence_history_org
    FOREIGN KEY (org_id) REFERENCES organizations(id) ON DELETE CASCADE;
