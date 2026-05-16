CREATE TABLE IF NOT EXISTS backup_log (
    id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id     UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    backed_up_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
