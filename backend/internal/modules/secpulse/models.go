// Package secpulse provides domain models for vulnerability management and asset registry.
package secpulse

import "time"

// Asset represents an infrastructure asset tracked in VulnBoard.
type Asset struct {
	ID          string    `json:"id"`
	OrgID       string    `json:"org_id"`
	Name        string    `json:"name"`
	Type        string    `json:"type"`
	Criticality string    `json:"criticality"`
	Tags        []string  `json:"tags"`
	OwnerID     *string   `json:"owner_id,omitempty"`
	ExternalURL *string   `json:"external_url,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// SLAConfig holds the per-org SLA remediation targets in days per severity.
type SLAConfig struct {
	OrgID        string `json:"org_id"`
	CriticalDays int    `json:"critical_days"`
	HighDays     int    `json:"high_days"`
	MediumDays   int    `json:"medium_days"`
	LowDays      int    `json:"low_days"`
}

// CreateAssetInput is the validated request body for creating an asset.
type CreateAssetInput struct {
	Name        string   `json:"name"         validate:"required,min=1,max=255"`
	Type        string   `json:"type"         validate:"required,oneof=server container webapp repository"`
	Criticality string   `json:"criticality"  validate:"required,oneof=low medium high critical"`
	Tags        []string `json:"tags"`
	OwnerID     *string  `json:"owner_id,omitempty"`
	ExternalURL string   `json:"external_url"`
}

// UpdateAssetInput is the validated request body for updating an asset.
type UpdateAssetInput struct {
	Name        *string  `json:"name"         validate:"omitempty,min=1,max=255"`
	Type        *string  `json:"type"         validate:"omitempty,oneof=server container webapp repository"`
	Criticality *string  `json:"criticality"  validate:"omitempty,oneof=low medium high critical"`
	Tags        []string `json:"tags"`
	OwnerID     *string  `json:"owner_id,omitempty"`
	ExternalURL *string  `json:"external_url,omitempty"`
}

// CSVAssetRow represents one row from a bulk-import CSV file.
type CSVAssetRow struct {
	Name        string
	Type        string
	Criticality string
	Tags        []string
	ExternalURL string
}

// Scan represents a scanner job record.
type Scan struct {
	ID           string     `json:"id"`
	OrgID        string     `json:"org_id"`
	AssetID      string     `json:"asset_id"`
	Scanner      string     `json:"scanner"`
	Status       string     `json:"status"`
	TargetURL    string     `json:"target_url,omitempty"`
	TargetIP     string     `json:"target_ip,omitempty"`
	ErrorMessage string     `json:"error_message,omitempty"`
	FindingCount int        `json:"finding_count"`
	DurationMs   *int64     `json:"duration_ms,omitempty"`
	StartedAt    *time.Time `json:"started_at,omitempty"`
	CompletedAt  *time.Time `json:"completed_at,omitempty"`
	CreatedAt    time.Time  `json:"created_at"`
}

// Finding represents a normalized vulnerability finding from any scanner.
type Finding struct {
	ID              string     `json:"id"`
	OrgID           string     `json:"org_id"`
	AssetID         string     `json:"asset_id"`
	ScanID          *string    `json:"scan_id,omitempty"`
	CVEID           *string    `json:"cve_id,omitempty"`
	Title           string     `json:"title"`
	Description     string     `json:"description,omitempty"`
	Severity        string     `json:"severity"`
	CVSSScore       *float64   `json:"cvss_score,omitempty"`
	EPSSScore       *float64   `json:"epss_score,omitempty"`
	EPSSPercentile  *float64   `json:"epss_percentile,omitempty"`
	RiskScore       *float64   `json:"risk_score,omitempty"`
	Status          string     `json:"status"`
	Scanner         string     `json:"scanner"`
	RawID           string     `json:"raw_id,omitempty"`
	Sources         []string   `json:"sources"`
	TemplateID      string     `json:"template_id,omitempty"`
	AssignedTo      *string    `json:"assigned_to,omitempty"`
	Justification   string     `json:"justification,omitempty"`
	ReopenCount     int        `json:"reopen_count"`
	OccurrenceCount int        `json:"occurrence_count"`
	LastSeenAt      time.Time  `json:"last_seen_at"`
	SLADueAt        *time.Time `json:"sla_due_at,omitempty"`
	CreatedAt       time.Time  `json:"created_at"`
	UpdatedAt       time.Time  `json:"updated_at"`
}

// FindingFilter holds filter parameters for listing findings.
type FindingFilter struct {
	Severity  string
	Status    string
	AssetID   string
	SortBy    string // "risk_score" | "created_at"
	Order     string // "asc" | "desc"
	Page      int
	Limit     int
}

// CreateScanInput is the validated request body for triggering a scan.
type CreateScanInput struct {
	Scanner   string `json:"scanner"    validate:"required,oneof=trivy nuclei openvas"`
	TargetURL string `json:"target_url" validate:"omitempty,url"`
	TargetIP  string `json:"target_ip"  validate:"omitempty,ip"`
	FailOn    *struct {
		Critical int `json:"critical"`
		High     int `json:"high"`
	} `json:"fail_on,omitempty"`
}

// UpdateFindingInput is the validated request body for updating a finding.
type UpdateFindingInput struct {
	Status        *string `json:"status"        validate:"omitempty,oneof=open in_progress resolved accepted_risk false_positive"`
	AssignedTo    *string `json:"assigned_to"`
	Justification *string `json:"justification"`
}

// BulkFindingInput is the request body for bulk-updating findings.
type BulkFindingInput struct {
	IDs        []string `json:"ids"         validate:"required,min=1"`
	Status     *string  `json:"status"      validate:"omitempty,oneof=open in_progress resolved accepted_risk false_positive"`
	AssignedTo *string  `json:"assigned_to"`
}

// SuppressionRule defines a rule that suppresses matching findings.
type SuppressionRule struct {
	ID         string    `json:"id"`
	OrgID      string    `json:"org_id"`
	CVEID      *string   `json:"cve_id,omitempty"`
	AssetTag   *string   `json:"asset_tag,omitempty"`
	Reason     string    `json:"reason"`
	CreatedBy  *string   `json:"created_by,omitempty"`
	MatchCount int       `json:"match_count"`
	CreatedAt  time.Time `json:"created_at"`
}

// CreateSuppressionInput is the validated request body for creating a suppression rule.
type CreateSuppressionInput struct {
	CVEID    *string `json:"cve_id"`
	AssetTag *string `json:"asset_tag"`
	Reason   string  `json:"reason" validate:"required"`
}

// ScanSchedule holds a recurring scan schedule for an asset.
type ScanSchedule struct {
	ID        string     `json:"id"`
	OrgID     string     `json:"org_id"`
	AssetID   string     `json:"asset_id"`
	Scanner   string     `json:"scanner"`
	CronExpr  string     `json:"cron_expr"`
	IsActive  bool       `json:"is_active"`
	LastRun   *time.Time `json:"last_run,omitempty"`
	NextRun   *time.Time `json:"next_run,omitempty"`
	CreatedAt time.Time  `json:"created_at"`
}

// CreateScanScheduleInput is the validated request body for creating a scan schedule.
type CreateScanScheduleInput struct {
	Scanner  string `json:"scanner"   validate:"required,oneof=trivy nuclei openvas"`
	CronExpr string `json:"cron_expr" validate:"required"`
}

// CIScanResult is the response from a CI gate scan.
type CIScanResult struct {
	Passed      bool `json:"passed"`
	NewFindings struct {
		Critical int `json:"critical"`
		High     int `json:"high"`
	} `json:"new_findings"`
	ScanID string `json:"scan_id"`
}

// SLAEntry is one row in the SLA dashboard: a single open finding annotated with
// the org's configured remediation deadline and whether that deadline has passed.
type SLAEntry struct {
	AssetID      string `json:"asset_id"`
	AssetName    string `json:"asset_name"`
	FindingID    string `json:"finding_id"`
	FindingTitle string `json:"finding_title"`
	Severity     string `json:"severity"`
	Status       string `json:"status"`
	// DaysOpen is the number of calendar days the finding has been open (since created_at).
	DaysOpen int `json:"days_open"`
	// SLADays is the org-configured remediation window for this severity (from vb_sla_config).
	SLADays int `json:"sla_days"`
	// Overdue is true when DaysOpen exceeds SLADays, meaning the remediation deadline has passed.
	Overdue bool `json:"overdue"`
}

// SBOMSummary holds metadata for a generated SBOM record.
type SBOMSummary struct {
	ID             string    `json:"id"`
	AssetID        string    `json:"asset_id"`
	Format         string    `json:"format"`
	ComponentCount int       `json:"component_count"`
	CreatedAt      time.Time `json:"created_at"`
}

// ComponentSummary holds one component row for the EOL dashboard.
type ComponentSummary struct {
	ID        string  `json:"id"`
	Name      string  `json:"name"`
	Version   string  `json:"version"`
	PURL      string  `json:"purl,omitempty"`
	EOLStatus string  `json:"eol_status"`
	EOLDate   *string `json:"eol_date,omitempty"`
	AssetID   string  `json:"asset_id"`
}

// RiskTrendPoint holds daily aggregated risk data.
type RiskTrendPoint struct {
	Date           string  `json:"date"`
	TotalRiskScore float64 `json:"total_risk_score"`
	OpenCount      int     `json:"open_count"`
	CriticalCount  int     `json:"critical_count"`
}

// Report holds metadata for a generated executive report.
type Report struct {
	ID          string                 `json:"id"`
	OrgID       string                 `json:"org_id"`
	GeneratedBy *string                `json:"generated_by,omitempty"`
	Title       string                 `json:"title"`
	Scope       map[string]interface{} `json:"scope"`
	FilePath    string                 `json:"file_path,omitempty"`
	Status      string                 `json:"status"`
	ExpiresAt   *time.Time             `json:"expires_at,omitempty"`
	CreatedAt   time.Time              `json:"created_at"`
}
