-- S70-3: DSGVO Art. 25 Privacy by Design & Default assessments
CREATE TABLE po_privacy_design_assessments (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    processing_activity_id UUID NOT NULL
        REFERENCES po_processing_activities(id) ON DELETE CASCADE,
    -- Art. 25 Abs. 1 — by Design
    design_measures TEXT NOT NULL DEFAULT '',
    design_at_conception BOOLEAN NOT NULL DEFAULT false,
    risk_considered BOOLEAN NOT NULL DEFAULT false,
    -- Art. 25 Abs. 2 — by Default
    data_minimization BOOLEAN NOT NULL DEFAULT false,
    purpose_limitation BOOLEAN NOT NULL DEFAULT false,
    storage_limitation BOOLEAN NOT NULL DEFAULT false,
    access_limitation BOOLEAN NOT NULL DEFAULT false,
    default_settings_note TEXT,
    -- Gesamtbewertung
    assessment_result TEXT NOT NULL
        CHECK (assessment_result IN ('compliant', 'partially', 'not_assessed'))
        DEFAULT 'not_assessed',
    reviewed_by UUID REFERENCES users(id),
    reviewed_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (org_id, processing_activity_id)
);
CREATE INDEX idx_po_privacy_design_org ON po_privacy_design_assessments (org_id, assessment_result);
