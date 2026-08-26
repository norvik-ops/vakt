// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

// Package reporting holds the reporting-and-monitoring domain of vaktcomply.
// It owns Continuous Control Monitoring (CCM), the KPI dashboard (S61-7),
// and NIS2 Art.23 stage reporting (S88). The domain is self-contained —
// it depends only on the generated db layer and the shared pgxpool, never
// on the parent vaktcomply package.
package reporting

import "time"

// CCMCheck represents an automated compliance control check definition.
type CCMCheck struct {
	ID            string            `json:"id"`
	OrgID         string            `json:"org_id"`
	ControlID     string            `json:"control_id"`
	Name          string            `json:"name"`
	CheckType     string            `json:"check_type"`
	Config        map[string]string `json:"config"`
	IntervalHours int               `json:"interval_hours"`
	LastRunAt     *time.Time        `json:"last_run_at,omitempty"`
	LastStatus    string            `json:"last_status,omitempty"`
	LastOutput    string            `json:"last_output,omitempty"`
	Enabled       bool              `json:"enabled"`
	CreatedAt     time.Time         `json:"created_at"`
	UpdatedAt     time.Time         `json:"updated_at"`
}

// CreateCCMCheckInput holds validated input for creating a CCM check.
type CreateCCMCheckInput struct {
	ControlID     string            `json:"control_id"     validate:"required"`
	Name          string            `json:"name"           validate:"required,max=255"`
	CheckType     string            `json:"check_type"     validate:"required,oneof=http_endpoint trivy_no_critical evidence_freshness custom_script"`
	Config        map[string]string `json:"config"`
	IntervalHours int               `json:"interval_hours" validate:"min=1,max=8760"`
}

// ToggleCCMCheckInput holds the enabled flag for toggling a CCM check.
type ToggleCCMCheckInput struct {
	Enabled bool `json:"enabled"`
}

// CCMResult represents the result of a single CCM check execution.
type CCMResult struct {
	ID      string    `json:"id"`
	CheckID string    `json:"check_id"`
	Status  string    `json:"status"`
	Output  string    `json:"output,omitempty"`
	RanAt   time.Time `json:"ran_at"`
}

// ── KPI Dashboard (S61-7) ─────────────────────────────────────────────────────

// KPISnapshot holds the 12 ISMS KPIs computed for a single organisation on a given date.
type KPISnapshot struct {
	ID                    string    `json:"id"`
	OrgID                 string    `json:"org_id"`
	SnapshotDate          string    `json:"snapshot_date"`
	ComplianceScore       *float64  `json:"kpi_compliance_score"`
	OpenCriticalControls  *int      `json:"kpi_open_critical_controls"`
	OpenHighRisks         *int      `json:"kpi_open_high_risks"`
	ResidualRiskAvg       *float64  `json:"kpi_residual_risk_avg"`
	OpenIncidents         *int      `json:"kpi_open_incidents"`
	IncidentMTTRDays      *float64  `json:"kpi_incident_mttr_days"`
	EvidenceCoverage      *float64  `json:"kpi_evidence_coverage"`
	ExpiringEvidenceCount *int      `json:"kpi_expiring_evidence_count"`
	FindingSLACompliance  *float64  `json:"kpi_finding_sla_compliance"`
	OpenMajorNCs          *int      `json:"kpi_open_major_ncs"`
	SuppliersOverduePct   *float64  `json:"kpi_suppliers_overdue_pct"`
	PhishingClickRate     *float64  `json:"kpi_phishing_click_rate"`
	CreatedAt             time.Time `json:"created_at"`
}

// KPIDashboard bundles the most recent KPI snapshot with 90-day history.
type KPIDashboard struct {
	Current *KPISnapshot  `json:"current"`
	History []KPISnapshot `json:"history"`
}

// ── NIS2 Art.23 Stage Reporting (S88) ────────────────────────────────────────

// NIS2IncidentRow is a minimal incident view used for NIS2 deadline checking.
type NIS2IncidentRow struct {
	ID    string
	OrgID string
	Title string
	// DiscoveredAt is when the org became aware of the incident. It is the legal
	// anchor for every NIS2 Art. 23(4) deadline — never time.Now(). Populated by
	// GetNIS2Incident; ListNIS2OpenIncidents leaves it zero, because the deadline
	// sweep reads the stored due dates, not the anchor.
	DiscoveredAt                time.Time
	Status                      string
	NIS2Reportable              *bool
	NIS2ReportingStage          *string
	NIS2DetectedAt              *time.Time
	NIS2EarlyWarningDue         *time.Time
	NIS2FullReportDue           *time.Time
	NIS2FinalReportDue          *time.Time
	NIS2EarlyWarningSubmittedAt *time.Time
	NIS2FullReportSubmittedAt   *time.Time
	NIS2FinalReportSubmittedAt  *time.Time
}

// NIS2ReportabilityCheck holds the three NIS2 Art.23 meldepflicht criteria.
type NIS2ReportabilityCheck struct {
	CausesSignificantDisruption bool `json:"causes_significant_disruption"`
	AffectsThirdParties         bool `json:"affects_third_parties"`
	CausesFinancialDamage       bool `json:"causes_financial_damage"`
}

// IsReportable returns true if any criterion is satisfied.
func (c NIS2ReportabilityCheck) IsReportable() bool {
	return c.CausesSignificantDisruption || c.AffectsThirdParties || c.CausesFinancialDamage
}

// NIS2ReportInput holds form data for a single reporting stage.
type NIS2ReportInput struct {
	AffectedServices      string  `json:"affected_services"`
	InitialAssessment     string  `json:"initial_assessment"`
	RootCause             string  `json:"root_cause"`
	AffectedUsersEstimate *int    `json:"affected_users_estimate,omitempty"`
	MeasuresTaken         string  `json:"measures_taken"`
	EstimatedRecovery     *string `json:"estimated_recovery,omitempty"`
	FullRootCauseAnalysis string  `json:"full_root_cause_analysis"`
	PermanentMeasures     string  `json:"permanent_measures"`
	EffectivenessEvidence string  `json:"effectiveness_evidence"`
}

// NIS2ReportStatus is returned by GET /incidents/{id}/nis2-status.
type NIS2ReportStatus struct {
	IsReportable    bool              `json:"is_reportable"`
	ReportingStage  string            `json:"reporting_stage"`
	DetectedAt      *time.Time        `json:"detected_at,omitempty"`
	Deadlines       NIS2Deadlines     `json:"deadlines"`
	CompletedStages []string          `json:"completed_stages"`
	Reports         []NIS2StageReport `json:"reports"`
}

// NIS2Deadlines holds all three deadline timestamps.
type NIS2Deadlines struct {
	EarlyWarning *time.Time `json:"early_warning,omitempty"`
	FullReport   *time.Time `json:"full_report,omitempty"`
	FinalReport  *time.Time `json:"final_report,omitempty"`
}

// NIS2StageReport is a summary of a submitted report stage.
type NIS2StageReport struct {
	ID          string     `json:"id"`
	Stage       string     `json:"stage"`
	SubmittedAt *time.Time `json:"submitted_at,omitempty"`
	PDFPath     string     `json:"pdf_path,omitempty"`
}

// AuthorityContact represents an entry in the DACH authority directory.
type AuthorityContact struct {
	ID    string  `json:"id"`
	OrgID *string `json:"org_id,omitempty"`
	// SA14-04: country is CHECK-constrained to ('de','at','ch','eu') in the DB
	// (migration 175). Validate it at the handler so a bad value is a 422 before
	// the INSERT, not a 500 from the check_violation.
	Country       string `json:"country"        validate:"required,oneof=de at ch eu"`
	Sector        string `json:"sector,omitempty"`
	AuthorityName string `json:"authority_name" validate:"required"`
	ReportURL     string `json:"report_url,omitempty" validate:"omitempty,url"`
	Email         string `json:"email,omitempty"      validate:"omitempty,email"`
	Phone         string `json:"phone,omitempty"`
	Notes         string `json:"notes,omitempty"`
	IsPrimary     bool   `json:"is_primary"`
	IsBuiltin     bool   `json:"is_builtin"`
	CreatedAt     string `json:"created_at"`
}
