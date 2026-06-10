-- S67-5: Read-Only Auditor-Rolle + Audit-Paket-Export
-- Extends ck_auditor_links with description and framework filter.

ALTER TABLE ck_auditor_links
    ADD COLUMN IF NOT EXISTS description        TEXT,
    ADD COLUMN IF NOT EXISTS allowed_frameworks TEXT[] NOT NULL DEFAULT '{}';
