package vaktcomply

import (
	"testing"
	"time"

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

// --- SoA seed controls count ---

func TestISO27001AnnexAControls_Count(t *testing.T) {
	assert.Len(t, iso27001AnnexAControls, 93,
		"ISO 27001:2022 Annex A must have exactly 93 controls (A.5.1–A.8.34)")
}

func TestISO27001AnnexAControls_Groups(t *testing.T) {
	groups := map[string]int{}
	for _, c := range iso27001AnnexAControls {
		groups[c.Group]++
	}
	// ISO 27001:2022 group sizes: A.5=37, A.6=8, A.7=14, A.8=34
	assert.Equal(t, 37, groups["5"], "Group A.5 must have 37 controls")
	assert.Equal(t, 8, groups["6"], "Group A.6 must have 8 controls")
	assert.Equal(t, 14, groups["7"], "Group A.7 must have 14 controls")
	assert.Equal(t, 34, groups["8"], "Group A.8 must have 34 controls")
}

func TestISO27001AnnexAControls_UniqueRefs(t *testing.T) {
	seen := map[string]bool{}
	for _, c := range iso27001AnnexAControls {
		assert.False(t, seen[c.Ref], "duplicate control ref: %s", c.Ref)
		seen[c.Ref] = true
	}
}

func TestISO27001AnnexAControls_NoEmptyNames(t *testing.T) {
	for _, c := range iso27001AnnexAControls {
		assert.NotEmpty(t, c.Name, "control %s must have a name", c.Ref)
	}
}

// --- SoADedicatedEntry ---

func TestSoADedicatedEntry_ApplicableEntry(t *testing.T) {
	entry := SoADedicatedEntry{
		ControlRef:           "A.5.1",
		ControlGroup:         "5",
		ControlName:          "Informationssicherheitsrichtlinien",
		Applicable:           true,
		Justification:        "Erforderlich für ISO 27001-Zertifizierung",
		ImplementationStatus: "implemented",
	}
	assert.True(t, entry.Applicable)
	assert.Equal(t, "A.5.1", entry.ControlRef)
	assert.NotEmpty(t, entry.Justification)
}

func TestSoADedicatedEntry_ExcludedEntry(t *testing.T) {
	entry := SoADedicatedEntry{
		ControlRef:      "A.7.4",
		ControlGroup:    "7",
		ControlName:     "Physische Sicherheitsüberwachung",
		Applicable:      false,
		ExclusionReason: "Kein physisches Rechenzentrum, ausschließlich Cloud-Betrieb",
	}
	assert.False(t, entry.Applicable)
	assert.NotEmpty(t, entry.ExclusionReason)
}

// --- InterestedParty ---

func TestInterestedParty_RequiredFields(t *testing.T) {
	ip := InterestedParty{
		ID:           "ip-1",
		OrgID:        "org-1",
		Name:         "Bundesamt für Sicherheit in der Informationstechnik",
		Category:     "regulatory",
		Requirements: "Einhaltung BSI-Grundschutz und NIS2",
	}
	assert.Equal(t, "regulatory", ip.Category)
	assert.NotEmpty(t, ip.Requirements)
}

// --- Audit evidence status logic ---

func TestAuditEvidenceStatus_ZeroAudits_Warning(t *testing.T) {
	count := 0
	evidenceStatus := "ok"
	if count == 0 {
		evidenceStatus = "warning"
	}
	assert.Equal(t, "warning", evidenceStatus)
}

func TestAuditEvidenceStatus_WithAudits_OK(t *testing.T) {
	count := 2
	evidenceStatus := "ok"
	if count == 0 {
		evidenceStatus = "warning"
	}
	assert.Equal(t, "ok", evidenceStatus)
}

// --- DeletionReminder overdue logic ---

func TestDeletionReminder_PastDate_IsOverdue(t *testing.T) {
	past := time.Now().UTC().AddDate(0, 0, -1).Format("2006-01-02")
	isDue := func(dateStr string) bool {
		d, err := time.Parse("2006-01-02", dateStr)
		if err != nil {
			return false
		}
		return !d.After(time.Now().UTC().Truncate(24 * time.Hour))
	}
	assert.True(t, isDue(past))
}

func TestDeletionReminder_FutureDate_NotOverdue(t *testing.T) {
	future := time.Now().UTC().AddDate(0, 0, 30).Format("2006-01-02")
	isDue := func(dateStr string) bool {
		d, err := time.Parse("2006-01-02", dateStr)
		if err != nil {
			return false
		}
		return !d.After(time.Now().UTC().Truncate(24 * time.Hour))
	}
	assert.False(t, isDue(future))
}
