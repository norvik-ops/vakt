// Package secprivacy provides DSGVO documentation: VVT, DPIA, AVV, breach notifications.
package secprivacy

import "time"

// VVTEntry represents one entry in the Verzeichnis von Verarbeitungstätigkeiten (Art. 30 DSGVO).
type VVTEntry struct {
	ID                    string    `json:"id"`
	OrgID                 string    `json:"org_id"`
	Name                  string    `json:"name"`
	Purpose               string    `json:"purpose"`
	LegalBasis            string    `json:"legal_basis"`
	DataCategories        []string  `json:"data_categories"`
	DataSubjects          []string  `json:"data_subjects"`
	Recipients            []string  `json:"recipients"`
	RetentionPeriod       string    `json:"retention_period,omitempty"`
	ThirdCountryTransfer  bool      `json:"third_country_transfer"`
	Safeguards            string    `json:"safeguards,omitempty"`
	ResponsiblePerson     string    `json:"responsible_person,omitempty"`
	Status                string    `json:"status"` // active | archived
	CreatedAt             time.Time `json:"created_at"`
	UpdatedAt             time.Time `json:"updated_at"`
}

// CreateVVTInput holds validated input for creating a VVT entry.
type CreateVVTInput struct {
	Name                 string   `json:"name"         validate:"required,max=255"`
	Purpose              string   `json:"purpose"      validate:"required,max=10000"`
	LegalBasis           string   `json:"legal_basis"  validate:"required"`
	DataCategories       []string `json:"data_categories"`
	DataSubjects         []string `json:"data_subjects"`
	Recipients           []string `json:"recipients"`
	RetentionPeriod      string   `json:"retention_period"`
	ThirdCountryTransfer bool     `json:"third_country_transfer"`
	Safeguards           string   `json:"safeguards"   validate:"max=10000"`
	ResponsiblePerson    string   `json:"responsible_person"`
}

// DPIA represents a Datenschutz-Folgenabschätzung (Art. 35 DSGVO).
type DPIA struct {
	ID                   string     `json:"id"`
	OrgID                string     `json:"org_id"`
	VVTEntryID           *string    `json:"vvt_entry_id,omitempty"`
	Title                string     `json:"title"`
	Description          string     `json:"description,omitempty"`
	NecessityAssessment  string     `json:"necessity_assessment,omitempty"`
	RiskAssessment       string     `json:"risk_assessment,omitempty"`
	MitigationMeasures   string     `json:"mitigation_measures,omitempty"`
	ResidualRisk         string     `json:"residual_risk,omitempty"`
	DPOConsultation      bool       `json:"dpo_consultation"`
	Status               string     `json:"status"` // draft | in_review | approved
	ReviewedBy           *string    `json:"reviewed_by,omitempty"`
	ReviewedAt           *time.Time `json:"reviewed_at,omitempty"`
	CreatedAt            time.Time  `json:"created_at"`
	UpdatedAt            time.Time  `json:"updated_at"`
}

// CreateDPIAInput holds validated input for creating a DPIA.
type CreateDPIAInput struct {
	VVTEntryID          *string `json:"vvt_entry_id"`
	Title               string  `json:"title"                validate:"required,max=255"`
	Description         string  `json:"description"          validate:"max=10000"`
	NecessityAssessment string  `json:"necessity_assessment" validate:"max=10000"`
	RiskAssessment      string  `json:"risk_assessment"      validate:"max=10000"`
	MitigationMeasures  string  `json:"mitigation_measures"  validate:"max=10000"`
	ResidualRisk        string  `json:"residual_risk"        validate:"max=10000"`
	DPOConsultation     bool    `json:"dpo_consultation"`
}

// AVV represents an Auftragsverarbeitungsvertrag (Art. 28 DSGVO).
type AVV struct {
	ID                 string     `json:"id"`
	OrgID              string     `json:"org_id"`
	ProcessorName      string     `json:"processor_name"`
	ServiceDescription string     `json:"service_description"`
	ContractDate       *time.Time `json:"contract_date,omitempty"`
	ReviewDate         *time.Time `json:"review_date,omitempty"`
	Status             string     `json:"status"` // active | expired | terminated
	Notes              string     `json:"notes,omitempty"`
	TemplateID         string     `json:"template_id,omitempty"`
	Body               string     `json:"body,omitempty"`
	SCCModule          string     `json:"scc_module,omitempty"`
	SCCAnnexI          string     `json:"scc_annex_i,omitempty"`
	SCCAnnexII         string     `json:"scc_annex_ii,omitempty"`
	SCCAnnexIII        string     `json:"scc_annex_iii,omitempty"`
	CreatedAt          time.Time  `json:"created_at"`
	UpdatedAt          time.Time  `json:"updated_at"`
}

// CreateAVVInput holds validated input for creating an AVV.
type CreateAVVInput struct {
	ProcessorName      string     `json:"processor_name"      validate:"required,max=255"`
	ServiceDescription string     `json:"service_description" validate:"required,max=10000"`
	ContractDate       *time.Time `json:"contract_date"`
	ReviewDate         *time.Time `json:"review_date"`
	Notes              string     `json:"notes"               validate:"max=10000"`
}

// Breach represents a data breach notification record (Art. 33/34 DSGVO).
type Breach struct {
	ID                            string     `json:"id"`
	OrgID                         string     `json:"org_id"`
	Title                         string     `json:"title"`
	Description                   string     `json:"description"`
	DiscoveredAt                  time.Time  `json:"discovered_at"`
	AuthorityDeadlineAt           time.Time  `json:"authority_deadline_at"`
	AuthorityNotifiedAt           *time.Time `json:"authority_notified_at,omitempty"`
	SubjectsNotificationRequired  bool       `json:"subjects_notification_required"`
	SubjectsNotifiedAt            *time.Time `json:"subjects_notified_at,omitempty"`
	AffectedCount                 *int       `json:"affected_count,omitempty"`
	DataCategories                []string   `json:"data_categories"`
	Status                        string     `json:"status"` // open | authority_notified | closed
	CreatedAt                     time.Time  `json:"created_at"`
	UpdatedAt                     time.Time  `json:"updated_at"`
}

// CreateBreachInput holds validated input for creating a breach record.
type CreateBreachInput struct {
	Title                        string     `json:"title"          validate:"required,max=255"`
	Description                  string     `json:"description"    validate:"required,max=10000"`
	DiscoveredAt                 time.Time  `json:"discovered_at"  validate:"required"`
	SubjectsNotificationRequired bool       `json:"subjects_notification_required"`
	AffectedCount                *int       `json:"affected_count"`
	DataCategories               []string   `json:"data_categories"`
}

// UpdateVVTInput holds validated input for updating a VVT entry.
type UpdateVVTInput struct {
	Name                 string   `json:"name"          validate:"required,max=255"`
	Purpose              string   `json:"purpose"       validate:"required,max=10000"`
	LegalBasis           string   `json:"legal_basis"   validate:"required"`
	DataCategories       []string `json:"data_categories"`
	DataSubjects         []string `json:"data_subjects"`
	Recipients           []string `json:"recipients"`
	RetentionPeriod      string   `json:"retention_period"`
	ThirdCountryTransfer bool     `json:"third_country_transfer"`
	Safeguards           string   `json:"safeguards"    validate:"max=10000"`
	ResponsiblePerson    string   `json:"responsible_person"`
	Status               string   `json:"status"        validate:"required,oneof=active archived"`
}

// UpdateDPIAInput holds validated input for updating a DPIA.
type UpdateDPIAInput struct {
	Title               string `json:"title"                validate:"required,max=255"`
	Description         string `json:"description"          validate:"max=10000"`
	NecessityAssessment string `json:"necessity_assessment" validate:"max=10000"`
	RiskAssessment      string `json:"risk_assessment"      validate:"max=10000"`
	MitigationMeasures  string `json:"mitigation_measures"  validate:"max=10000"`
	ResidualRisk        string `json:"residual_risk"        validate:"max=10000"`
	DPOConsultation     bool   `json:"dpo_consultation"`
}

// UpdateAVVInput holds validated input for updating an AVV.
type UpdateAVVInput struct {
	ProcessorName      string     `json:"processor_name"      validate:"required,max=255"`
	ServiceDescription string     `json:"service_description" validate:"required,max=10000"`
	ContractDate       *time.Time `json:"contract_date"`
	ReviewDate         *time.Time `json:"review_date"`
	Status             string     `json:"status"              validate:"required,oneof=active expired terminated"`
	Notes              string     `json:"notes"               validate:"max=10000"`
}

// CreateAVVFromTemplateInput holds input for creating an AVV from a built-in template.
type CreateAVVFromTemplateInput struct {
	TemplateID string            `json:"template_id" validate:"required"`
	Vars       map[string]string `json:"vars"`
}

// UpdateAVVSCCInput holds validated input for attaching EU Standard Contractual Clauses to an AVV.
type UpdateAVVSCCInput struct {
	SCCModule string `json:"scc_module" validate:"omitempty,oneof=module_1 module_2 module_3 module_4"`
	AnnexI    string `json:"annex_i"    validate:"max=10000"`
	AnnexII   string `json:"annex_ii"   validate:"max=10000"`
	AnnexIII  string `json:"annex_iii"  validate:"max=10000"`
}

// UpdateBreachInput holds validated input for updating a breach record.
type UpdateBreachInput struct {
	Title                        string   `json:"title"       validate:"required,max=255"`
	Description                  string   `json:"description" validate:"required,max=10000"`
	SubjectsNotificationRequired bool     `json:"subjects_notification_required"`
	AffectedCount                *int     `json:"affected_count"`
	DataCategories               []string `json:"data_categories"`
}

// DSR represents a data subject request (Betroffenenanfrage) under Art. 15-21 DSGVO.
// The controller must respond within 30 days of receipt (Art. 12 Abs. 3 DSGVO);
// due_date is set automatically at creation to enforce that deadline.
type DSR struct {
	// ID is the unique UUID of the DSR record.
	ID string `json:"id"`
	// OrgID scopes the record to a single tenant organisation.
	OrgID string `json:"org_id"`
	// RequesterName is the full name of the data subject who submitted the request.
	RequesterName string `json:"requester_name"`
	// RequesterEmail is the contact address for sending the controller's response.
	RequesterEmail string `json:"requester_email"`
	// Type classifies the legal basis of the request.
	// Allowed values: access (Art. 15), erasure (Art. 17), portability (Art. 20),
	// objection (Art. 21), rectification (Art. 16).
	Type string `json:"type"`
	// Description contains the requester's free-text explanation of the request (optional).
	Description string `json:"description,omitempty"`
	// Status tracks the processing lifecycle.
	// Allowed values: open | in_progress | completed | rejected.
	Status string `json:"status"`
	// DueDate is the YYYY-MM-DD response deadline, always 30 calendar days after
	// ReceivedAt as required by Art. 12 Abs. 3 DSGVO. Set by the repository on insert.
	DueDate *string `json:"due_date,omitempty"`
	// ReceivedAt records when the request was received; the 30-day Art. 12 clock starts here.
	ReceivedAt time.Time `json:"received_at"`
	// CompletedAt is stamped when Status transitions to "completed" or "rejected".
	CompletedAt *time.Time `json:"completed_at,omitempty"`
	// Notes holds internal handling notes, not shared with the requester.
	Notes string `json:"notes,omitempty"`
	// CreatedAt is the database insert timestamp.
	CreatedAt time.Time `json:"created_at"`
	// UpdatedAt is the timestamp of the most recent modification.
	UpdatedAt time.Time `json:"updated_at"`
}

// CreateDSRInput holds validated input for creating a DSR.
// The repository automatically computes due_date = now + 30 days (Art. 12 DSGVO);
// callers must not supply it.
type CreateDSRInput struct {
	// RequesterName is the full name of the data subject; required for identity verification.
	RequesterName string `json:"requester_name"  validate:"required"`
	// RequesterEmail must be a valid address; it is the channel for the official response.
	RequesterEmail string `json:"requester_email" validate:"required,email"`
	// Type selects the DSGVO right being exercised.
	// Must be one of: access, erasure, portability, objection, rectification.
	Type        string `json:"type"            validate:"required,oneof=access erasure portability objection rectification"`
	Description string `json:"description,omitempty" validate:"max=10000"`
	Notes       string `json:"notes,omitempty"       validate:"max=10000"`
}

// UpdateDSRInput holds validated input for updating a DSR.
// Transitioning to "completed" or "rejected" automatically stamps CompletedAt
// in the repository so the response timeline is preserved for audit purposes.
type UpdateDSRInput struct {
	// Status is the new processing state.
	// Allowed values: open | in_progress | completed | rejected.
	Status string `json:"status" validate:"required,oneof=open in_progress completed rejected"`
	Notes  string `json:"notes,omitempty" validate:"max=10000"`
}

// PortalDSRInput is the payload submitted by a data subject via the public DSR portal.
type PortalDSRInput struct {
	Type        string `json:"type"        validate:"required,oneof=access deletion correction objection"`
	FirstName   string `json:"first_name"  validate:"required"`
	LastName    string `json:"last_name"   validate:"required"`
	Email       string `json:"email"       validate:"required,email"`
	Description string `json:"description" validate:"max=10000"`
	Locale      string `json:"locale"`
}

// DSRPortalInfo is the public-facing metadata returned for a given portal slug.
type DSRPortalInfo struct {
	OrgName string `json:"org_name"`
	Slug    string `json:"slug"`
	Intro   string `json:"intro,omitempty"`
	Enabled bool   `json:"enabled"`
}

// UpdateDSRPortalSettingsInput holds validated input for configuring the DSR self-service portal.
type UpdateDSRPortalSettingsInput struct {
	Enabled  bool   `json:"enabled"`
	Slug     string `json:"slug"`
	DPOEmail string `json:"dpo_email"`
	Intro    string `json:"intro"`
}
