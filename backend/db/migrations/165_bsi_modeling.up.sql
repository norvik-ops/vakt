CREATE TABLE ck_bsi_modeling (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    asset_id UUID NOT NULL REFERENCES vb_assets(id) ON DELETE CASCADE,
    control_id UUID NOT NULL REFERENCES ck_controls(id) ON DELETE CASCADE,
    priority TEXT NOT NULL CHECK (priority IN ('R1', 'R2', 'R3')) DEFAULT 'R1',
    justification_for_exclusion TEXT NOT NULL DEFAULT '',
    check_status TEXT CHECK (check_status IN ('yes', 'partial', 'no', 'not_applicable')),
    interview_notes TEXT NOT NULL DEFAULT '',
    site_visit_notes TEXT NOT NULL DEFAULT '',
    created_by UUID NOT NULL REFERENCES users(id),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (org_id, asset_id, control_id)
);
CREATE INDEX idx_ck_bsi_modeling_org_asset ON ck_bsi_modeling (org_id, asset_id);
CREATE INDEX idx_ck_bsi_modeling_org_control ON ck_bsi_modeling (org_id, control_id);
