CREATE TABLE retention_config (
    org_id                 UUID PRIMARY KEY REFERENCES organizations(id) ON DELETE CASCADE,
    -- Retention in Tagen, 0 = deaktiviert
    audit_log_days         INT NOT NULL DEFAULT 365,
    findings_resolved_days INT NOT NULL DEFAULT 180,
    notifications_days     INT NOT NULL DEFAULT 90,
    scan_history_days      INT NOT NULL DEFAULT 365,
    -- E-Mail-Digest Einstellungen
    digest_enabled         BOOLEAN NOT NULL DEFAULT false,
    digest_hour            SMALLINT NOT NULL DEFAULT 8,  -- UTC Stunde (0-23)
    updated_at             TIMESTAMPTZ NOT NULL DEFAULT now()
);
