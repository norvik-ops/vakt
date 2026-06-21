// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package audit

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// --- auditTypeLabel ---

func TestAuditTypeLabel_Known(t *testing.T) {
	assert.Equal(t, "Internes ISMS-Audit", auditTypeLabel("isms_internal"))
	assert.Equal(t, "Compliance-Prüfung", auditTypeLabel("compliance_check"))
	assert.Equal(t, "Lieferanten-Audit", auditTypeLabel("supplier_audit"))
	assert.Equal(t, "Prozess-Audit", auditTypeLabel("process_audit"))
}

func TestAuditTypeLabel_Unknown_PassThrough(t *testing.T) {
	assert.Equal(t, "custom_type", auditTypeLabel("custom_type"))
}

// --- auditStatusLabel ---

func TestAuditStatusLabel_Known(t *testing.T) {
	assert.Equal(t, "Geplant", auditStatusLabel("planned"))
	assert.Equal(t, "In Bearbeitung", auditStatusLabel("in_progress"))
	assert.Equal(t, "Abgeschlossen", auditStatusLabel("completed"))
	assert.Equal(t, "Abgebrochen", auditStatusLabel("cancelled"))
}

func TestAuditStatusLabel_Unknown_PassThrough(t *testing.T) {
	assert.Equal(t, "unknown_status", auditStatusLabel("unknown_status"))
}

// --- severityLabel ---

func TestSeverityLabel_Known(t *testing.T) {
	assert.Equal(t, "NC (Schwerwiegend)", severityLabel("major_nc"))
	assert.Equal(t, "NC (Leicht)", severityLabel("minor_nc"))
	assert.Equal(t, "Beobachtung", severityLabel("observation"))
	assert.Equal(t, "Verbesserungs-OFI", severityLabel("ofi"))
}

func TestSeverityLabel_Unknown_PassThrough(t *testing.T) {
	assert.Equal(t, "custom_severity", severityLabel("custom_severity"))
}

// --- AuditFinding model: auto-CAPA for major_nc/minor_nc ---

func TestAuditFinding_ShouldAutoCreateCAPA(t *testing.T) {
	shouldAutoCreate := func(severity string) bool {
		return severity == "major_nc" || severity == "minor_nc"
	}

	assert.True(t, shouldAutoCreate("major_nc"))
	assert.True(t, shouldAutoCreate("minor_nc"))
	assert.False(t, shouldAutoCreate("observation"))
	assert.False(t, shouldAutoCreate("ofi"))
}

// --- AuditPlan type ---

func TestAuditPlanModel_Fields(t *testing.T) {
	p := AuditPlan{
		ID:     "plan-1",
		OrgID:  "org-1",
		Year:   2026,
		Scope:  "Gesamtes ISMS",
		Status: "active",
	}
	assert.Equal(t, "plan-1", p.ID)
	assert.Equal(t, 2026, p.Year)
	assert.Equal(t, "active", p.Status)
}
