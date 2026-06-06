package retention

import (
	"strings"
	"testing"
)

// TestAuditLogRetentionUsesSoftDelete guards against regression to hard-delete.
// A DELETE FROM audit_log would permanently break the SHA-256 hash chain
// (ADR-0040/ADR-0050), making every subsequent row appear tampered to
// cmd/audit-verify. This test needs no DB — it validates the SQL text directly.
func TestAuditLogRetentionUsesSoftDelete(t *testing.T) {
	q := strings.ToUpper(sqlAuditLogSoftDelete)

	if strings.Contains(q, "DELETE FROM") {
		t.Fatal("audit_log retention must not use DELETE — hash chain would break; use UPDATE SET deleted_at instead")
	}
	if !strings.Contains(q, "UPDATE AUDIT_LOG") {
		t.Errorf("expected UPDATE AUDIT_LOG, got:\n%s", sqlAuditLogSoftDelete)
	}
	if !strings.Contains(q, "SET    DELETED_AT") && !strings.Contains(q, "SET DELETED_AT") {
		t.Errorf("expected SET deleted_at in query, got:\n%s", sqlAuditLogSoftDelete)
	}
}

// TestAuditLogSoftDeleteIsIdempotent verifies the WHERE clause guards against
// double-soft-deleting rows (which would update updated_at unnecessarily and
// interfere with monitoring).
func TestAuditLogSoftDeleteIsIdempotent(t *testing.T) {
	q := strings.ToUpper(sqlAuditLogSoftDelete)
	if !strings.Contains(q, "DELETED_AT IS NULL") {
		t.Errorf("soft-delete query must be idempotent (AND deleted_at IS NULL), got:\n%s", sqlAuditLogSoftDelete)
	}
}

// TestAuditLogSoftDeleteScopedToOrg verifies org isolation — a retention run
// for one org must never touch another org's rows.
func TestAuditLogSoftDeleteScopedToOrg(t *testing.T) {
	q := strings.ToUpper(sqlAuditLogSoftDelete)
	if !strings.Contains(q, "ORG_ID") {
		t.Errorf("soft-delete query must be scoped to org_id, got:\n%s", sqlAuditLogSoftDelete)
	}
}

// TestAuditLogSoftDeleteUsesCreatedAtCutoff verifies the time-based filter is
// present — without it the query would soft-delete the entire org's audit log.
func TestAuditLogSoftDeleteUsesCreatedAtCutoff(t *testing.T) {
	q := strings.ToUpper(sqlAuditLogSoftDelete)
	if !strings.Contains(q, "CREATED_AT") {
		t.Errorf("soft-delete query must filter by created_at (retention window), got:\n%s", sqlAuditLogSoftDelete)
	}
}
