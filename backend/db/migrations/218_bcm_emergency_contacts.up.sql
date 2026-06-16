-- S86-1: BSI-200-4 Alarmierungsplan
CREATE TABLE ck_emergency_contacts (
    id               UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id           UUID NOT NULL,
    name             TEXT NOT NULL,
    role             TEXT NOT NULL DEFAULT '',
    phone            TEXT NOT NULL DEFAULT '',
    email            TEXT NOT NULL DEFAULT '',
    escalation_level INT  NOT NULL DEFAULT 1
                         CHECK (escalation_level IN (1,2,3)),
    available_24_7   BOOL NOT NULL DEFAULT FALSE,
    notes            TEXT NOT NULL DEFAULT '',
    created_at       TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at       TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX idx_ck_emergency_contacts_org_id ON ck_emergency_contacts (org_id);
