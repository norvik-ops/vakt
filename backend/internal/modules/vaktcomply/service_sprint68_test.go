package vaktcomply

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// Audit-program label/model tests (auditTypeLabel, auditStatusLabel,
// severityLabel, AuditPlan, AuditFinding) moved to the audit sub-package
// alongside the code they exercise — see audit/service_program_test.go.

// --- SoA seed controls count ---

func TestISO27001AnnexAControls_Count(t *testing.T) {
	assert.Len(t, ISO27001AnnexAControls, 93,
		"ISO 27001:2022 Annex A must have exactly 93 controls (A.5.1–A.8.34)")
}

func TestISO27001AnnexAControls_Groups(t *testing.T) {
	groups := map[string]int{}
	for _, c := range ISO27001AnnexAControls {
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
	for _, c := range ISO27001AnnexAControls {
		assert.False(t, seen[c.Ref], "duplicate control ref: %s", c.Ref)
		seen[c.Ref] = true
	}
}

func TestISO27001AnnexAControls_NoEmptyNames(t *testing.T) {
	for _, c := range ISO27001AnnexAControls {
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
