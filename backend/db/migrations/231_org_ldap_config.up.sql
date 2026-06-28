ALTER TABLE organizations
    ADD COLUMN IF NOT EXISTS ldap_url           TEXT,
    ADD COLUMN IF NOT EXISTS ldap_bind_dn       TEXT,
    ADD COLUMN IF NOT EXISTS ldap_bind_pass_enc BYTEA,
    ADD COLUMN IF NOT EXISTS ldap_base_dn       TEXT,
    ADD COLUMN IF NOT EXISTS ldap_user_filter   TEXT,
    ADD COLUMN IF NOT EXISTS ldap_group_filter  TEXT,
    ADD COLUMN IF NOT EXISTS ldap_tls           BOOLEAN NOT NULL DEFAULT false;
