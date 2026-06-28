ALTER TABLE organizations
    DROP COLUMN IF EXISTS ldap_url,
    DROP COLUMN IF EXISTS ldap_bind_dn,
    DROP COLUMN IF EXISTS ldap_bind_pass_enc,
    DROP COLUMN IF EXISTS ldap_base_dn,
    DROP COLUMN IF EXISTS ldap_user_filter,
    DROP COLUMN IF EXISTS ldap_group_filter,
    DROP COLUMN IF EXISTS ldap_tls;
