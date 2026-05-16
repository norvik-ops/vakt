CREATE TABLE IF NOT EXISTS po_dsr (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id          UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    requester_name  TEXT NOT NULL,
    requester_email TEXT NOT NULL,
    type            TEXT NOT NULL, -- 'access' | 'erasure' | 'portability' | 'objection' | 'rectification'
    description     TEXT,
    status          TEXT NOT NULL DEFAULT 'open', -- 'open' | 'in_progress' | 'completed' | 'rejected'
    due_date        DATE,          -- 30-day deadline from received_at
    received_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
    completed_at    TIMESTAMPTZ,
    notes           TEXT,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_po_dsr_org ON po_dsr(org_id, status, received_at DESC);
