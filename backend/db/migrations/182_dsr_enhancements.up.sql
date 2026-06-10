-- S68-2: DSR Management Enhancements (DSGVO Art. 15-22)
-- Extends po_dsr with channel, reference_id, extension tracking and overdue status.

ALTER TABLE po_dsr
    ADD COLUMN IF NOT EXISTS channel           TEXT CHECK (channel IN ('email', 'postal', 'form', 'verbal', 'other')),
    ADD COLUMN IF NOT EXISTS reference_id      TEXT,
    ADD COLUMN IF NOT EXISTS extension_due_at  TIMESTAMPTZ,
    ADD COLUMN IF NOT EXISTS extension_reason  TEXT,
    ADD COLUMN IF NOT EXISTS resolved_by       UUID REFERENCES users(id),
    ADD COLUMN IF NOT EXISTS assigned_to       UUID REFERENCES users(id);

-- Add 'extended' and 'overdue' to status check (recreate constraint)
ALTER TABLE po_dsr DROP CONSTRAINT IF EXISTS po_dsr_status_check;
ALTER TABLE po_dsr ADD CONSTRAINT po_dsr_status_check
    CHECK (status IN ('open', 'in_progress', 'completed', 'rejected', 'extended', 'overdue'));

-- Add 'restriction', 'no_profiling' request types
ALTER TABLE po_dsr DROP CONSTRAINT IF EXISTS po_dsr_type_check;
ALTER TABLE po_dsr ADD CONSTRAINT po_dsr_type_check
    CHECK (type IN ('access', 'erasure', 'portability', 'objection', 'rectification', 'restriction', 'no_profiling'));

-- Index for overdue detection
CREATE INDEX IF NOT EXISTS idx_po_dsr_due_open ON po_dsr (org_id, due_date)
    WHERE status NOT IN ('completed', 'rejected', 'extended');
