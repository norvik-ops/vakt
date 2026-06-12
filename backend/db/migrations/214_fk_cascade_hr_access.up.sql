-- 214_fk_cascade_hr_access.up.sql
-- Add ON DELETE CASCADE to org_id in hr_access_concepts.
-- hr_access_roles and hr_access_concept_versions cascade from concept_id, covered transitively.

DELETE FROM hr_access_concepts WHERE org_id NOT IN (SELECT id FROM organizations);
ALTER TABLE hr_access_concepts
    ADD CONSTRAINT fk_hr_access_concepts_org
    FOREIGN KEY (org_id) REFERENCES organizations(id) ON DELETE CASCADE;
