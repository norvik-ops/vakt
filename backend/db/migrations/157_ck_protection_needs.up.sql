CREATE TABLE ck_protection_need_assessments (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id          UUID NOT NULL,
    name            TEXT NOT NULL,
    object_type     TEXT NOT NULL CHECK (object_type IN ('process','system','information','location')),
    object_name     TEXT NOT NULL,
    confidentiality TEXT NOT NULL DEFAULT 'normal' CHECK (confidentiality IN ('normal','hoch','sehr_hoch')),
    integrity       TEXT NOT NULL DEFAULT 'normal' CHECK (integrity IN ('normal','hoch','sehr_hoch')),
    availability    TEXT NOT NULL DEFAULT 'normal' CHECK (availability IN ('normal','hoch','sehr_hoch')),
    overall         TEXT NOT NULL DEFAULT 'normal' CHECK (overall IN ('normal','hoch','sehr_hoch')),
    status          TEXT NOT NULL DEFAULT 'draft' CHECK (status IN ('draft','finalized')),
    finalized_at    TIMESTAMPTZ,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_ck_protection_need_assessments_org_id ON ck_protection_need_assessments (org_id);
