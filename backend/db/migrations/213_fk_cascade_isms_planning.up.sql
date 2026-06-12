-- 213_fk_cascade_isms_planning.up.sql
-- Add ON DELETE CASCADE to org_id columns in ISMS planning tables.

-- ck_bcp_plans (ck_bcp_tests cascade from plan_id, covered transitively)
DELETE FROM ck_bcp_plans WHERE org_id NOT IN (SELECT id FROM organizations);
ALTER TABLE ck_bcp_plans
    ADD CONSTRAINT fk_ck_bcp_plans_org
    FOREIGN KEY (org_id) REFERENCES organizations(id) ON DELETE CASCADE;

-- ck_protection_need_assessments
DELETE FROM ck_protection_need_assessments WHERE org_id NOT IN (SELECT id FROM organizations);
ALTER TABLE ck_protection_need_assessments
    ADD CONSTRAINT fk_ck_protection_need_assessments_org
    FOREIGN KEY (org_id) REFERENCES organizations(id) ON DELETE CASCADE;
