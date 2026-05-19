CREATE TABLE ck_evidence_history (
    id           UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    evidence_id  UUID        NOT NULL REFERENCES ck_evidence(id) ON DELETE CASCADE,
    org_id       UUID        NOT NULL,
    changed_by   UUID        REFERENCES users(id) ON DELETE SET NULL,
    changed_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    title        TEXT,
    description  TEXT,
    status       TEXT,
    file_url     TEXT,
    change_note  TEXT
);
CREATE INDEX idx_evidence_history_evidence_id ON ck_evidence_history(evidence_id, changed_at DESC);
