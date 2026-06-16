-- S88-3: Gefährdungs-/Maßnahmen-Katalog (Risk-Catalog).
-- Records the provenance of a risk created from the embedded threat library so
-- catalog origin + later catalog updates stay traceable.

CREATE TABLE ck_threat_library_links (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id        UUID NOT NULL,
    risk_id       UUID NOT NULL REFERENCES ck_risks(id) ON DELETE CASCADE,
    catalog_id    TEXT NOT NULL,          -- threat-library.json item id, e.g. "T-RANSOMWARE"
    catalog_version TEXT NOT NULL DEFAULT '',
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX idx_ck_threat_library_links_org_id ON ck_threat_library_links (org_id);
CREATE INDEX idx_ck_threat_library_links_risk_id ON ck_threat_library_links (risk_id);
