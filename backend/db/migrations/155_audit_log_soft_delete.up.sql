-- 154_audit_log_soft_delete.up.sql
--
-- Replace hard-DELETE retention of audit_log rows with soft-delete so that
-- the SHA-256 hash chain (added in migration 149) remains verifiable after
-- retention runs.
--
-- Hard-deleting a row removes it from the chain, making every subsequent
-- row in the same org appear tampered (prev_hash mismatch). Soft-delete
-- marks the row as logically removed while leaving it in-place for the
-- chain verifier (cmd/audit-verify).
--
-- UI-facing read paths (List, admin panel, SIEM export, dashboard, etc.)
-- filter on deleted_at IS NULL so soft-deleted rows are invisible to users.
-- The chain verifier and the writer's SELECT-for-UPDATE tail query do NOT
-- filter on deleted_at so that the chain remains continuous.
--
-- See FINDING DATA-002.

ALTER TABLE audit_log ADD COLUMN IF NOT EXISTS deleted_at TIMESTAMPTZ;

-- Partial index: only indexes the small fraction of rows that have been
-- soft-deleted, making the retention-sweep UPDATE fast.
-- WHERE deleted_at IS NOT NULL uses an IS NOT NULL literal — IMMUTABLE
-- per CLAUDE.md migration rules (no STABLE/VOLATILE functions in predicates).
CREATE INDEX IF NOT EXISTS idx_audit_log_deleted_at
    ON audit_log (org_id, deleted_at)
    WHERE deleted_at IS NOT NULL;
