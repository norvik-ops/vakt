-- 057 rollback: remove Trust Center extension tables and columns
DROP TABLE IF EXISTS tc_public_policies;
DROP TABLE IF EXISTS tc_certificates;

ALTER TABLE organizations
    DROP COLUMN IF EXISTS trust_center_subprocessors_md,
    DROP COLUMN IF EXISTS trust_center_show_certs,
    DROP COLUMN IF EXISTS trust_center_show_policies,
    DROP COLUMN IF EXISTS trust_center_show_frameworks,
    DROP COLUMN IF EXISTS trust_center_logo_url;
