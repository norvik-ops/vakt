-- 121: revoked_at Spalte für ck_auditor_links.
--
-- Hintergrund: Der existierende Code (Repository.RevokeAuditorLink,
-- Repository.GetAuditorLinkByHash, Repository.GetAuditorLinkByID,
-- Repository.ListAuditorLinks) selektiert und schreibt die Spalte
-- revoked_at, sie wurde aber nie via Migration angelegt. Bei Aufrufen
-- gegen Postgres würde das fehlschlagen („column "revoked_at" does not
-- exist").
--
-- Migration ist idempotent (IF NOT EXISTS) und additiv — keine Daten-
-- Auswirkung auf bestehende Zeilen (alle revoked_at = NULL → „nicht
-- widerrufen", was der erwarteten Semantik entspricht).

ALTER TABLE ck_auditor_links
    ADD COLUMN IF NOT EXISTS revoked_at TIMESTAMPTZ;

-- Partial-Index für schnelles „nur aktive Auditor-Links auflisten".
CREATE INDEX IF NOT EXISTS idx_ck_auditor_links_active
    ON ck_auditor_links(org_id, created_at DESC)
    WHERE revoked_at IS NULL;
