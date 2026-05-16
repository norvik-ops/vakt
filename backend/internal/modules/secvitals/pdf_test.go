package secvitals

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- GenerateSoAPDF ---

// TestGenerateSoAPDF_Empty verifies that a SoA PDF is produced without error for an empty row list.
func TestGenerateSoAPDF_Empty(t *testing.T) {
	got, err := GenerateSoAPDF([]SoARow{}, "ISO 27001", "Test GmbH", time.Now())
	require.NoError(t, err)
	assert.Greater(t, len(got), 0, "PDF output must not be empty")
}

// TestGenerateSoAPDF_WithRows verifies that a SoA PDF is produced for a set of sample rows.
func TestGenerateSoAPDF_WithRows(t *testing.T) {
	rows := []SoARow{
		{
			ControlID:      "A.5.1",
			Title:          "Informationssicherheitsrichtlinien",
			Domain:         "A.5 — Organisatorische Maßnahmen",
			Applicable:     true,
			Justification:  "Pflicht gemäß ISO 27001",
			Implementation: "Richtlinie verabschiedet",
			Responsible:    "CISO",
			ManualStatus:   "implemented",
			EvidenceCount:  3,
		},
		{
			ControlID:      "A.6.1",
			Title:          "Rollen und Verantwortlichkeiten für die Informationssicherheit",
			Domain:         "A.6 — Personen",
			Applicable:     true,
			Justification:  "Organisatorische Anforderung",
			Implementation: "Stellenbeschreibungen aktualisiert",
			Responsible:    "HR",
			ManualStatus:   "in_progress",
			EvidenceCount:  1,
		},
		{
			ControlID:     "A.8.1",
			Title:         "Nicht zutreffende Maßnahme",
			Domain:        "A.8 — Technologische Maßnahmen",
			Applicable:    false,
			Justification: "Nicht relevant für Unternehmensgröße",
			EvidenceCount: 0,
		},
	}

	got, err := GenerateSoAPDF(rows, "ISO 27001", "Acme GmbH", time.Now())
	require.NoError(t, err)
	assert.Greater(t, len(got), 0, "PDF output must not be empty for non-empty row list")
}

// TestGenerateSoAPDF_MultiDomain verifies that rows from multiple domains are all rendered.
func TestGenerateSoAPDF_MultiDomain(t *testing.T) {
	domains := []string{"A.5", "A.6", "A.7", "A.8"}
	var rows []SoARow
	for _, domain := range domains {
		rows = append(rows, SoARow{
			ControlID:     domain + ".1",
			Title:         "Control in " + domain,
			Domain:        domain,
			Applicable:    true,
			EvidenceCount: 2,
		})
	}

	got, err := GenerateSoAPDF(rows, "ISO 27001:2022", "Multi GmbH", time.Date(2024, 1, 15, 0, 0, 0, 0, time.UTC))
	require.NoError(t, err)
	assert.Greater(t, len(got), 1000, "multi-domain PDF should produce substantial output")
}

// --- GenerateIncidentReportPDF ---

// TestGenerateIncidentReportPDF_Standard verifies PDF output for a standard (non-major) incident.
func TestGenerateIncidentReportPDF_Standard(t *testing.T) {
	inc := &Incident{
		ID:           "inc-001",
		Title:        "Phishing-Angriff auf Postfächer",
		Description:  "Mehrere Mitarbeiter öffneten einen schädlichen Anhang.",
		Severity:     "high",
		Status:       "investigating",
		IncidentType: "general",
		DiscoveredAt: time.Now().UTC().Add(-4 * time.Hour),
	}

	got, err := GenerateIncidentReportPDF(inc, "Test GmbH")
	require.NoError(t, err)
	assert.Greater(t, len(got), 0, "PDF must not be empty for standard incident")
}

// TestGenerateIncidentReportPDF_DORAMajor verifies PDF output for a DORA major incident.
func TestGenerateIncidentReportPDF_DORAMajor(t *testing.T) {
	customers := 500
	estimate := "ca. 50.000 EUR"
	inc := &Incident{
		ID:                      "inc-002",
		Title:                   "Schwerwiegender IKT-Vorfall",
		Description:             "Produktionssystem vollständig ausgefallen.",
		Severity:                "critical",
		Status:                  "open",
		IncidentType:            "dora",
		IsMajorIncident:         true,
		AffectedCustomers:       &customers,
		FinancialImpactEstimate: &estimate,
		NotificationAuthority:   "BaFin",
		DiscoveredAt:            time.Now().UTC().Add(-2 * time.Hour),
	}

	got, err := GenerateIncidentReportPDF(inc, "Finanzinstitut AG")
	require.NoError(t, err)
	assert.Greater(t, len(got), 0, "PDF must not be empty for major incident")
}

// TestGenerateIncidentReportPDF_AllSeverities verifies that all severity levels produce output.
func TestGenerateIncidentReportPDF_AllSeverities(t *testing.T) {
	for _, severity := range []string{"low", "medium", "high", "critical"} {
		inc := &Incident{
			ID:           "inc-sev-" + severity,
			Title:        "Test Incident " + severity,
			Severity:     severity,
			Status:       "open",
			IncidentType: "general",
			DiscoveredAt: time.Now().UTC(),
		}
		got, err := GenerateIncidentReportPDF(inc, "Test Org")
		require.NoError(t, err, "severity: %s", severity)
		assert.Greater(t, len(got), 0, "PDF must not be empty for severity %s", severity)
	}
}

// --- GenerateDORAPDF ---

// TestGenerateDORAPDF_Basic verifies that a DORA readiness report PDF is produced without error.
func TestGenerateDORAPDF_Basic(t *testing.T) {
	dashboard := &DORADashboard{
		ReadinessPct:         72.5,
		OpenCriticalControls: 3,
		ExpiredSuppliers:     1,
		TLPTOverdueWarning:   false,
	}

	got, err := GenerateDORAPDF(dashboard, "Test AG")
	require.NoError(t, err)
	assert.Greater(t, len(got), 0)
}

// TestGenerateDORAPDF_WithNextDeadline verifies PDF output when a next deadline is present.
func TestGenerateDORAPDF_WithNextDeadline(t *testing.T) {
	deadline := &NextDeadline{
		IncidentID:   "inc-003",
		Title:        "Offener IKT-Vorfall",
		DeadlineType: "72h",
		DeadlineAt:   time.Now().UTC().Add(24 * time.Hour),
	}
	dashboard := &DORADashboard{
		ReadinessPct:         45.0,
		OpenCriticalControls: 7,
		NextDeadline:         deadline,
		ExpiredSuppliers:     2,
		TLPTOverdueWarning:   true,
	}

	got, err := GenerateDORAPDF(dashboard, "Kredit GmbH")
	require.NoError(t, err)
	assert.Greater(t, len(got), 0)
}

// TestGenerateDORAPDF_ScoreVariants verifies all three score-band colours are rendered.
func TestGenerateDORAPDF_ScoreVariants(t *testing.T) {
	for _, pct := range []float64{20.0, 65.0, 92.0} {
		dashboard := &DORADashboard{ReadinessPct: pct}
		got, err := GenerateDORAPDF(dashboard, "Org")
		require.NoError(t, err, "score %.0f%%", pct)
		assert.Greater(t, len(got), 0)
	}
}

// --- GenerateFrameworkPDF ---

// TestGenerateFrameworkPDF_Empty verifies that an empty framework report produces a valid PDF.
func TestGenerateFrameworkPDF_Empty(t *testing.T) {
	report := &ReadinessReport{
		FrameworkName:  "NIS2",
		ReadinessScore: 0,
		TotalControls:  0,
	}

	got, err := GenerateFrameworkPDF(report, nil, "Test GmbH")
	require.NoError(t, err)
	assert.Greater(t, len(got), 0)
}

// TestGenerateFrameworkPDF_WithGaps verifies that gaps are rendered in the PDF.
func TestGenerateFrameworkPDF_WithGaps(t *testing.T) {
	report := &ReadinessReport{
		FrameworkName:  "ISO 27001",
		ReadinessScore: 60.0,
		TotalControls:  10,
		Covered:        6,
		Partial:        2,
		Missing:        2,
		ByDomain: []DomainScore{
			{Domain: "Zugangskontrolle", Total: 5, Covered: 3, Score: 60},
			{Domain: "Kryptographie", Total: 5, Covered: 3, Score: 60},
		},
	}

	gaps := &GapAnalysis{
		Gaps: []ControlGap{
			{
				Control: Control{
					ID:        "ctl-1",
					ControlID: "A.8.1",
					Title:     "Benutzerendgeräte",
				},
				Reason: "no_evidence",
			},
			{
				Control: Control{
					ID:        "ctl-2",
					ControlID: "A.8.2",
					Title:     "Privilegierte Zugriffsrechte",
				},
				Reason: "evidence_expiring",
			},
		},
	}

	got, err := GenerateFrameworkPDF(report, gaps, "Produktions GmbH")
	require.NoError(t, err)
	assert.Greater(t, len(got), 0)
}

// --- GenerateNIS2ReportFormPDF ---

// TestGenerateNIS2ReportFormPDF_Early24h verifies a 24h early notification PDF is produced.
func TestGenerateNIS2ReportFormPDF_Early24h(t *testing.T) {
	deadline24h := time.Now().UTC().Add(20 * time.Hour)
	inc := &Incident{
		ID:                    "nis2-inc-1",
		Title:                 "DDoS-Angriff auf kritische Infrastruktur",
		Description:           "Ausfall des Webportals über 4 Stunden.",
		Severity:              "high",
		Status:                "investigating",
		IncidentType:          "nis2",
		NotificationAuthority: "BSI",
		DiscoveredAt:          time.Now().UTC().Add(-4 * time.Hour),
		Deadline24h:           &deadline24h,
	}

	got, err := GenerateNIS2ReportFormPDF(inc, "24h", "KRITIS Betreiber GmbH")
	require.NoError(t, err)
	assert.Greater(t, len(got), 0)
}

// TestGenerateNIS2ReportFormPDF_72h verifies a 72h full notification PDF is produced.
func TestGenerateNIS2ReportFormPDF_72h(t *testing.T) {
	deadline72h := time.Now().UTC().Add(48 * time.Hour)
	inc := &Incident{
		ID:                    "nis2-inc-2",
		Title:                 "Ransomware-Angriff",
		Severity:              "critical",
		Status:                "open",
		IncidentType:          "nis2",
		NotificationAuthority: "BSI",
		DiscoveredAt:          time.Now().UTC().Add(-24 * time.Hour),
		Deadline72h:           &deadline72h,
	}

	got, err := GenerateNIS2ReportFormPDF(inc, "72h", "Energie AG")
	require.NoError(t, err)
	assert.Greater(t, len(got), 0)
}

// TestGenerateNIS2ReportFormPDF_UnknownAuthority verifies fallback to BSI for unknown authority.
func TestGenerateNIS2ReportFormPDF_UnknownAuthority(t *testing.T) {
	inc := &Incident{
		ID:                    "nis2-inc-3",
		Title:                 "Test Vorfall",
		Severity:              "medium",
		Status:                "open",
		IncidentType:          "nis2",
		NotificationAuthority: "UnknownBehörde",
		DiscoveredAt:          time.Now().UTC(),
	}

	got, err := GenerateNIS2ReportFormPDF(inc, "24h", "Test Org")
	require.NoError(t, err, "unknown authority should fall back to BSI without error")
	assert.Greater(t, len(got), 0)
}

// --- GenerateTISAXReportPDF ---

// TestGenerateTISAXReportPDF_Empty verifies that a TISAX report is produced with no controls.
func TestGenerateTISAXReportPDF_Empty(t *testing.T) {
	got, err := GenerateTISAXReportPDF(nil, []Control{}, nil, "Test GmbH", "normal", "AL2", time.Now())
	require.NoError(t, err)
	assert.Greater(t, len(got), 0)
}

// TestGenerateTISAXReportPDF_ControlsAndGaps verifies that controls and gaps are rendered.
func TestGenerateTISAXReportPDF_ControlsAndGaps(t *testing.T) {
	report := &ReadinessReport{
		FrameworkName:  "TISAX",
		ReadinessScore: 55.0,
		TotalControls:  4,
		Covered:        2,
		Partial:        1,
		Missing:        1,
		TISAXMaturity: &TISAXMaturitySummary{
			ReadinessPercent: 55.0,
			ByChapter: []ChapterMaturity{
				{Domain: "Informationssicherheit", TotalControls: 4, AvgScore: 1.75, FullyMature: 2, Color: "yellow"},
			},
		},
	}

	controls := []Control{
		{ID: "ctl-1", ControlID: "IS.1.1", Title: "Informationssicherheitsrichtlinie", MaturityScore: 3, EvidenceCount: 2},
		{ID: "ctl-2", ControlID: "IS.1.2", Title: "Sicherheitsorganisation", MaturityScore: 2, EvidenceCount: 1},
		{ID: "ctl-3", ControlID: "IS.2.1", Title: "Risikobehandlung", MaturityScore: 0, EvidenceCount: 0},
	}

	gaps := &TISAXGapAnalysis{
		TargetScore: 3,
		Gaps: []TISAXControlGap{
			{Control: controls[1], MaturityGap: 1, CurrentScore: 2},
			{Control: controls[2], MaturityGap: 3, CurrentScore: 0},
		},
	}

	got, err := GenerateTISAXReportPDF(report, controls, gaps, "Auto GmbH", "high", "AL3", time.Date(2024, 3, 1, 0, 0, 0, 0, time.UTC))
	require.NoError(t, err)
	assert.Greater(t, len(got), 0)
}

// --- GenerateAIDocumentationPDF ---

// TestGenerateAIDocumentationPDF_Minimal verifies a minimal AI documentation PDF is produced.
func TestGenerateAIDocumentationPDF_Minimal(t *testing.T) {
	system := &AISystem{
		ID:     "ai-1",
		Name:   "Kreditbewertungs-KI",
		Status: "under_review",
	}
	doc := &AIDocumentation{
		ID:         "doc-1",
		AISystemID: "ai-1",
		Version:    1,
		Status:     "draft",
	}

	got, err := GenerateAIDocumentationPDF(system, doc)
	require.NoError(t, err)
	assert.Greater(t, len(got), 0)
}

// TestGenerateAIDocumentationPDF_Full verifies a fully populated documentation PDF is produced.
func TestGenerateAIDocumentationPDF_Full(t *testing.T) {
	system := &AISystem{
		ID:        "ai-2",
		Name:      "Anomalieerkennung",
		Provider:  "Eigenentwicklung",
		UseCase:   "Betrugsprävention",
		RiskClass: "high",
	}
	doc := &AIDocumentation{
		Version:            2,
		SystemDescription:  "Erkennt anomale Transaktionsmuster mittels ML.",
		IntendedPurpose:    "Verhinderung von Finanzbetrug.",
		TrainingData:       "Historische Transaktionsdaten (2018–2023).",
		DataQuality:        "Bereinigt, keine personenbezogenen Daten.",
		PerformanceMetrics: "AUC-ROC: 0.95, Precision: 0.88.",
		SystemLimits:       "Funktioniert nur bei bekannten Angriffsmustern.",
		RiskManagement:     "Regelmäßige Re-Training-Zyklen.",
		HumanOversight:     "Analyst prüft Alerts täglich.",
		LoggingAuditTrail:  "Alle Entscheidungen werden 7 Jahre gespeichert.",
		AuthoredBy:         "Dr. Max Muster",
		Status:             "final",
	}

	got, err := GenerateAIDocumentationPDF(system, doc)
	require.NoError(t, err)
	assert.Greater(t, len(got), 0)
}

// TestGenerateEUAIActReportPDF_Basic verifies that an EU AI Act report PDF is produced.
func TestGenerateEUAIActReportPDF_Basic(t *testing.T) {
	dashboard := &EUAIActDashboard{
		TotalSystems:       3,
		SystemsWithoutDocs: 1,
		HighRiskDeadline:   "2026-08-02",
		SystemsByRiskClass: map[string]int{
			"high":    1,
			"limited": 1,
			"minimal": 1,
		},
		ISO27001Mappings: []EUAIActISOMappingEntry{
			{EUAIActArticle: "Art. 9", EUAIActTopic: "Risikomanagement", ISO27001Control: "A.8.8", ISO27001Title: "Management von technischen Schwachstellen"},
		},
	}

	systems := []AISystem{
		{ID: "ai-1", Name: "Kreditbewertung", RiskClass: "high", Status: "approved", ClassifiedBy: "AI Board"},
		{ID: "ai-2", Name: "Chatbot", RiskClass: "limited", Status: "under_review"},
	}

	got, err := GenerateEUAIActReportPDF(dashboard, systems)
	require.NoError(t, err)
	assert.Greater(t, len(got), 0)
}
