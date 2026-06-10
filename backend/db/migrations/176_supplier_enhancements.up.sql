-- S67-2: Lieferanten-Risikomanagement (A.5.19-21)
-- Extends ck_suppliers with ISO 27001-required fields.

ALTER TABLE ck_suppliers
    ADD COLUMN IF NOT EXISTS category              TEXT CHECK (category IN (
        'software', 'cloud', 'hardware', 'service', 'telecom', 'other'
    )) DEFAULT 'other',
    ADD COLUMN IF NOT EXISTS data_access           BOOLEAN    NOT NULL DEFAULT false,
    ADD COLUMN IF NOT EXISTS avv_document_id       UUID,
    ADD COLUMN IF NOT EXISTS last_assessment_score INTEGER    CHECK (last_assessment_score BETWEEN 1 AND 5),
    ADD COLUMN IF NOT EXISTS next_assessment_due   DATE,
    ADD COLUMN IF NOT EXISTS status                TEXT       NOT NULL DEFAULT 'active'
        CHECK (status IN ('active', 'inactive', 'terminated')),
    ADD COLUMN IF NOT EXISTS contract_start        DATE,
    ADD COLUMN IF NOT EXISTS data_protection_score INTEGER    CHECK (data_protection_score BETWEEN 1 AND 5),
    ADD COLUMN IF NOT EXISTS availability_score    INTEGER    CHECK (availability_score BETWEEN 1 AND 5),
    ADD COLUMN IF NOT EXISTS security_certifications TEXT,
    ADD COLUMN IF NOT EXISTS audit_rights          BOOLEAN,
    ADD COLUMN IF NOT EXISTS sub_processors_known  BOOLEAN,
    ADD COLUMN IF NOT EXISTS incident_notification BOOLEAN;

-- Index for overdue assessment queries
CREATE INDEX IF NOT EXISTS idx_ck_suppliers_assessment_due
    ON ck_suppliers (org_id, next_assessment_due)
    WHERE status = 'active';

-- Backfill criticality-based next_assessment_due for existing active suppliers
-- that already have a last_assessment_at but no next_assessment_due.
UPDATE ck_suppliers
SET next_assessment_due = (
    CASE criticality
        WHEN 'critical'  THEN (last_assessment_at::date + INTERVAL '365 days')::date
        WHEN 'important' THEN (last_assessment_at::date + INTERVAL '730 days')::date
        ELSE                  (last_assessment_at::date + INTERVAL '1095 days')::date
    END
)
WHERE last_assessment_at IS NOT NULL
  AND next_assessment_due IS NULL;
