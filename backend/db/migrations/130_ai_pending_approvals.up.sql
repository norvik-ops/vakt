CREATE TABLE IF NOT EXISTS ai_pending_approvals (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    run_id TEXT NOT NULL,
    org_id UUID NOT NULL,
    user_id UUID,
    tool_name TEXT NOT NULL,
    args JSONB NOT NULL DEFAULT '{}',
    status TEXT NOT NULL DEFAULT 'pending',
    decided_by UUID,
    decided_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_ai_pending_approvals_run_id ON ai_pending_approvals (run_id);
CREATE INDEX IF NOT EXISTS idx_ai_pending_approvals_org ON ai_pending_approvals (org_id, created_at DESC);
