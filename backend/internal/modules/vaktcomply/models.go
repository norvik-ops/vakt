// Package vaktcomply provides domain models for compliance automation (NIS2, ISO 27001, BSI-Grundschutz).
package vaktcomply

import (
	"encoding/json"
	"time"
)

// --- Risk Assessment (FR-CK12) ---

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
	// NIS2 Art.23 stage-based reporting workflow (Migration 175)
	NIS2Reportable              *bool      `json:"nis2_reportable,omitempty"`
	NIS2ReportingStage          *string    `json:"nis2_reporting_stage,omitempty"`
	NIS2DetectedAt              *time.Time `json:"nis2_detected_at,omitempty"`
	NIS2EarlyWarningDue         *time.Time `json:"nis2_early_warning_due,omitempty"`
	NIS2FullReportDue           *time.Time `json:"nis2_full_report_due,omitempty"`
	NIS2FinalReportDue          *time.Time `json:"nis2_final_report_due,omitempty"`
	NIS2EarlyWarningSubmittedAt *time.Time `json:"nis2_early_warning_submitted_at,omitempty"`
	NIS2FullReportSubmittedAt   *time.Time `json:"nis2_full_report_submitted_at,omitempty"`
	NIS2FinalReportSubmittedAt  *time.Time `json:"nis2_final_report_submitted_at,omitempty"`
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

// AuditRecord and its input types now live in the audit sub-package.

// --- Update inputs ---

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

// UpdateAuditRecordInput now lives in the audit sub-package.

// --- Control Implementation Tasks ---

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
	// ISO 27001 A.5.19-21 fields (Migration 176 / S67-2)
	Category               string     `json:"category,omitempty"`
	DataAccess             bool       `json:"data_access"`
	AvvDocumentID          *string    `json:"avv_document_id,omitempty"`
	LastAssessmentScore    *int       `json:"last_assessment_score,omitempty"`
	NextAssessmentDue      *time.Time `json:"next_assessment_due,omitempty"`
	SupplierStatus         string     `json:"supplier_status,omitempty"` // active | inactive | terminated
	ContractStart          *time.Time `json:"contract_start,omitempty"`
	DataProtectionScore    *int       `json:"data_protection_score,omitempty"`
	AvailabilityScore      *int       `json:"availability_score,omitempty"`
	SecurityCertifications string     `json:"security_certifications,omitempty"`
	AuditRights            *bool      `json:"audit_rights,omitempty"`
	SubProcessorsKnown     *bool      `json:"sub_processors_known,omitempty"`
	IncidentNotification   *bool      `json:"incident_notification,omitempty"`
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
	// ISO 27001 A.5.19-21 fields (Migration 176 / S67-2)
	Category               string     `json:"category"               validate:"omitempty,oneof=software cloud hardware service telecom other"`
	DataAccess             bool       `json:"data_access"`
	AvvDocumentID          *string    `json:"avv_document_id,omitempty"`
	LastAssessmentScore    *int       `json:"last_assessment_score"  validate:"omitempty,min=1,max=5"`
	NextAssessmentDue      *time.Time `json:"next_assessment_due"`
	SupplierStatus         string     `json:"supplier_status"        validate:"omitempty,oneof=active inactive terminated"`
	ContractStart          *time.Time `json:"contract_start"`
	DataProtectionScore    *int       `json:"data_protection_score"  validate:"omitempty,min=1,max=5"`
	AvailabilityScore      *int       `json:"availability_score"     validate:"omitempty,min=1,max=5"`
	SecurityCertifications string     `json:"security_certifications"`
	AuditRights            *bool      `json:"audit_rights"`
	SubProcessorsKnown     *bool      `json:"sub_processors_known"`
	IncidentNotification   *bool      `json:"incident_notification"`
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
	// ISO 27001 A.5.19-21 fields (Migration 176 / S67-2)
	Category               string     `json:"category"               validate:"omitempty,oneof=software cloud hardware service telecom other"`
	DataAccess             bool       `json:"data_access"`
	AvvDocumentID          *string    `json:"avv_document_id,omitempty"`
	LastAssessmentScore    *int       `json:"last_assessment_score"  validate:"omitempty,min=1,max=5"`
	NextAssessmentDue      *time.Time `json:"next_assessment_due"`
	SupplierStatus         string     `json:"supplier_status"        validate:"omitempty,oneof=active inactive terminated"`
	ContractStart          *time.Time `json:"contract_start"`
	DataProtectionScore    *int       `json:"data_protection_score"  validate:"omitempty,min=1,max=5"`
	AvailabilityScore      *int       `json:"availability_score"     validate:"omitempty,min=1,max=5"`
	SecurityCertifications string     `json:"security_certifications"`
	AuditRights            *bool      `json:"audit_rights"`
	SubProcessorsKnown     *bool      `json:"sub_processors_known"`
	IncidentNotification   *bool      `json:"incident_notification"`
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

// --- Maßnahmen-Katalog (control measures) ---

// --- CAPA (Corrective and Preventive Actions) ---

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

// --- DORA IKT-Drittanbieter-Register (Art. 28-44 / S38-1) ---

// --- S39-1: BSI-Meldepflicht-Klassifizierung ---

// ── S60: Schutzbedarfsfeststellung ────────────────────────────────────────────

// ── S61-1: ISMS Scope ──

// ISMSScope represents a versioned ISMS scope document.
type ISMSScope struct {
	ID                      string          `json:"id"`
	OrgID                   string          `json:"org_id"`
	Version                 int             `json:"version"`
	Status                  string          `json:"status"`
	ScopeDefinition         string          `json:"scope_definition"`
	Exclusions              json.RawMessage `json:"exclusions"`
	OutsourcingDependencies string          `json:"outsourcing_dependencies"`
	ChangeNote              string          `json:"change_note"`
	ApprovedBy              *string         `json:"approved_by,omitempty"`
	ApprovedAt              *time.Time      `json:"approved_at,omitempty"`
	CreatedBy               string          `json:"created_by"`
	CreatedAt               time.Time       `json:"created_at"`
	UpdatedAt               time.Time       `json:"updated_at"`
}

// CreateISMSScopeInput holds the request body for creating or versioning an ISMS scope.
type CreateISMSScopeInput struct {
	ScopeDefinition         string          `json:"scope_definition"`
	Exclusions              json.RawMessage `json:"exclusions"`
	OutsourcingDependencies string          `json:"outsourcing_dependencies"`
	ChangeNote              string          `json:"change_note"`
}

// ApproveISMSScopeInput holds the id for an approval action.
type ApproveISMSScopeInput struct {
	ID string `json:"-"`
}

// ── S61-3: NC/CA Root Cause + Wirksamkeitsprüfung ──

// ── S61-6: Pentest Tracking ──

// Pentest represents a penetration test record for an organisation.
type Pentest struct {
	ID               string    `json:"id"`
	OrgID            string    `json:"org_id"`
	Title            string    `json:"title"`
	Scope            string    `json:"scope"`
	PentestDate      string    `json:"pentest_date"`
	TesterType       string    `json:"tester_type"`
	TesterName       string    `json:"tester_name"`
	Methodology      *string   `json:"methodology,omitempty"`
	FindingsCritical int       `json:"findings_critical"`
	FindingsHigh     int       `json:"findings_high"`
	FindingsMedium   int       `json:"findings_medium"`
	FindingsLow      int       `json:"findings_low"`
	Status           string    `json:"status"`
	RetestDate       *string   `json:"retest_date,omitempty"`
	Notes            string    `json:"notes"`
	CreatedBy        string    `json:"created_by"`
	CreatedAt        time.Time `json:"created_at"`
	UpdatedAt        time.Time `json:"updated_at"`
}

// CreatePentestInput is the request body for creating a new pentest record.
type CreatePentestInput struct {
	Title            string  `json:"title"        validate:"required"`
	Scope            string  `json:"scope"`
	PentestDate      string  `json:"pentest_date" validate:"required"`
	TesterType       string  `json:"tester_type"  validate:"required,oneof=internal external"`
	TesterName       string  `json:"tester_name"`
	Methodology      *string `json:"methodology,omitempty" validate:"omitempty,oneof=blackbox greybox whitebox"`
	FindingsCritical int     `json:"findings_critical"`
	FindingsHigh     int     `json:"findings_high"`
	FindingsMedium   int     `json:"findings_medium"`
	FindingsLow      int     `json:"findings_low"`
	Notes            string  `json:"notes"`
}

// UpdatePentestInput is the request body for updating an existing pentest record.
type UpdatePentestInput struct {
	Title            string  `json:"title"        validate:"required"`
	Scope            string  `json:"scope"`
	TesterType       string  `json:"tester_type"  validate:"required,oneof=internal external"`
	TesterName       string  `json:"tester_name"`
	Methodology      *string `json:"methodology,omitempty" validate:"omitempty,oneof=blackbox greybox whitebox"`
	FindingsCritical int     `json:"findings_critical"`
	FindingsHigh     int     `json:"findings_high"`
	FindingsMedium   int     `json:"findings_medium"`
	FindingsLow      int     `json:"findings_low"`
	Status           string  `json:"status"       validate:"required,oneof=in_progress completed remediation closed"`
	RetestDate       *string `json:"retest_date,omitempty"`
	Notes            string  `json:"notes"`
}

// ── S61-2: Management Review ──────────────────────────────────────────────────

// ImprovementDecision is a single decision item within a management review output.
type ImprovementDecision struct {
	Decision    string `json:"decision"`
	Responsible string `json:"responsible"`
	DueDate     string `json:"due_date"`
}

// ManagementReview and its input types now live in the audit sub-package.

// ── S61-7: ISMS KPI Dashboard ─────────────────────────────────────────────────

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

// AuditorLink represents a time-limited read-only access token for external auditors.
type AuditorLink struct {
	ID                string    `json:"id"`
	OrgID             string    `json:"org_id"`
	FrameworkID       string    `json:"framework_id"`
	TokenHash         string    `json:"-"` // never exposed
	CreatedBy         string    `json:"created_by"`
	ExpiresAt         time.Time `json:"expires_at"`
	UsedCount         int       `json:"used_count"`
	MaxUses           *int      `json:"max_uses,omitempty"`
	Description       string    `json:"description,omitempty"`        // S67-5
	AllowedFrameworks []string  `json:"allowed_frameworks,omitempty"` // S67-5
	CreatedAt         time.Time `json:"created_at"`
	// ShareURL is populated on creation with the raw token embedded.
	ShareURL string `json:"share_url,omitempty"`
}

// AuditorLinkListItem is the response shape for listing auditor links (E09.1).
type AuditorLinkListItem struct {
	ID                string     `json:"id"`
	OrgID             string     `json:"org_id"`
	FrameworkID       string     `json:"framework_id"`
	Label             string     `json:"label"`
	Description       string     `json:"description,omitempty"`        // S67-5
	AllowedFrameworks []string   `json:"allowed_frameworks,omitempty"` // S67-5
	CreatedBy         string     `json:"created_by"`
	ExpiresAt         time.Time  `json:"expires_at"`
	LastAccessedAt    *time.Time `json:"last_accessed_at,omitempty"`
	AccessCount       int        `json:"access_count"`
	RevokedAt         *time.Time `json:"revoked_at,omitempty"`
	CreatedAt         time.Time  `json:"created_at"`
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

// --- DORA Dashboard (Story 27.5) ---

// DORADashboard holds the computed DORA readiness summary for the dashboard.
type DORADashboard struct {
	ReadinessPct         float64       `json:"readiness_pct"`
	OpenCriticalControls int           `json:"open_critical_controls"`
	NextDeadline         *NextDeadline `json:"next_deadline,omitempty"`
	ExpiredSuppliers     int           `json:"expired_suppliers"`
	TLPTOverdueWarning   bool          `json:"tlpt_overdue_warning"`
	// IKT-Drittanbieter (S38-1/2/3)
	ThirdPartyCount       int `json:"third_party_count"`
	CriticalThirdParties  int `json:"critical_third_parties"`
	MissingExitStrategies int `json:"missing_exit_strategies"`
	// TLPT summary (S40-1) — last 3 TLPT tests for PDF
	RecentResilienceTests []ResilienceTest `json:"recent_resilience_tests,omitempty"`
}

// NextDeadline holds the nearest unreported DORA deadline.
type NextDeadline struct {
	IncidentID   string    `json:"incident_id"`
	Title        string    `json:"title"`
	DeadlineType string    `json:"deadline_type"` // "4h" | "24h" | "72h" | "30d"
	DeadlineAt   time.Time `json:"deadline_at"`
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

type ClassifyReportingInput struct {
	EssentialService bool `json:"essential_service"`
	CustomerData     bool `json:"customer_data"`
	PersonalData     bool `json:"personal_data"`
}

type ClassificationResult struct {
	Obligation string `json:"obligation"` // "probably" | "none" | "unclear"
	Authority  string `json:"authority"`  // "BSI" | "BaFin+BSI" | "LDA" | ""
	Reason     string `json:"reason"`
}
