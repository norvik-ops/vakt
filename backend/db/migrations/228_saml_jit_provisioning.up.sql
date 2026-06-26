-- S105-3: JIT provisioning toggle for SAML direct SP.
-- When false, users must be pre-provisioned before SAML login works.
ALTER TABLE org_saml_configs
    ADD COLUMN jit_provisioning BOOLEAN NOT NULL DEFAULT TRUE;
