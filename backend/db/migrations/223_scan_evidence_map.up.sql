-- S88-8: Scan→Comply-Evidence-Brücke. Idempotency map so a re-scan that
-- re-emits the same finding does not attach duplicate evidence to the same
-- control. finding_id is an opaque key from vaktscan (no FK — module isolation:
-- vaktcomply does not reference vaktscan tables).

CREATE TABLE ck_scan_evidence_map (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id      UUID NOT NULL,
    finding_id  TEXT NOT NULL,
    control_id  UUID NOT NULL REFERENCES ck_controls(id) ON DELETE CASCADE,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (org_id, finding_id, control_id)
);
CREATE INDEX idx_ck_scan_evidence_map_org_id ON ck_scan_evidence_map (org_id);
