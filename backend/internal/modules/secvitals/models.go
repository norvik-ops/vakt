// Package secvitals provides domain models for compliance automation (NIS2, ISO 27001, BSI-Grundschutz).
package secvitals

import "time"

// Framework represents a compliance framework enabled for an organisation.
type Framework struct {
	ID             string    `json:"id"`
	OrgID          string    `json:"org_id"`
	Name           string    `json:"name"`
	Version        string    `json:"version"`
	IsBuiltin      bool      `json:"is_builtin"`
	ReadinessScore float64   `json:"readiness_score,omitempty"`
	CreatedAt      time.Time `json:"created_at"`
}

// Control represents an individual compliance control within a framework.
type Control struct {
	ID                  string `json:"id"`
	FrameworkID         string `json:"framework_id"`
	OrgID               string `json:"org_id"`
	ControlID           string `json:"control_id"`
	Title               string `json:"title"`
	Description         string `json:"description,omitempty"`
	Domain              string `json:"domain"`
	EvidenceType        string `json:"evidence_type"`
	Weight              int    `json:"weight"`
	EvidenceCount       int    `json:"evidence_count,omitempty"`
	Status              string `json:"status,omitempty"` // computed: covered/partial/missing/not_applicable/in_progress/implemented
	NotApplicable       bool   `json:"not_applicable"`
	NotApplicableReason string `json:"not_applicable_reason,omitempty"`
	ManualStatus        string `json:"manual_status,omitempty"` // "" | "in_progress" | "implemented"
	ISO27001Mapping     string `json:"iso27001_mapping,omitempty"`
	MaturityScore       int    `json:"maturity_score"` // 0–3 (TISAX VDA ISA maturity level)
	Owner               string `json:"owner,omitempty"`
	// Review tracking (Migration 075)
	LastReviewedAt     *time.Time `json:"last_reviewed_at"`
	ReviewIntervalDays int        `json:"review_interval_days"`
	NextReviewDue      *time.Time `json:"next_review_due"`
	LastReviewedBy     string     `json:"last_reviewed_by"`
	ReviewNote         string     `json:"review_note"`
	IsReviewOverdue    bool       `json:"is_review_overdue"` // computed: next_review_due < NOW() AND next_review_due IS NOT NULL
	DueDate            *time.Time `json:"due_date,omitempty"`
}

// ControlReview represents a single periodic review event for a control.
type ControlReview struct {
	ID             string    `json:"id"`
	ControlID      string    `json:"control_id"`
	ReviewedBy     string    `json:"reviewed_by"`
	ReviewNote     string    `json:"review_note"`
	StatusAtReview string    `json:"status_at_review"`
	ReviewedAt     time.Time `json:"reviewed_at"`
}

// RecordReviewInput holds validated input for recording a control review.
type RecordReviewInput struct {
	ReviewedBy     string `json:"reviewed_by"          validate:"required,max=200"`
	ReviewNote     string `json:"review_note"          validate:"max=2000"`
	ReviewInterval int    `json:"review_interval_days" validate:"omitempty,min=30,max=3650"`
}

// UpdateControlInput holds input for PATCH /secvitals/controls/:id.
// not_applicable takes precedence over manual_status; set manual_status="" to reset to computed.
type UpdateControlInput struct {
	NotApplicable bool    `json:"not_applicable"`
	Reason        string  `json:"reason"`
	ManualStatus  string  `json:"manual_status" validate:"omitempty,oneof=in_progress implemented"`
	MaturityScore *int    `json:"maturity_score" validate:"omitempty,min=0,max=3"`
	Owner         string  `json:"owner"          validate:"omitempty,max=200"`
	DueDate       *string `json:"due_date"      validate:"omitempty,datetime=2006-01-02"`
}

// BulkUpdateControlsInput holds input for PATCH /secvitals/controls/bulk.
type BulkUpdateControlsInput struct {
	IDs    []string `json:"ids"    validate:"required,min=1,max=100"`
	Status string   `json:"status" validate:"required,oneof=implemented in_progress not_implemented not_applicable"`
}

// BulkUpdateCAPAsInput holds input for PATCH /secvitals/capas/bulk.
type BulkUpdateCAPAsInput struct {
	IDs    []string `json:"ids"    validate:"required,min=1,max=100"`
	Status string   `json:"status" validate:"required,oneof=open in_progress implemented verified closed"`
}

// UpdateSoAMetadataInput holds SoA-specific fields for PATCH /secvitals/controls/:id/soa.
type UpdateSoAMetadataInput struct {
	Justification  string `json:"justification"`
	Implementation string `json:"implementation"`
	Responsible    string `json:"responsible"`
}

// SoARow is a flattened view of a control enriched with SoA metadata and evidence count,
// used exclusively by the SoA PDF generator.
type SoARow struct {
	ControlID      string
	Title          string
	Domain         string
	Applicable     bool
	Justification  string
	Implementation string
	Responsible    string
	ManualStatus   string
	EvidenceCount  int
}

// TISAXControlGap describes a single maturity gap in TISAX coverage.
type TISAXControlGap struct {
	Control      Control `json:"control"`
	MaturityGap  int     `json:"maturity_gap"`
	CurrentScore int     `json:"current_score"`
}

// TISAXGapAnalysis lists TISAX controls that have not yet reached full maturity.
type TISAXGapAnalysis struct {
	FrameworkID string            `json:"framework_id"`
	TargetScore int               `json:"target_score"`
	Gaps        []TISAXControlGap `json:"gaps"`
}

// Evidence represents a piece of compliance evidence attached to a control.
type Evidence struct {
	ID               string     `json:"id"`
	ControlID        string     `json:"control_id"`
	OrgID            string     `json:"org_id"`
	Title            string     `json:"title"`
	Description      string     `json:"description,omitempty"`
	Source           string     `json:"source"`
	FilePath         string     `json:"file_path,omitempty"`
	FileSize         int64      `json:"file_size,omitempty"`
	CollectorData    []byte     `json:"collector_data,omitempty"`
	Status           string     `json:"status"`
	Version          int        `json:"version"`
	ExpiresAt        *time.Time `json:"expires_at,omitempty"`
	ExpiryNotifiedAt *time.Time `json:"expiry_notified_at,omitempty"`
	UploadedBy       string     `json:"uploaded_by,omitempty"`
	ReviewedBy       string     `json:"reviewed_by,omitempty"`
	ReviewedAt       *time.Time `json:"reviewed_at,omitempty"`
	CreatedAt        time.Time  `json:"created_at"`
	UpdatedAt        time.Time  `json:"updated_at"`
}

// Review represents a control review assignment.
type Review struct {
	ID          string     `json:"id"`
	ControlID   string     `json:"control_id"`
	OrgID       string     `json:"org_id"`
	AssignedTo  string     `json:"assigned_to"`
	AssignedBy  string     `json:"assigned_by"`
	DueDate     time.Time  `json:"due_date"`
	Status      string     `json:"status"`
	Notes       string     `json:"notes,omitempty"`
	CompletedAt *time.Time `json:"completed_at,omitempty"`
	CreatedAt   time.Time  `json:"created_at"`
}

// AuditorLink represents a time-limited read-only access token for external auditors.
type AuditorLink struct {
	ID          string    `json:"id"`
	OrgID       string    `json:"org_id"`
	FrameworkID string    `json:"framework_id"`
	TokenHash   string    `json:"-"` // never exposed
	CreatedBy   string    `json:"created_by"`
	ExpiresAt   time.Time `json:"expires_at"`
	UsedCount   int       `json:"used_count"`
	MaxUses     *int      `json:"max_uses,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
	// ShareURL is populated on creation with the raw token embedded.
	ShareURL string `json:"share_url,omitempty"`
}

// AuditorLinkListItem is the response shape for listing auditor links (E09.1).
type AuditorLinkListItem struct {
	ID             string     `json:"id"`
	OrgID          string     `json:"org_id"`
	FrameworkID    string     `json:"framework_id"`
	Label          string     `json:"label"`
	CreatedBy      string     `json:"created_by"`
	ExpiresAt      time.Time  `json:"expires_at"`
	LastAccessedAt *time.Time `json:"last_accessed_at,omitempty"`
	AccessCount    int        `json:"access_count"`
	RevokedAt      *time.Time `json:"revoked_at,omitempty"`
	CreatedAt      time.Time  `json:"created_at"`
}

// ControlWithEvidence holds a control and its associated evidence items for auditor detail view (E09.2).
type ControlWithEvidence struct {
	Control  Control    `json:"control"`
	Evidence []Evidence `json:"evidence"`
}

// AuditorDetailView is the enhanced auditor view response (E09.2).
type AuditorDetailView struct {
	Framework Framework             `json:"framework"`
	Report    *ReadinessReport      `json:"report"`
	Controls  []ControlWithEvidence `json:"controls"`
}

// EvidenceMetadata is written into evidence_metadata.json inside the export ZIP (E09.3).
type EvidenceMetadata struct {
	Control  Control    `json:"control"`
	Evidence []Evidence `json:"evidence"`
}

// ChapterMaturity holds per-domain TISAX maturity data within a TISAXMaturitySummary.
type ChapterMaturity struct {
	Domain        string  `json:"domain"`
	AvgScore      float64 `json:"avg_score"`
	TotalControls int     `json:"total_controls"`
	FullyMature   int     `json:"fully_mature"`
	Color         string  `json:"color"` // "green" avg>=2.5, "yellow" avg>=1.5, "red" otherwise
}

// TISAXMaturitySummary summarises TISAX maturity across all domains.
type TISAXMaturitySummary struct {
	AvgScore         float64           `json:"avg_score"`
	ByChapter        []ChapterMaturity `json:"by_chapter"`
	ReadinessPercent float64           `json:"readiness_percent"`
}

// ReadinessReport summarises compliance readiness for a framework.
type ReadinessReport struct {
	FrameworkID    string                `json:"framework_id"`
	FrameworkName  string                `json:"framework_name"`
	TotalControls  int                   `json:"total_controls"`
	Covered        int                   `json:"covered"`
	Partial        int                   `json:"partial"`
	Missing        int                   `json:"missing"`
	ReadinessScore float64               `json:"readiness_score"`
	ByDomain       []DomainScore         `json:"by_domain"`
	TISAXMaturity  *TISAXMaturitySummary `json:"tisax_maturity,omitempty"`
}

// DomainScore holds per-domain readiness data within a ReadinessReport.
type DomainScore struct {
	Domain  string  `json:"domain"`
	Score   float64 `json:"score"`
	Total   int     `json:"total"`
	Covered int     `json:"covered"`
}

// GapAnalysis lists controls that are missing or at-risk evidence.
type GapAnalysis struct {
	FrameworkID string       `json:"framework_id"`
	Gaps        []ControlGap `json:"gaps"`
}

// ControlGap describes a single gap in compliance coverage.
type ControlGap struct {
	Control   Control    `json:"control"`
	Reason    string     `json:"reason"` // "no_evidence", "evidence_expiring", "review_pending"
	ExpiresAt *time.Time `json:"expires_at,omitempty"`
}

// EvidenceHistoryEntry represents a single audit history record for an evidence item.
type EvidenceHistoryEntry struct {
	ID         string    `json:"id"`
	EvidenceID string    `json:"evidence_id"`
	ChangedBy  *string   `json:"changed_by_id,omitempty"`
	ChangedAt  time.Time `json:"changed_at"`
	Title      string    `json:"title,omitempty"`
	Status     string    `json:"status,omitempty"`
	ChangeNote string    `json:"change_note,omitempty"`
}

// AddEvidenceInput holds validated input for adding evidence to a control.
type AddEvidenceInput struct {
	Title       string     `json:"title"       validate:"required,max=255"`
	Description string     `json:"description"`
	Source      string     `json:"source"      validate:"required,oneof=manual github aws azure ad"`
	FilePath    string     `json:"file_path"`
	FileSize    int64      `json:"file_size"`
	ExpiresAt   *time.Time `json:"expires_at"`
}

// --- Risk Assessment (FR-CK12) ---

// Risk represents a single entry in the organisation's risk register.
type Risk struct {
	ID             string `json:"id"`
	OrgID          string `json:"org_id"`
	Title          string `json:"title"`
	Description    string `json:"description,omitempty"`
	Category       string `json:"category,omitempty"`
	Likelihood     int    `json:"likelihood"` // 1–5
	Impact         int    `json:"impact"`     // 1–5
	RiskScore      int    `json:"risk_score"` // computed: likelihood * impact
	Owner          string `json:"owner,omitempty"`
	Status         string `json:"status"`    // open | mitigated | accepted | closed
	Treatment      string `json:"treatment"` // avoid | mitigate | transfer | accept
	TreatmentNotes string `json:"treatment_notes,omitempty"`
	// Treatment workflow fields (Migration 071)
	TreatmentOption    string     `json:"treatment_option"`
	TreatmentPlan      string     `json:"treatment_plan"`
	TreatmentOwner     string     `json:"treatment_owner"`
	TreatmentDueDate   *time.Time `json:"treatment_due_date"`
	TreatmentStatus    string     `json:"treatment_status"`
	ResidualLikelihood *int       `json:"residual_likelihood"`
	ResidualImpact     *int       `json:"residual_impact"`
	CreatedAt          time.Time  `json:"created_at"`
	UpdatedAt          time.Time  `json:"updated_at"`
}

// CreateRiskInput holds validated input for creating a risk entry.
type CreateRiskInput struct {
	Title          string `json:"title"       validate:"required,max=255"`
	Description    string `json:"description"`
	Category       string `json:"category"`
	Likelihood     int    `json:"likelihood"  validate:"required,min=1,max=5"`
	Impact         int    `json:"impact"      validate:"required,min=1,max=5"`
	Owner          string `json:"owner"`
	Treatment      string `json:"treatment"   validate:"required,oneof=avoid mitigate transfer accept"`
	TreatmentNotes string `json:"treatment_notes"`
}

// --- Incident Register (FR-CK13) ---

// Incident represents a security or operational incident.
type Incident struct {
	ID              string     `json:"id"`
	OrgID           string     `json:"org_id"`
	Title           string     `json:"title"`
	Description     string     `json:"description,omitempty"`
	Severity        string     `json:"severity"` // low | medium | high | critical
	Status          string     `json:"status"`   // open | investigating | resolved | closed
	DiscoveredAt    time.Time  `json:"discovered_at"`
	ResolvedAt      *time.Time `json:"resolved_at,omitempty"`
	AffectedSystems []string   `json:"affected_systems"`
	BreachID        *string    `json:"breach_id,omitempty"`
	// Deadline tracking (NIS2 / DORA)
	IncidentType          string     `json:"incident_type"`                    // general | nis2 | dora
	ReportingObligation   string     `json:"reporting_obligation"`             // unknown | required | not_required
	NotificationAuthority string     `json:"notification_authority,omitempty"` // BSI | BaFin | BNetzA | ...
	Deadline4h            *time.Time `json:"deadline_4h,omitempty"`
	Deadline24h           *time.Time `json:"deadline_24h,omitempty"`
	Deadline72h           *time.Time `json:"deadline_72h,omitempty"`
	Deadline30d           *time.Time `json:"deadline_30d,omitempty"`
	Reported4hAt          *time.Time `json:"reported_4h_at,omitempty"`
	Reported24hAt         *time.Time `json:"reported_24h_at,omitempty"`
	Reported72hAt         *time.Time `json:"reported_72h_at,omitempty"`
	Reported30dAt         *time.Time `json:"reported_30d_at,omitempty"`
	// DORA-specific fields (Migration 041)
	AffectedCustomers       *int    `json:"affected_customers,omitempty"`
	FinancialImpactEstimate *string `json:"financial_impact_estimate,omitempty"`
	IsMajorIncident         bool    `json:"is_major_incident"`
	// Supplier link (Migration 042)
	SupplierID *string `json:"supplier_id,omitempty"`
	// Dedup guards — set true once the 12h-before warning has been sent (Migration 053)
	NotifiedWarn24h bool `json:"-"`
	NotifiedWarn72h bool `json:"-"`
	NotifiedWarn30d bool `json:"-"`
	// Computed deadline status — populated by service layer, not stored
	DeadlineStatus *IncidentDeadlineStatus `json:"deadline_status,omitempty"`
	CreatedAt      time.Time               `json:"created_at"`
	UpdatedAt      time.Time               `json:"updated_at"`
}

// IncidentDeadlineStatus holds computed status for each reporting deadline.
type IncidentDeadlineStatus struct {
	Has4h  bool          `json:"has_4h"`
	Has24h bool          `json:"has_24h"`
	Has72h bool          `json:"has_72h"`
	Has30d bool          `json:"has_30d"`
	D4h    *DeadlineInfo `json:"d_4h,omitempty"`
	D24h   *DeadlineInfo `json:"d_24h,omitempty"`
	D72h   *DeadlineInfo `json:"d_72h,omitempty"`
	D30d   *DeadlineInfo `json:"d_30d,omitempty"`
}

// DeadlineInfo holds status for a single reporting deadline.
type DeadlineInfo struct {
	Deadline   *time.Time `json:"deadline"`
	ReportedAt *time.Time `json:"reported_at,omitempty"`
	Status     string     `json:"status"` // green | yellow | red | done
	HoursLeft  float64    `json:"hours_left"`
}

// CreateIncidentInput holds validated input for creating an incident record.
type CreateIncidentInput struct {
	Title                 string    `json:"title"                    validate:"required,max=255"`
	Description           string    `json:"description"              validate:"required"`
	Severity              string    `json:"severity"                 validate:"required,oneof=low medium high critical"`
	DiscoveredAt          time.Time `json:"discovered_at"`
	AffectedSystems       []string  `json:"affected_systems"`
	BreachID              *string   `json:"breach_id"`
	IncidentType          string    `json:"incident_type"            validate:"omitempty,oneof=general nis2 dora"`
	ReportingObligation   string    `json:"reporting_obligation"     validate:"omitempty,oneof=unknown required not_required"`
	NotificationAuthority string    `json:"notification_authority"`
	// DORA-specific fields (Migration 041)
	AffectedCustomers       *int    `json:"affected_customers"       validate:"omitempty,min=0"`
	FinancialImpactEstimate *string `json:"financial_impact_estimate"`
	IsMajorIncident         bool    `json:"is_major_incident"`
}

// MarkDeadlineReportedInput holds the deadline key to mark as reported.
type MarkDeadlineReportedInput struct {
	Deadline string `json:"deadline" validate:"required,oneof=4h 24h 72h 30d"`
}

// --- Policy Management (FR-CK14) ---

// Policy represents a managed policy document.
type Policy struct {
	ID            string     `json:"id"`
	OrgID         string     `json:"org_id"`
	Title         string     `json:"title"`
	Description   string     `json:"description,omitempty"`
	Category      string     `json:"category,omitempty"`
	Status        string     `json:"status"`       // draft | active | archived
	Version       string     `json:"version"`      // user-editable version label, e.g. "1.0"
	VersionNum    int        `json:"version_num"`  // auto-incremented integer version counter
	VersionNote   string     `json:"version_note"` // change summary for the latest version
	LastUpdatedBy string     `json:"last_updated_by"`
	ReviewedAt    *time.Time `json:"reviewed_at,omitempty"`
	NextReviewDue *string    `json:"next_review_due,omitempty"` // date string YYYY-MM-DD
	EffectiveDate *time.Time `json:"effective_date,omitempty"`
	ReviewDate    *time.Time `json:"review_date,omitempty"`
	Owner         string     `json:"owner,omitempty"`
	CreatedAt     time.Time  `json:"created_at"`
	UpdatedAt     time.Time  `json:"updated_at"`
}

// PolicyVersion holds a historical snapshot of a policy at a given version number.
type PolicyVersion struct {
	ID          string    `json:"id"`
	OrgID       string    `json:"org_id"`
	PolicyID    string    `json:"policy_id"`
	Version     int       `json:"version"`
	Title       string    `json:"title"`
	Content     string    `json:"content"`
	Status      string    `json:"status"`
	VersionNote string    `json:"version_note"`
	UpdatedBy   string    `json:"updated_by"`
	CreatedAt   time.Time `json:"created_at"`
}

// CreatePolicyInput holds validated input for creating a policy.
type CreatePolicyInput struct {
	Title         string     `json:"title"    validate:"required,max=255"`
	Description   string     `json:"description"`
	Category      string     `json:"category"`
	Version       string     `json:"version"`
	EffectiveDate *time.Time `json:"effective_date"`
	ReviewDate    *time.Time `json:"review_date"`
	Owner         string     `json:"owner"`
}

// --- Internal Audit Records (FR-CK15) ---

// AuditRecord represents an internal audit.
type AuditRecord struct {
	ID              string    `json:"id"`
	OrgID           string    `json:"org_id"`
	Title           string    `json:"title"`
	Scope           string    `json:"scope,omitempty"`
	Auditor         string    `json:"auditor,omitempty"`
	AuditDate       time.Time `json:"audit_date"`
	Status          string    `json:"status"` // planned | in_progress | completed
	Findings        string    `json:"findings,omitempty"`
	Recommendations string    `json:"recommendations,omitempty"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
}

// CreateAuditRecordInput holds validated input for creating an audit record.
type CreateAuditRecordInput struct {
	Title           string    `json:"title"      validate:"required,max=255"`
	Scope           string    `json:"scope"`
	Auditor         string    `json:"auditor"`
	AuditDate       time.Time `json:"audit_date" validate:"required"`
	Findings        string    `json:"findings"`
	Recommendations string    `json:"recommendations"`
}

// --- Update inputs ---

type UpdateRiskInput struct {
	Title          string `json:"title"       validate:"required,max=255"`
	Description    string `json:"description"`
	Category       string `json:"category"`
	Likelihood     int    `json:"likelihood"  validate:"required,min=1,max=5"`
	Impact         int    `json:"impact"      validate:"required,min=1,max=5"`
	Owner          string `json:"owner"`
	Status         string `json:"status"      validate:"required,oneof=open mitigated accepted closed"`
	Treatment      string `json:"treatment"   validate:"required,oneof=avoid mitigate transfer accept"`
	TreatmentNotes string `json:"treatment_notes"`
}

// UpdateRiskTreatmentInput holds the treatment workflow fields for PATCH /risks/:id/treatment.
type UpdateRiskTreatmentInput struct {
	TreatmentOption    string  `json:"treatment_option"    validate:"omitempty,oneof=accept mitigate transfer avoid"`
	TreatmentPlan      string  `json:"treatment_plan"      validate:"max=5000"`
	TreatmentOwner     string  `json:"treatment_owner"     validate:"max=200"`
	TreatmentDueDate   *string `json:"treatment_due_date"`
	TreatmentStatus    string  `json:"treatment_status"    validate:"omitempty,oneof=pending in_progress implemented verified"`
	ResidualLikelihood *int    `json:"residual_likelihood" validate:"omitempty,min=1,max=5"`
	ResidualImpact     *int    `json:"residual_impact"     validate:"omitempty,min=1,max=5"`
}

type UpdateIncidentInput struct {
	Title                 string   `json:"title"                    validate:"required,max=255"`
	Description           string   `json:"description"              validate:"required"`
	Severity              string   `json:"severity"                 validate:"required,oneof=low medium high critical"`
	Status                string   `json:"status"                   validate:"required,oneof=open investigating resolved closed"`
	AffectedSystems       []string `json:"affected_systems"`
	IncidentType          string   `json:"incident_type"            validate:"omitempty,oneof=general nis2 dora"`
	ReportingObligation   string   `json:"reporting_obligation"     validate:"omitempty,oneof=unknown required not_required"`
	NotificationAuthority string   `json:"notification_authority"`
	// DORA-specific fields (Migration 041)
	AffectedCustomers       *int    `json:"affected_customers"       validate:"omitempty,min=0"`
	FinancialImpactEstimate *string `json:"financial_impact_estimate"`
	IsMajorIncident         bool    `json:"is_major_incident"`
}

// --- Reportability Assessment (Story 31.1) ---

// ReportabilityAnswers stores answers to the NIS2 meldepflicht questionnaire.
type ReportabilityAnswers struct {
	AffectsExternalData     bool `json:"affects_external_data"`
	AffectsEssentialService bool `json:"affects_essential_service"`
	PersonalDataCompromised bool `json:"personal_data_compromised"`
}

// AssessReportabilityInput is the handler input for the questionnaire.
type AssessReportabilityInput struct {
	ReportabilityAnswers
}

// ReportabilityResult is returned after assessing an incident's reporting obligation.
type ReportabilityResult struct {
	Obligation            string               `json:"obligation"` // required | not_required | unknown
	GDPRRequired          bool                 `json:"gdpr_required"`
	NotificationAuthority string               `json:"notification_authority"`
	Explanation           string               `json:"explanation"`
	Answers               ReportabilityAnswers `json:"answers"`
}

// --- Incident Report Archive (Story 31.3) ---

// IncidentReport is an archived generated meldungsformular entry.
type IncidentReport struct {
	ID          string    `json:"id"`
	OrgID       string    `json:"org_id"`
	IncidentID  string    `json:"incident_id"`
	ReportType  string    `json:"report_type"` // 24h | 72h | 30d
	Authority   string    `json:"authority"`
	Metadata    any       `json:"metadata,omitempty"`
	GeneratedAt time.Time `json:"generated_at"`
}

// AuthorityInfo contains submission channel info for a reporting authority.
type AuthorityInfo struct {
	Name       string `json:"name"`
	Portal     string `json:"portal"`
	Phone      string `json:"phone"`
	SubmitNote string `json:"submit_note"`
}

// incidentAuthorityDirectory maps authority keys to contact information.
var incidentAuthorityDirectory = map[string]AuthorityInfo{
	"BSI": {
		Name:       "Bundesamt für Sicherheit in der Informationstechnik (BSI)",
		Portal:     "https://meldung.bsi.bund.de",
		Phone:      "+49 228 9582-777",
		SubmitNote: "Meldung über das BSI MELDUNG Portal einreichen oder per Fax an +49 228 9582-5777.",
	},
	"BaFin": {
		Name:       "Bundesanstalt für Finanzdienstleistungsaufsicht (BaFin)",
		Portal:     "https://www.bafin.de",
		Phone:      "+49 228 4108-0",
		SubmitNote: "Meldung per BaFin-Meldeplattform oder schriftlich einreichen.",
	},
	"BNetzA": {
		Name:       "Bundesnetzagentur (BNetzA)",
		Portal:     "https://www.bundesnetzagentur.de",
		Phone:      "+49 228 14-0",
		SubmitNote: "Meldung per Online-Formular oder schriftlich an die BNetzA.",
	},
	"LBA": {
		Name:       "Luftfahrt-Bundesamt (LBA)",
		Portal:     "https://www.lba.de",
		Phone:      "+49 531 2355-0",
		SubmitNote: "Meldung schriftlich oder per E-Mail an das Luftfahrt-Bundesamt sowie parallel an das BSI.",
	},
}

// sectorAuthorityMap maps org sector codes to the relevant authority keys.
var sectorAuthorityMap = map[string][]string{
	"energy":       {"BNetzA", "BSI"},
	"water":        {"BSI"},
	"health":       {"BSI"},
	"finance":      {"BaFin", "BSI"},
	"transport":    {"BSI"},
	"telecom":      {"BNetzA", "BSI"},
	"waste":        {"BSI"},
	"aerospace":    {"LBA", "BSI"},
	"public_admin": {"BSI"},
	"other":        {"BSI"},
}

// OrgSectorSettings holds the sector and federal state configured for an org.
type OrgSectorSettings struct {
	Sector       string `json:"sector"`
	FederalState string `json:"federal_state,omitempty"`
}

// UpdateOrgSectorInput is the handler input for updating sector settings.
type UpdateOrgSectorInput struct {
	Sector       string `json:"sector"        validate:"required,oneof=energy water health finance transport telecom waste aerospace public_admin other"`
	FederalState string `json:"federal_state"`
}

type UpdatePolicyInput struct {
	Title         string     `json:"title"    validate:"required,max=255"`
	Description   string     `json:"description"`
	Category      string     `json:"category"`
	Status        string     `json:"status"   validate:"required,oneof=draft active archived"`
	Version       string     `json:"version"`
	EffectiveDate *time.Time `json:"effective_date"`
	ReviewDate    *time.Time `json:"review_date"`
	Owner         string     `json:"owner"`
	// Versioning fields (Migration 076)
	VersionNote   *string `json:"version_note"    validate:"omitempty,max=500"`
	UpdatedBy     *string `json:"updated_by"      validate:"omitempty,max=200"`
	NextReviewDue *string `json:"next_review_due"`
}

type UpdateAuditRecordInput struct {
	Title           string    `json:"title"      validate:"required,max=255"`
	Scope           string    `json:"scope"`
	Auditor         string    `json:"auditor"`
	AuditDate       time.Time `json:"audit_date" validate:"required"`
	Status          string    `json:"status"     validate:"required,oneof=planned in_progress completed"`
	Findings        string    `json:"findings"`
	Recommendations string    `json:"recommendations"`
}

// --- Control Implementation Tasks ---

// ControlTask is a user-created implementation step for a compliance control.
type ControlTask struct {
	ID        string    `json:"id"`
	ControlID string    `json:"control_id"`
	OrgID     string    `json:"org_id"`
	Text      string    `json:"text"`
	Completed bool      `json:"completed"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// CreateControlTaskInput holds validated input for creating a control task.
type CreateControlTaskInput struct {
	Text string `json:"text" validate:"required,max=500"`
}

// UpdateControlTaskInput holds validated input for toggling a task's completion.
type UpdateControlTaskInput struct {
	Completed bool `json:"completed"`
}

// --- Supplier Register (NIS2 Art. 21 / DORA Art. 28) ---

// Supplier represents a third-party supplier in the supplier register.
type Supplier struct {
	ID           string     `json:"id"`
	OrgID        string     `json:"org_id"`
	Name         string     `json:"name"`
	ContactName  string     `json:"contact_name,omitempty"`
	ContactEmail string     `json:"contact_email,omitempty"`
	ServiceType  string     `json:"service_type,omitempty"`
	Criticality  string     `json:"criticality"` // standard | important | critical
	NIS2Relevant bool       `json:"nis2_relevant"`
	DORARelevant bool       `json:"dora_relevant"`
	ContractEnd  *time.Time `json:"contract_end,omitempty"`
	Notes        string     `json:"notes,omitempty"`
	// DORA-specific fields (Migration 042)
	SubSuppliers       []string `json:"sub_suppliers,omitempty"`
	DataLocation       string   `json:"data_location,omitempty"`
	ExitStrategyExists bool     `json:"exit_strategy_exists"`
	// Assessment fields (Migration 046)
	AssessmentStatus string     `json:"assessment_status"` // none | pending | completed
	LastAssessmentAt *time.Time `json:"last_assessment_at,omitempty"`
	// Computed — not stored in DB
	ContractStatus string    `json:"contract_status,omitempty"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

// CreateSupplierInput holds validated input for creating a supplier.
type CreateSupplierInput struct {
	Name         string     `json:"name"                  validate:"required,max=255"`
	ContactName  string     `json:"contact_name"`
	ContactEmail string     `json:"contact_email"         validate:"omitempty,email"`
	ServiceType  string     `json:"service_type"`
	Criticality  string     `json:"criticality"           validate:"omitempty,oneof=standard important critical"`
	NIS2Relevant bool       `json:"nis2_relevant"`
	DORARelevant bool       `json:"dora_relevant"`
	ContractEnd  *time.Time `json:"contract_end"`
	Notes        string     `json:"notes"`
	// DORA-specific fields (Migration 042)
	SubSuppliers       []string `json:"sub_suppliers"`
	DataLocation       string   `json:"data_location"         validate:"omitempty,oneof=EU NonEU Hybrid"`
	ExitStrategyExists bool     `json:"exit_strategy_exists"`
	// Assessment fields (Migration 046)
	AssessmentStatus string     `json:"assessment_status"     validate:"omitempty,oneof=none pending completed"`
	LastAssessmentAt *time.Time `json:"last_assessment_at"`
}

// UpdateSupplierInput holds validated input for updating a supplier.
type UpdateSupplierInput struct {
	Name         string     `json:"name"                  validate:"required,max=255"`
	ContactName  string     `json:"contact_name"`
	ContactEmail string     `json:"contact_email"         validate:"omitempty,email"`
	ServiceType  string     `json:"service_type"`
	Criticality  string     `json:"criticality"           validate:"omitempty,oneof=standard important critical"`
	NIS2Relevant bool       `json:"nis2_relevant"`
	DORARelevant bool       `json:"dora_relevant"`
	ContractEnd  *time.Time `json:"contract_end"`
	Notes        string     `json:"notes"`
	// DORA-specific fields (Migration 042)
	SubSuppliers       []string `json:"sub_suppliers"`
	DataLocation       string   `json:"data_location"         validate:"omitempty,oneof=EU NonEU Hybrid"`
	ExitStrategyExists bool     `json:"exit_strategy_exists"`
	// Assessment fields (Migration 046)
	AssessmentStatus string     `json:"assessment_status"     validate:"omitempty,oneof=none pending completed"`
	LastAssessmentAt *time.Time `json:"last_assessment_at"`
}

// SupplierFilter holds optional filter parameters for listing suppliers.
type SupplierFilter struct {
	Criticality      string
	AssessmentStatus string
}

// CSVImportError describes a single row-level error during CSV import.
type CSVImportError struct {
	Row     int    `json:"row"`
	Message string `json:"message"`
}

// CSVImportResult summarises the outcome of a supplier CSV import.
type CSVImportResult struct {
	Imported int              `json:"imported"`
	Skipped  int              `json:"skipped"`
	Errors   []CSVImportError `json:"errors"`
}

// --- Resilience Tests (DORA Art. 24-27) ---

// ResilienceTest represents a TLPT, pentest, or other resilience test record.
type ResilienceTest struct {
	ID                string    `json:"id"`
	OrgID             string    `json:"org_id"`
	Type              string    `json:"type"`
	Scope             string    `json:"scope,omitempty"`
	Provider          string    `json:"provider,omitempty"`
	TestDate          time.Time `json:"test_date"`
	Summary           string    `json:"summary,omitempty"`
	RemediationStatus string    `json:"remediation_status"`
	AttachmentURL     string    `json:"attachment_url,omitempty"`
	OverdueWarning    bool      `json:"overdue_warning,omitempty"` // computed
	CreatedAt         time.Time `json:"created_at"`
	UpdatedAt         time.Time `json:"updated_at"`
}

// CreateResilienceTestInput holds validated input for creating a resilience test entry.
type CreateResilienceTestInput struct {
	Type              string    `json:"type"               validate:"required,oneof=tlpt pentest scenario_based vulnerability_assessment"`
	Scope             string    `json:"scope"`
	Provider          string    `json:"provider"`
	TestDate          time.Time `json:"test_date"          validate:"required"`
	Summary           string    `json:"summary"`
	RemediationStatus string    `json:"remediation_status" validate:"omitempty,oneof=open in_progress completed accepted"`
}

// UpdateResilienceTestInput holds validated input for updating a resilience test entry.
type UpdateResilienceTestInput struct {
	Type              string    `json:"type"               validate:"required,oneof=tlpt pentest scenario_based vulnerability_assessment"`
	Scope             string    `json:"scope"`
	Provider          string    `json:"provider"`
	TestDate          time.Time `json:"test_date"          validate:"required"`
	Summary           string    `json:"summary"`
	RemediationStatus string    `json:"remediation_status" validate:"required,oneof=open in_progress completed accepted"`
}

// --- Framework Mappings (Story 28.2) ---

// FrameworkMapping represents a cross-framework control mapping stored in ck_framework_mappings.
type FrameworkMapping struct {
	ID              string    `json:"id"`
	OrgID           string    `json:"org_id"`
	SourceControlID string    `json:"source_control_id"`
	TargetControlID string    `json:"target_control_id"`
	CreatedAt       time.Time `json:"created_at"`
}

// ControlMapping represents a row from the global ck_framework_control_mappings table
// after JOIN-resolution to org-specific control UUIDs.
type ControlMapping struct {
	ID                string `json:"id"`
	SourceFramework   string `json:"source_framework"`
	SourceControlCode string `json:"source_control_code"`
	TargetFramework   string `json:"target_framework"`
	TargetControlCode string `json:"target_control_code"`
	MappingType       string `json:"mapping_type"`
	// Resolved fields populated by JOIN at query time.
	TargetControlID     string `json:"target_control_id"`
	TargetControlTitle  string `json:"target_control_title"`
	TargetFrameworkName string `json:"target_framework_name"`
}

// MappingResult describes a TISAX control together with its mapped ISO 27001 control and coverage status.
type MappingResult struct {
	TISAXControlID    string `json:"tisax_control_id"`
	TISAXControlTitle string `json:"tisax_control_title"`
	ISOControlID      string `json:"iso_control_id"`
	ISOControlTitle   string `json:"iso_control_title"`
	Covered           bool   `json:"covered"`
}

// --- Questionnaire Builder (Story 29.2) ---

// Questionnaire represents a supplier/compliance questionnaire.
type Questionnaire struct {
	ID          string     `json:"id"`
	OrgID       string     `json:"org_id"`
	Name        string     `json:"name"`
	Description string     `json:"description,omitempty"`
	IsTemplate  bool       `json:"is_template"`
	Questions   []Question `json:"questions,omitempty"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
}

// Question represents a single question in a questionnaire.
type Question struct {
	ID              string    `json:"id"`
	QuestionnaireID string    `json:"questionnaire_id"`
	OrderIdx        int       `json:"order_idx"`
	QuestionText    string    `json:"question_text"`
	QuestionType    string    `json:"question_type"` // yes_no | multiple_choice | free_text | file_upload
	Options         []string  `json:"options,omitempty"`
	Required        bool      `json:"required"`
	ControlID       *string   `json:"control_id,omitempty"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
}

// CreateQuestionnaireInput holds validated input for creating a questionnaire.
type CreateQuestionnaireInput struct {
	Name        string `json:"name"         validate:"required,max=500"`
	Description string `json:"description"`
	IsTemplate  bool   `json:"is_template"`
	CloneFromID string `json:"clone_from_id"`
}

// UpdateQuestionnaireInput holds validated input for updating a questionnaire (no clone_from_id).
type UpdateQuestionnaireInput struct {
	Name        string `json:"name"        validate:"required,max=500"`
	Description string `json:"description"`
	IsTemplate  bool   `json:"is_template"`
}

// CreateQuestionInput holds validated input for creating a question.
type CreateQuestionInput struct {
	QuestionText string   `json:"question_text" validate:"required,max=1000"`
	QuestionType string   `json:"question_type" validate:"required,oneof=yes_no multiple_choice free_text file_upload"`
	Options      []string `json:"options"`
	Required     bool     `json:"required"`
	ControlID    string   `json:"control_id"`
}

// ReorderQuestionsInput holds a new ordering of question IDs.
type ReorderQuestionsInput struct {
	Order []string `json:"order" validate:"required,min=1"`
}

// --- Supplier Portal Assessments (Story 29.3) ---

// Assessment represents a supplier portal assessment sent to a supplier.
type Assessment struct {
	ID              string     `json:"id"`
	OrgID           string     `json:"org_id"`
	SupplierID      string     `json:"supplier_id"`
	QuestionnaireID string     `json:"questionnaire_id"`
	TokenHash       string     `json:"-"` // never expose raw hash
	ExpiresAt       time.Time  `json:"expires_at"`
	Status          string     `json:"status"`
	SubmittedAt     *time.Time `json:"submitted_at,omitempty"`
	SubmittedByIP   string     `json:"-"`
	UserAgent       string     `json:"-"`
	CreatedAt       time.Time  `json:"created_at"`
}

// AssessmentWithQuestionnaire holds an assessment together with its questionnaire and questions.
type AssessmentWithQuestionnaire struct {
	Assessment
	Questionnaire *Questionnaire `json:"questionnaire"`
	ShareURL      string         `json:"share_url,omitempty"`
}

// CreateAssessmentInput holds validated input for creating a supplier assessment.
type CreateAssessmentInput struct {
	QuestionnaireID string `json:"questionnaire_id" validate:"required,uuid"`
	ExpiresInDays   int    `json:"expires_in_days"   validate:"required,min=1,max=365"`
}

// AnswerInput holds a single answer from a supplier portal submission.
type AnswerInput struct {
	QuestionID    string   `json:"question_id"    validate:"required,uuid"`
	AnswerText    string   `json:"answer_text"`
	AnswerBool    *bool    `json:"answer_bool"`
	AnswerOptions []string `json:"answer_options"`
	FileURL       string   `json:"file_url"`
}

// SaveAnswersInput holds a set of answers for intermediate save or submission.
type SaveAnswersInput struct {
	Answers []AnswerInput `json:"answers" validate:"required"`
}

// --- DORA Dashboard (Story 27.5) ---

// DORADashboard holds the computed DORA readiness summary for the dashboard.
type DORADashboard struct {
	ReadinessPct         float64       `json:"readiness_pct"`
	OpenCriticalControls int           `json:"open_critical_controls"`
	NextDeadline         *NextDeadline `json:"next_deadline,omitempty"`
	ExpiredSuppliers     int           `json:"expired_suppliers"`
	TLPTOverdueWarning   bool          `json:"tlpt_overdue_warning"`
}

// NextDeadline holds the nearest unreported DORA deadline.
type NextDeadline struct {
	IncidentID   string    `json:"incident_id"`
	Title        string    `json:"title"`
	DeadlineType string    `json:"deadline_type"` // "4h" | "24h" | "72h" | "30d"
	DeadlineAt   time.Time `json:"deadline_at"`
}

// --- Assessment Review & Evidence Import (Story 29.4) ---

// ReviewAnswerInput holds validated input for reviewing a single supplier answer.
type ReviewAnswerInput struct {
	ReviewStatus string `json:"review_status" validate:"required,oneof=accepted needs_rework"`
	ReworkNote   string `json:"rework_note"`
}

// AnswerWithQuestion holds a supplier answer joined with its question and control info.
type AnswerWithQuestion struct {
	AnswerID       string
	AssessmentID   string
	OrgID          string
	QuestionID     string
	QuestionText   string
	ControlID      *string
	AnswerText     string
	FileURL        string
	ReviewStatus   *string
	ReworkNote     *string
	CertExpiryDate *time.Time
}

// AnswerWithReview is the response shape for listing answers including review status.
type AnswerWithReview struct {
	ID             string     `json:"id"`
	QuestionText   string     `json:"question_text"`
	AnswerText     string     `json:"answer_text"`
	FileURL        string     `json:"file_url"`
	ReviewStatus   *string    `json:"review_status"`
	ReworkNote     *string    `json:"rework_note"`
	ControlID      *string    `json:"control_id"`
	CertExpiryDate *time.Time `json:"cert_expiry_date"`
	EvidenceID     *string    `json:"evidence_id"`
}

// SupplierStatus holds the computed traffic-light status for a supplier.
type SupplierStatus struct {
	SupplierID string         `json:"supplier_id"`
	Status     string         `json:"status"` // green | yellow | red
	Score      int            `json:"score"`  // 0–100
	Details    map[string]any `json:"details"`
}

// CertExpiryWarning describes a soon-expiring certificate answer.
type CertExpiryWarning struct {
	SupplierID     string    `json:"supplier_id"`
	SupplierName   string    `json:"supplier_name"`
	AnswerID       string    `json:"answer_id"`
	QuestionText   string    `json:"question_text"`
	CertExpiryDate time.Time `json:"cert_expiry_date"`
	FileURL        string    `json:"file_url"`
}

// UpdateAssessmentInput holds validated input for updating an assessment status.
type UpdateAssessmentInput struct {
	Status string `json:"status" validate:"required,oneof=reviewed"`
}

// --- AI System Inventory (EU AI Act) ---

// AISystem represents a KI-System in the organisation's AI inventory.
type AISystem struct {
	ID                      string     `json:"id"`
	OrgID                   string     `json:"org_id"`
	Name                    string     `json:"name"`
	Description             string     `json:"description,omitempty"`
	Provider                string     `json:"provider,omitempty"`
	UseCase                 string     `json:"use_case,omitempty"`
	AffectedGroups          string     `json:"affected_groups,omitempty"`
	AutonomyLevel           string     `json:"autonomy_level"` // assistive | partial | full
	InProductionSince       *time.Time `json:"in_production_since,omitempty"`
	Status                  string     `json:"status"`               // under_review | approved | prohibited | decommissioned
	RiskClass               string     `json:"risk_class,omitempty"` // minimal | limited | high | unacceptable
	ClassificationRationale string     `json:"classification_rationale,omitempty"`
	ClassifiedAt            *time.Time `json:"classified_at,omitempty"`
	ClassifiedBy            string     `json:"classified_by,omitempty"`
	CreatedAt               time.Time  `json:"created_at"`
	UpdatedAt               time.Time  `json:"updated_at"`
}

// AISystemFilters holds optional filter criteria for listing AI systems.
type AISystemFilters struct {
	RiskClass string
	Status    string
}

// CreateAISystemInput holds validated input for creating an AI system entry.
type CreateAISystemInput struct {
	Name                    string     `json:"name"           validate:"required,max=255"`
	Description             string     `json:"description"`
	Provider                string     `json:"provider"`
	UseCase                 string     `json:"use_case"`
	AffectedGroups          string     `json:"affected_groups"`
	AutonomyLevel           string     `json:"autonomy_level" validate:"omitempty,oneof=assistive partial full"`
	InProductionSince       *time.Time `json:"in_production_since"`
	RiskClass               string     `json:"risk_class"     validate:"omitempty,oneof=minimal limited high unacceptable"`
	ClassificationRationale string     `json:"classification_rationale"`
}

// UpdateAISystemInput holds validated input for updating an AI system entry.
type UpdateAISystemInput struct {
	Name                    string     `json:"name"           validate:"required,max=255"`
	Description             string     `json:"description"`
	Provider                string     `json:"provider"`
	UseCase                 string     `json:"use_case"`
	AffectedGroups          string     `json:"affected_groups"`
	AutonomyLevel           string     `json:"autonomy_level" validate:"omitempty,oneof=assistive partial full"`
	InProductionSince       *time.Time `json:"in_production_since"`
	Status                  string     `json:"status"         validate:"omitempty,oneof=under_review approved prohibited decommissioned"`
	RiskClass               string     `json:"risk_class"     validate:"omitempty,oneof=minimal limited high unacceptable"`
	ClassificationRationale string     `json:"classification_rationale"`
	ClassifiedBy            string     `json:"classified_by"`
}

// AIClassification records a single risk classification event for an AI system.
type AIClassification struct {
	ID            string         `json:"id"`
	OrgID         string         `json:"org_id"`
	AISystemID    string         `json:"ai_system_id"`
	RiskClass     string         `json:"risk_class"`
	Rationale     string         `json:"rationale,omitempty"`
	ClassifiedBy  string         `json:"classified_by,omitempty"`
	WizardAnswers map[string]any `json:"wizard_answers,omitempty"`
	ClassifiedAt  time.Time      `json:"classified_at"`
}

// ClassifyAISystemInput holds the payload for saving a wizard classification result.
type ClassifyAISystemInput struct {
	RiskClass     string         `json:"risk_class"     validate:"required,oneof=minimal limited high unacceptable"`
	Rationale     string         `json:"rationale"`
	ClassifiedBy  string         `json:"classified_by"`
	WizardAnswers map[string]any `json:"wizard_answers"`
}

// AIDocumentation stores the technical dossier for a high-risk AI system (Art. 11, Annex IV EU AI Act).
type AIDocumentation struct {
	ID                 string    `json:"id"`
	OrgID              string    `json:"org_id"`
	AISystemID         string    `json:"ai_system_id"`
	Version            int       `json:"version"`
	SystemDescription  string    `json:"system_description,omitempty"`
	IntendedPurpose    string    `json:"intended_purpose,omitempty"`
	TrainingData       string    `json:"training_data,omitempty"`
	DataQuality        string    `json:"data_quality,omitempty"`
	PerformanceMetrics string    `json:"performance_metrics,omitempty"`
	SystemLimits       string    `json:"system_limits,omitempty"`
	RiskManagement     string    `json:"risk_management,omitempty"`
	HumanOversight     string    `json:"human_oversight,omitempty"`
	LoggingAuditTrail  string    `json:"logging_audit_trail,omitempty"`
	AuthoredBy         string    `json:"authored_by,omitempty"`
	Status             string    `json:"status"` // draft | final
	CreatedAt          time.Time `json:"created_at"`
	UpdatedAt          time.Time `json:"updated_at"`
}

// UpsertAIDocumentationInput is used for creating or updating a documentation draft.
type UpsertAIDocumentationInput struct {
	SystemDescription  string `json:"system_description"`
	IntendedPurpose    string `json:"intended_purpose"`
	TrainingData       string `json:"training_data"`
	DataQuality        string `json:"data_quality"`
	PerformanceMetrics string `json:"performance_metrics"`
	SystemLimits       string `json:"system_limits"`
	RiskManagement     string `json:"risk_management"`
	HumanOversight     string `json:"human_oversight"`
	LoggingAuditTrail  string `json:"logging_audit_trail"`
	AuthoredBy         string `json:"authored_by"`
	Status             string `json:"status" validate:"omitempty,oneof=draft final"`
}

// EUAIActISOMappingEntry represents a mapping between EU AI Act requirements and ISO 27001 controls.
type EUAIActISOMappingEntry struct {
	EUAIActArticle  string `json:"eu_ai_act_article"`
	EUAIActTopic    string `json:"eu_ai_act_topic"`
	ISO27001Control string `json:"iso27001_control"`
	ISO27001Title   string `json:"iso27001_title"`
}

// EUAIActDashboard aggregates EU AI Act compliance status across all AI systems.
type EUAIActDashboard struct {
	TotalSystems             int                      `json:"total_systems"`
	SystemsByRiskClass       map[string]int           `json:"systems_by_risk_class"`
	SystemsByStatus          map[string]int           `json:"systems_by_status"`
	SystemsWithoutDocs       int                      `json:"systems_without_documentation"`
	HighRiskDeadline         string                   `json:"high_risk_deadline"`
	HighRiskDeadlineDaysLeft int                      `json:"high_risk_deadline_days_left"`
	ISO27001Mappings         []EUAIActISOMappingEntry `json:"iso27001_mappings"`
}

// euAIActISOMappings holds the static EU AI Act ↔ ISO 27001 mapping.
var euAIActISOMappings = []EUAIActISOMappingEntry{
	{"Art. 9", "Risikomanagementsystem", "A.6.1", "Maßnahmen zur Informationssicherheit im Risikomanagement"},
	{"Art. 9", "Risikomanagementsystem", "A.6.2", "Behandlung von Informationssicherheitsrisiken"},
	{"Art. 10", "Datensteuerung und -qualität", "A.8.1", "Klassifizierung von Informationen"},
	{"Art. 10", "Datensteuerung und -qualität", "A.5.12", "Klassifizierung von Informationen"},
	{"Art. 11", "Technische Dokumentation", "A.5.1", "Richtlinien für Informationssicherheit"},
	{"Art. 11", "Technische Dokumentation", "A.5.37", "Dokumentation der Betriebsverfahren"},
	{"Art. 12", "Protokollierung und Monitoring", "A.8.15", "Protokollierung"},
	{"Art. 12", "Protokollierung und Monitoring", "A.8.17", "Zeitsynchronisation"},
	{"Art. 14", "Menschliche Aufsicht", "A.6.7", "Telearbeit"},
	{"Art. 14", "Menschliche Aufsicht", "A.8.6", "Kapazitätsmanagement"},
	{"Art. 15", "Genauigkeit und Robustheit", "A.8.8", "Behandlung von technischen Schwachstellen"},
	{"Art. 17", "Qualitätsmanagementsystem", "A.5.35", "Unabhängige Überprüfung der Informationssicherheit"},
}

// GeneratePolicyDraftInput holds validated input for generating a policy draft via AI.
type GeneratePolicyDraftInput struct {
	PolicyType    string `json:"policy_type"    validate:"required"`
	FrameworkID   string `json:"framework_id"`
	OrgName       string `json:"org_name"`
	CustomContext string `json:"custom_context"`
}

// --- Maßnahmen-Katalog (control measures) ---

// ControlMeasure represents a recommended implementation measure for a compliance control.
type ControlMeasure struct {
	ID          string    `json:"id"`
	ControlID   string    `json:"control_id"`
	OrgID       string    `json:"org_id"`
	Title       string    `json:"title"`
	Description string    `json:"description"`
	Difficulty  string    `json:"difficulty"` // easy | medium | hard
	StepOrder   int       `json:"step_order"`
	IsBuiltin   bool      `json:"is_builtin"`
	CreatedAt   time.Time `json:"created_at"`
}

// CreateMeasureInput holds validated input for creating a control measure.
type CreateMeasureInput struct {
	Title       string `json:"title"       validate:"required,min=3,max=200"`
	Description string `json:"description" validate:"max=2000"`
	Difficulty  string `json:"difficulty"  validate:"required,oneof=easy medium hard"`
	StepOrder   int    `json:"step_order"`
}

// UpdateMeasureInput holds validated input for updating a control measure.
type UpdateMeasureInput struct {
	Title       *string `json:"title"       validate:"omitempty,min=3,max=200"`
	Description *string `json:"description" validate:"omitempty,max=2000"`
	Difficulty  *string `json:"difficulty"  validate:"omitempty,oneof=easy medium hard"`
	StepOrder   *int    `json:"step_order"`
}

// --- CAPA (Corrective and Preventive Actions) ---

// CAPA represents a corrective or preventive action record.
type CAPA struct {
	ID               string     `json:"id"`
	OrgID            string     `json:"org_id"`
	SourceType       string     `json:"source_type"`
	SourceID         string     `json:"source_id"`
	Title            string     `json:"title"`
	Description      string     `json:"description"`
	RootCause        string     `json:"root_cause"`
	ActionPlan       string     `json:"action_plan"`
	AssigneeEmail    string     `json:"assignee_email"`
	DueDate          *time.Time `json:"due_date"`
	Priority         string     `json:"priority"`
	Status           string     `json:"status"`
	VerificationNote string     `json:"verification_note"`
	ClosedAt         *time.Time `json:"closed_at"`
	CreatedAt        time.Time  `json:"created_at"`
	UpdatedAt        time.Time  `json:"updated_at"`
}

// CreateCAPAInput holds validated input for creating a CAPA.
type CreateCAPAInput struct {
	SourceType    string  `json:"source_type"    validate:"required,oneof=audit incident risk manual"`
	SourceID      string  `json:"source_id"`
	Title         string  `json:"title"          validate:"required,min=3,max=300"`
	Description   string  `json:"description"    validate:"max=3000"`
	AssigneeEmail string  `json:"assignee_email" validate:"omitempty,email"`
	DueDate       *string `json:"due_date"`
	Priority      string  `json:"priority"       validate:"omitempty,oneof=low medium high critical"`
}

// UpdateCAPAInput holds validated input for updating a CAPA.
type UpdateCAPAInput struct {
	Title            *string `json:"title"             validate:"omitempty,min=3,max=300"`
	Description      *string `json:"description"       validate:"omitempty,max=3000"`
	RootCause        *string `json:"root_cause"        validate:"omitempty,max=3000"`
	ActionPlan       *string `json:"action_plan"       validate:"omitempty,max=5000"`
	AssigneeEmail    *string `json:"assignee_email"    validate:"omitempty,email"`
	DueDate          *string `json:"due_date"`
	Priority         *string `json:"priority"          validate:"omitempty,oneof=low medium high critical"`
	Status           *string `json:"status"            validate:"omitempty,oneof=open in_progress implemented verified closed"`
	VerificationNote *string `json:"verification_note" validate:"omitempty,max=3000"`
}

// --- Collaborative Tasks & Comments ---

// Task is an assignable work item attached to any compliance entity.
type Task struct {
	ID            string     `json:"id"`
	OrgID         string     `json:"org_id"`
	EntityType    string     `json:"entity_type"`
	EntityID      string     `json:"entity_id"`
	Title         string     `json:"title"`
	Description   string     `json:"description"`
	AssigneeEmail string     `json:"assignee_email"`
	DueDate       *time.Time `json:"due_date"`
	Status        string     `json:"status"`
	Priority      string     `json:"priority"`
	CreatedBy     string     `json:"created_by"`
	CreatedAt     time.Time  `json:"created_at"`
	UpdatedAt     time.Time  `json:"updated_at"`
}

// CreateTaskInput holds validated input for creating a collaborative task.
type CreateTaskInput struct {
	Title         string  `json:"title"          validate:"required,min=2,max=200"`
	Description   string  `json:"description"    validate:"max=2000"`
	AssigneeEmail string  `json:"assignee_email" validate:"omitempty,email"`
	DueDate       *string `json:"due_date"`
	Status        string  `json:"status"         validate:"omitempty,oneof=open in_progress done"`
	Priority      string  `json:"priority"       validate:"omitempty,oneof=low medium high critical"`
}

// UpdateTaskInput holds validated input for patching a collaborative task.
type UpdateTaskInput struct {
	Title         *string `json:"title"          validate:"omitempty,min=2,max=200"`
	Description   *string `json:"description"    validate:"omitempty,max=2000"`
	AssigneeEmail *string `json:"assignee_email" validate:"omitempty,email"`
	DueDate       *string `json:"due_date"`
	Status        *string `json:"status"         validate:"omitempty,oneof=open in_progress done"`
	Priority      *string `json:"priority"       validate:"omitempty,oneof=low medium high critical"`
}

// Comment is a threaded comment attached to any compliance entity.
type Comment struct {
	ID          string    `json:"id"`
	OrgID       string    `json:"org_id"`
	EntityType  string    `json:"entity_type"`
	EntityID    string    `json:"entity_id"`
	AuthorEmail string    `json:"author_email"`
	Body        string    `json:"body"`
	CreatedAt   time.Time `json:"created_at"`
}

// CreateCommentInput holds validated input for posting a comment.
type CreateCommentInput struct {
	Body        string `json:"body"         validate:"required,min=1,max=5000"`
	AuthorEmail string `json:"author_email" validate:"omitempty,email"`
}

// --- Evidence Files (Migration 074) ---

// EvidenceFile represents an uploaded document attached to a compliance evidence record.
type EvidenceFile struct {
	ID           string    `json:"id"`
	OrgID        string    `json:"org_id"`
	EvidenceID   string    `json:"evidence_id"`
	ControlID    string    `json:"control_id"`
	OriginalName string    `json:"original_name"`
	StoredName   string    `json:"stored_name"`
	MimeType     string    `json:"mime_type"`
	SizeBytes    int64     `json:"size_bytes"`
	UploadedBy   string    `json:"uploaded_by"`
	CreatedAt    time.Time `json:"created_at"`
	DownloadURL  string    `json:"download_url"` // computed, not stored
}
