ALTER TABLE po_dsr
    DROP COLUMN IF EXISTS channel,
    DROP COLUMN IF EXISTS reference_id,
    DROP COLUMN IF EXISTS extension_due_at,
    DROP COLUMN IF EXISTS extension_reason,
    DROP COLUMN IF EXISTS resolved_by,
    DROP COLUMN IF EXISTS assigned_to;

ALTER TABLE po_dsr DROP CONSTRAINT IF EXISTS po_dsr_status_check;
ALTER TABLE po_dsr ADD CONSTRAINT po_dsr_status_check
    CHECK (status IN ('open', 'in_progress', 'completed', 'rejected'));

ALTER TABLE po_dsr DROP CONSTRAINT IF EXISTS po_dsr_type_check;
ALTER TABLE po_dsr ADD CONSTRAINT po_dsr_type_check
    CHECK (type IN ('access', 'erasure', 'portability', 'objection', 'rectification'));

DROP INDEX IF EXISTS idx_po_dsr_due_open;
