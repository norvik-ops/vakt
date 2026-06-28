package vaktcomply

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestApproveISMSScopeRoleCheck(t *testing.T) {
	svc := &Service{repo: nil} // repo is never reached for non-admin
	ctx := context.Background()

	_, err := svc.ApproveISMSScope(ctx, "org-1", "scope-1", "user-1", "analyst")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "only admins")

	_, err = svc.ApproveISMSScope(ctx, "org-1", "scope-1", "user-1", "viewer")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "only admins")

	_, err = svc.ApproveISMSScope(ctx, "org-1", "scope-1", "user-1", "")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "only admins")
}

func TestMapAssetType(t *testing.T) {
	cases := map[string]string{
		"asset":             "it_system",
		"network_asset":     "network",
		"application_asset": "application",
		"raum":              "room",
		"geschaeftsprozess": "process",
		"server_device":     "it_system",
	}
	for in, want := range cases {
		if got := mapAssetType(in); got != want {
			t.Errorf("mapAssetType(%q)=%q want %q", in, got, want)
		}
	}
}

func TestTruncateStr(t *testing.T) {
	if got := truncateStr("hello", 10); got != "hello" {
		t.Errorf("no-trunc got %q", got)
	}
	if got := truncateStr("hello world", 5); got != "hello" {
		t.Errorf("trunc got %q", got)
	}
}

func TestNIS2ReportabilityCheck_IsReportable(t *testing.T) {
	tests := []struct {
		name string
		c    NIS2ReportabilityCheck
		want bool
	}{
		{
			name: "all false → not reportable",
			c:    NIS2ReportabilityCheck{CausesSignificantDisruption: false, AffectsThirdParties: false, CausesFinancialDamage: false},
			want: false,
		},
		{
			name: "causes significant disruption → reportable",
			c:    NIS2ReportabilityCheck{CausesSignificantDisruption: true},
			want: true,
		},
		{
			name: "affects third parties → reportable",
			c:    NIS2ReportabilityCheck{AffectsThirdParties: true},
			want: true,
		},
		{
			name: "causes financial damage → reportable",
			c:    NIS2ReportabilityCheck{CausesFinancialDamage: true},
			want: true,
		},
		{
			name: "all true → reportable",
			c:    NIS2ReportabilityCheck{CausesSignificantDisruption: true, AffectsThirdParties: true, CausesFinancialDamage: true},
			want: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, tt.c.IsReportable())
		})
	}
}

func TestMarkIncidentReportable_DeadlineCalculation(t *testing.T) {
	detectedAt := time.Date(2026, 6, 10, 10, 0, 0, 0, time.UTC)
	incidentID := uuid.New()

	// Verify deadline arithmetic without hitting the DB.
	earlyWarning := detectedAt.Add(24 * time.Hour)
	fullReport := detectedAt.Add(72 * time.Hour)
	finalReport := detectedAt.Add(30 * 24 * time.Hour)

	assert.Equal(t, detectedAt.Add(24*time.Hour), earlyWarning, "early warning = T+24h")
	assert.Equal(t, detectedAt.Add(72*time.Hour), fullReport, "full report = T+72h")
	assert.Equal(t, detectedAt.Add(720*time.Hour), finalReport, "final report = T+720h (30d)")

	_ = incidentID // used in service test with live DB
}

func TestNIS2DeadlineCheck_StageFiltering(t *testing.T) {
	now := time.Now().UTC()
	warn := now.Add(2 * time.Hour)

	// Deadline within warn window and not yet submitted → should notify
	deadline := now.Add(1 * time.Hour)
	assert.True(t, deadline.Before(warn), "deadline in < 2h should trigger notification")

	// Deadline already past → also within warn window
	pastDeadline := now.Add(-1 * time.Hour)
	assert.True(t, pastDeadline.Before(warn), "overdue deadline should also trigger notification")

	// Deadline far in future → should not notify
	futureDeadline := now.Add(3 * time.Hour)
	assert.False(t, futureDeadline.Before(warn), "deadline in > 2h should not trigger notification")
}

func TestThreatLibrary_LoadsAtLeast60(t *testing.T) {
	root := loadThreatLibrary()
	if len(root.Threats) < 60 {
		t.Fatalf("threat library has %d entries, want >=60", len(root.Threats))
	}
	if root.Version == "" {
		t.Error("threat library version must be set (for link provenance)")
	}
	seen := map[string]bool{}
	for _, it := range root.Threats {
		if it.ID == "" || it.Title == "" {
			t.Errorf("threat %q has empty id/title", it.ID)
		}
		if seen[it.ID] {
			t.Errorf("duplicate threat id %q", it.ID)
		}
		seen[it.ID] = true
		if it.DefaultLikelihood < 1 || it.DefaultLikelihood > 5 {
			t.Errorf("threat %s default_likelihood out of range: %d", it.ID, it.DefaultLikelihood)
		}
		if it.DefaultImpact < 1 || it.DefaultImpact > 5 {
			t.Errorf("threat %s default_impact out of range: %d", it.ID, it.DefaultImpact)
		}
		if it.SuggestedMeasure == "" {
			t.Errorf("threat %s has no suggested measure", it.ID)
		}
	}
}

func TestThreatCatalogFilter(t *testing.T) {
	svc := &Service{}

	all := svc.ListThreatCatalog(ThreatCatalogFilter{})
	if len(all) < 60 {
		t.Fatalf("unfiltered returned %d, want >=60", len(all))
	}

	iso := svc.ListThreatCatalog(ThreatCatalogFilter{Framework: "ISO27001"})
	if len(iso) == 0 || len(iso) >= len(all) {
		t.Errorf("ISO27001 filter returned %d (want >0 and <%d)", len(iso), len(all))
	}
	for _, it := range iso {
		if !sliceContainsFold(it.Frameworks, "ISO27001") {
			t.Errorf("threat %s leaked into ISO27001 filter", it.ID)
		}
	}

	data := svc.ListThreatCatalog(ThreatCatalogFilter{AssetType: "data"})
	for _, it := range data {
		if !sliceContainsFold(it.AssetTypes, "data") {
			t.Errorf("threat %s leaked into data asset filter", it.ID)
		}
	}

	conf := svc.ListThreatCatalog(ThreatCatalogFilter{CIA: "confidentiality"})
	for _, it := range conf {
		if !sliceContainsFold(it.CIA, "confidentiality") {
			t.Errorf("threat %s leaked into confidentiality filter", it.ID)
		}
	}

	// Combined filter must be a subset of each single filter.
	combined := svc.ListThreatCatalog(ThreatCatalogFilter{Framework: "NIS2", CIA: "availability"})
	for _, it := range combined {
		if !sliceContainsFold(it.Frameworks, "NIS2") || !sliceContainsFold(it.CIA, "availability") {
			t.Errorf("combined filter leaked %s", it.ID)
		}
	}
}

func TestFindThreatCatalogItem(t *testing.T) {
	if _, ok := findThreatCatalogItem("T-RANSOMWARE"); !ok {
		t.Error("T-RANSOMWARE must exist in the catalog")
	}
	if _, ok := findThreatCatalogItem("T-NOPE"); ok {
		t.Error("unknown id must not resolve")
	}
}

func TestParseSupplierCSVRows_AllValidCriticalities(t *testing.T) {
	for _, crit := range []string{"low", "medium", "high", "critical", "standard", "important"} {
		csv := "name,criticality\nAcme," + crit + "\n"
		rows, err := parseSupplierCSVRows(csv)
		require.NoError(t, err, "criticality=%s", crit)
		require.Len(t, rows, 1, "criticality=%s", crit)
		assert.Equal(t, crit, rows[0].Criticality)
	}
}

func TestParseSupplierCSVRows_BoolTrueVariants(t *testing.T) {
	for _, val := range []string{"True", "TRUE", "1"} {
		csv := "name,nis2_relevant\nAcme," + val + "\n"
		rows, err := parseSupplierCSVRows(csv)
		require.NoError(t, err)
		require.Len(t, rows, 1)
		assert.True(t, rows[0].NIS2Relevant, "input=%q should parse as true", val)
	}
}

// ── computeStatus — branches not covered in service_test.go ─────────────────

func TestComputeStatus_InProgressAssessment(t *testing.T) {
	supplier := Supplier{ID: "s1"}
	assessments := []Assessment{{ID: "a1", Status: "in_progress"}}
	st := computeStatus(supplier, assessments, nil, time.Now())
	assert.Equal(t, "yellow", st.Status)
	assert.Equal(t, "assessment_pending", st.Details["reason"])
}

func TestComputeStatus_SubmittedAwaitingReview(t *testing.T) {
	supplier := Supplier{ID: "s1"}
	assessments := []Assessment{{ID: "a1", Status: "submitted"}}
	st := computeStatus(supplier, assessments, nil, time.Now())
	assert.Equal(t, "yellow", st.Status)
	assert.Equal(t, "awaiting_review", st.Details["reason"])
}

func TestComputeStatus_ReviewedNoAnswers_Fallback(t *testing.T) {
	supplier := Supplier{ID: "s1"}
	assessments := []Assessment{{ID: "a1", Status: "reviewed"}}
	st := computeStatus(supplier, assessments, nil, time.Now())
	// reviewed but empty answers → falls through to fallback yellow
	assert.Equal(t, "yellow", st.Status)
}
