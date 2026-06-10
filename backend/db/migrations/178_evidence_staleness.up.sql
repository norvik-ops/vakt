-- S67-4: Evidence-Staleness-Scoring (ISO 27001 Clause 9.1)
-- Adds max-age and staleness tracking to compliance controls.

ALTER TABLE ck_controls
    ADD COLUMN IF NOT EXISTS evidence_max_age_days  INTEGER,
    ADD COLUMN IF NOT EXISTS evidence_status        TEXT
        CHECK (evidence_status IN ('ok', 'stale', 'missing', 'na'))
        DEFAULT 'missing',
    ADD COLUMN IF NOT EXISTS evidence_last_updated  TIMESTAMPTZ,
    ADD COLUMN IF NOT EXISTS evidence_expires_at    TIMESTAMPTZ;

-- Index for fast staleness queries (excludes n/a controls)
CREATE INDEX IF NOT EXISTS idx_ck_controls_evidence_status
    ON ck_controls (org_id, evidence_status)
    WHERE evidence_status != 'na';

-- Default max-age for all existing controls: 180 days.
-- Can be customised per-control in the UI.
UPDATE ck_controls
SET evidence_max_age_days = 180
WHERE evidence_max_age_days IS NULL;
