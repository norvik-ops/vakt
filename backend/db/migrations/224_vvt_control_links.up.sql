-- S88-9: VVT→ISO/TOM-Control-Verknüpfung. n:m link between a Vakt Privacy VVT
-- entry (Art. 30 processing activity) and a Vakt Comply control. vvt_id is an
-- opaque key (TEXT, no cross-module FK to po_processing_activities — module
-- isolation); vvt_name is denormalised for display. control_id FKs ck_controls.

CREATE TABLE ck_vvt_control_links (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id      UUID NOT NULL,
    vvt_id      TEXT NOT NULL,
    vvt_name    TEXT NOT NULL DEFAULT '',
    control_id  UUID NOT NULL REFERENCES ck_controls(id) ON DELETE CASCADE,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (org_id, vvt_id, control_id)
);
CREATE INDEX idx_ck_vvt_control_links_org_id ON ck_vvt_control_links (org_id);
CREATE INDEX idx_ck_vvt_control_links_control_id ON ck_vvt_control_links (control_id);
CREATE INDEX idx_ck_vvt_control_links_vvt_id ON ck_vvt_control_links (org_id, vvt_id);
