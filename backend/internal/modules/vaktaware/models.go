// Package vaktaware provides domain models for phishing simulation and security awareness training.
package vaktaware

import (
	"time"
)

// Template is an email template for phishing simulations.
type Template struct {
	ID           string    `json:"id"`
	OrgID        string    `json:"org_id"`
	Name         string    `json:"name"`
	Subject      string    `json:"subject"`
	FromName     string    `json:"from_name"`
	FromEmail    string    `json:"from_email"`
	HTMLBody     string    `json:"html_body"`
	AttackType   string    `json:"attack_type"`
	Category     string    `json:"category,omitempty"`
	Difficulty   string    `json:"difficulty,omitempty"`
	Language     string    `json:"language,omitempty"`
	Placeholders []string  `json:"placeholders,omitempty"`
	IsPreset     bool      `json:"is_preset"`
	CreatedBy    *string   `json:"created_by,omitempty"`
	CreatedAt    time.Time `json:"created_at"`
}

// EnrollmentRule defines automatic campaign enrollment based on triggers.
type EnrollmentRule struct {
	ID               string    `json:"id"`
	OrgID            string    `json:"org_id"`
	Name             string    `json:"name"`
	TriggerType      string    `json:"trigger_type"` // new_employee | phishing_click
	TargetCampaignID *string   `json:"target_campaign_id,omitempty"`
	IsActive         bool      `json:"is_active"`
	CreatedAt        time.Time `json:"created_at"`
	UpdatedAt        time.Time `json:"updated_at"`
}

// CreateEnrollmentRuleInput holds user-supplied data for creating an enrollment rule.
type CreateEnrollmentRuleInput struct {
	Name             string  `json:"name"              validate:"required"`
	TriggerType      string  `json:"trigger_type"      validate:"required,oneof=new_employee phishing_click"`
	TargetCampaignID *string `json:"target_campaign_id"`
}

// TrainingMatrixReport is the top-level structure for an audit-ready training report.
type TrainingMatrixReport struct {
	Period        ReportPeriod      `json:"period"`
	OrgName       string            `json:"org_name"`
	Campaigns     []CampaignSummary `json:"campaigns"`
	TotalStats    AwareStats        `json:"total_stats"`
	BSICompliance BSIOrp3Compliance `json:"bsi_compliance"`
	GeneratedAt   time.Time         `json:"generated_at"`
}

// ReportPeriod defines the date range of a training report.
type ReportPeriod struct {
	From time.Time `json:"from"`
	To   time.Time `json:"to"`
}

// CampaignSummary summarises a single campaign for a training report.
type CampaignSummary struct {
	ID             string  `json:"id"`
	Name           string  `json:"name"`
	Type           string  `json:"type"`
	RecipientCount int     `json:"recipient_count"`
	ClickRate      float64 `json:"click_rate"`
	// completion_rate was removed (S131-G2/R-M02): a phishing campaign has no schema
	// path to training completions (sr_assignments key module→target, not campaign), so
	// the field was permanently 0 — a misleading "0% completed" in the audit report.
	StartedAt   string `json:"started_at,omitempty"`
	CompletedAt string `json:"completed_at,omitempty"`
}

// AwareStats aggregates awareness training KPIs for the reporting period.
type AwareStats struct {
	TotalCampaigns          int     `json:"total_campaigns"`
	TotalParticipants       int     `json:"total_participants"`
	AvgClickRate            float64 `json:"avg_click_rate"`
	TotalTrainingsCompleted int     `json:"total_trainings_completed"`
}

// BSIOrp3Compliance tracks coverage of BSI ORP.3 requirements.
type BSIOrp3Compliance struct {
	FulfilledCount int               `json:"fulfilled_count"`
	TotalCount     int               `json:"total_count"`
	Requirements   []ORP3Requirement `json:"requirements"`
}

// ORP3Requirement represents a single BSI ORP.3 requirement and its fulfillment state.
type ORP3Requirement struct {
	ID          string   `json:"id"`
	Title       string   `json:"title"`
	Fulfilled   bool     `json:"fulfilled"`
	EvidenceIDs []string `json:"evidence_ids,omitempty"`
}

// CreateTemplateInput holds user-supplied data for creating a template.
type CreateTemplateInput struct {
	Name       string `json:"name"        validate:"required"`
	Subject    string `json:"subject"     validate:"required"`
	FromName   string `json:"from_name"   validate:"required"`
	FromEmail  string `json:"from_email"  validate:"required,email"`
	HTMLBody   string `json:"html_body"   validate:"required"`
	AttackType string `json:"attack_type" validate:"required,oneof=phishing vishing usb smishing"`
}

// TargetGroup is a named collection of phishing targets.
type TargetGroup struct {
	ID        string    `json:"id"`
	OrgID     string    `json:"org_id"`
	Name      string    `json:"name"`
	Source    string    `json:"source"`
	ADOU      *string   `json:"ad_ou,omitempty"`
	CreatedAt time.Time `json:"created_at"`
}

// Target is an individual recipient within a target group.
type Target struct {
	ID         string    `json:"id"`
	OrgID      string    `json:"org_id"`
	GroupID    string    `json:"group_id"`
	Email      string    `json:"email"`
	FirstName  string    `json:"first_name"`
	LastName   string    `json:"last_name"`
	Department string    `json:"department"`
	IsBounced  bool      `json:"is_bounced"`
	CreatedAt  time.Time `json:"created_at"`
}

// LandingPage is a capture page shown after a simulated phishing click.
type LandingPage struct {
	ID          string    `json:"id"`
	OrgID       string    `json:"org_id"`
	Name        string    `json:"name"`
	HTMLContent string    `json:"html_content"`
	CreatedAt   time.Time `json:"created_at"`
}

// Campaign represents a phishing simulation campaign.
type Campaign struct {
	ID              string     `json:"id"`
	OrgID           string     `json:"org_id"`
	Name            string     `json:"name"`
	Status          string     `json:"status"`
	TemplateID      *string    `json:"template_id,omitempty"`
	GroupID         *string    `json:"group_id,omitempty"`
	LandingPageID   *string    `json:"landing_page_id,omitempty"`
	FromName        string     `json:"from_name"`
	FromEmail       string     `json:"from_email"`
	Subject         string     `json:"subject"`
	ScheduledAt     *time.Time `json:"scheduled_at,omitempty"`
	StartedAt       *time.Time `json:"started_at,omitempty"`
	CompletedAt     *time.Time `json:"completed_at,omitempty"`
	Recurrence      *string    `json:"recurrence,omitempty"`
	NextRunAt       *time.Time `json:"next_run_at,omitempty"`
	TrackOpens      bool       `json:"track_opens"`
	BetriebsratMode bool       `json:"betriebsrat_mode"`
	CreatedBy       *string    `json:"created_by,omitempty"`
	CreatedAt       time.Time  `json:"created_at"`
	UpdatedAt       time.Time  `json:"updated_at"`
}

// CreateCampaignInput holds user-supplied data for creating a campaign.
type CreateCampaignInput struct {
	Name            string     `json:"name"             validate:"required"`
	TemplateID      *string    `json:"template_id"`
	GroupID         *string    `json:"group_id"`
	LandingPageID   *string    `json:"landing_page_id"`
	FromName        string     `json:"from_name"        validate:"required"`
	FromEmail       string     `json:"from_email"       validate:"required,email"`
	Subject         string     `json:"subject"          validate:"required"`
	ScheduledAt     *time.Time `json:"scheduled_at,omitempty"`
	Recurrence      string     `json:"recurrence"       validate:"omitempty,oneof=none monthly quarterly"`
	TrackOpens      bool       `json:"track_opens"`
	BetriebsratMode bool       `json:"betriebsrat_mode"`
}

// TrackingEvent records a single open, click, or form-submission interaction.
type TrackingEvent struct {
	ID            string    `json:"id"`
	OrgID         string    `json:"org_id"`
	CampaignID    string    `json:"campaign_id"`
	TargetID      *string   `json:"target_id,omitempty"`
	Department    string    `json:"department,omitempty"`
	Type          string    `json:"type"` // open|click|form_submission
	TrackingToken string    `json:"tracking_token"`
	IPAddress     string    `json:"ip_address,omitempty"`
	UserAgent     string    `json:"user_agent,omitempty"`
	OccurredAt    time.Time `json:"occurred_at"`
}

// CampaignStats aggregates metrics for a campaign.
//
// All three rates are PERCENTAGES (0–100), not fractions, and all three share the
// same denominator: everyone the campaign was aimed at. They can therefore be read
// against each other.
type CampaignStats struct {
	TotalTargets    int     `json:"total_targets"`
	EmailsSent      int     `json:"emails_sent"`
	Opens           int     `json:"opens"`
	Clicks          int     `json:"clicks"`
	FormSubmissions int     `json:"form_submissions"`
	OpenRate        float64 `json:"open_rate"`
	ClickRate       float64 `json:"click_rate"`
	SubmissionRate  float64 `json:"submission_rate"`

	// TrackingMeasured says whether these numbers were measured at all.
	//
	// Campaigns sent before migration 242 carry no `sent` events, because the
	// tracking token was never stored — so every click they received was rejected
	// as an invalid token and none of it was recorded. Their zeroes are not
	// measurements; they are the absence of measurement, and the two look
	// identical in a bar chart.
	//
	// A 0% click rate that nobody explains is a false audit finding: it is
	// indistinguishable from a workforce no phishing mail could fool. A 0% that
	// declares itself unmeasured is an honest gap. This flag is what lets the UI
	// and the Comply KPI tell them apart.
	TrackingMeasured bool `json:"tracking_measured"`
}

// SMTPConfig holds SMTP connection settings for sending campaign emails.
type SMTPConfig struct {
	Host   string
	Port   string
	User   string
	Pass   string
	From   string
	AppURL string // base URL of the deployment, used for absolute tracking links
}

// --- E15: Training models ---

// Question is a single quiz question with multiple-choice options.
type Question struct {
	Text    string   `json:"text"`
	Options []string `json:"options"`
	Answer  int      `json:"answer"` // index of correct answer
}

// TrainingModule is a video or quiz assigned to employees after a phishing event.
type TrainingModule struct {
	ID              string     `json:"id"`
	OrgID           string     `json:"org_id"`
	Title           string     `json:"title"`
	Type            string     `json:"type"`        // video|quiz
	AttackType      string     `json:"attack_type"` // phishing|vishing|usb|smishing
	ContentURL      string     `json:"content_url"`
	DurationSeconds int        `json:"duration_seconds"`
	PassingScore    int        `json:"passing_score"`
	Questions       []Question `json:"questions,omitempty"`
	CreatedBy       *string    `json:"created_by,omitempty"`
	CreatedAt       time.Time  `json:"created_at"`
}

// CreateModuleInput holds user-supplied data for creating a training module.
type CreateModuleInput struct {
	Title           string     `json:"title"            validate:"required"`
	Type            string     `json:"type"             validate:"required,oneof=video quiz"`
	AttackType      string     `json:"attack_type"      validate:"required,oneof=phishing vishing usb smishing"`
	ContentURL      string     `json:"content_url"      validate:"required"`
	DurationSeconds int        `json:"duration_seconds" validate:"min=0"`
	PassingScore    int        `json:"passing_score"    validate:"min=1,max=100"`
	Questions       []Question `json:"questions,omitempty"`
}

// Assignment links a training module to a specific target or department.
type Assignment struct {
	ID         string    `json:"id"`
	OrgID      string    `json:"org_id"`
	ModuleID   string    `json:"module_id"`
	TargetID   *string   `json:"target_id,omitempty"`
	Department string    `json:"department,omitempty"`
	DueDate    time.Time `json:"due_date"`
	IsOverdue  bool      `json:"is_overdue"`
	CreatedAt  time.Time `json:"created_at"`
}

// AssignmentDetail is the per-module assignment view used by TrainingPage.tsx:
// one row per assigned target, with email and completion outcome joined in
// (as opposed to Assignment, which is the raw sr_assignments row).
type AssignmentDetail struct {
	ID          string     `json:"id"`
	ModuleID    string     `json:"module_id"`
	UserEmail   string     `json:"user_email"`
	Status      string     `json:"status"` // assigned | completed | failed
	AssignedAt  time.Time  `json:"assigned_at"`
	CompletedAt *time.Time `json:"completed_at,omitempty"`
	Score       *int       `json:"score,omitempty"`
}

// Completion records that an assignment was finished, along with the quiz score.
type Completion struct {
	ID           string    `json:"id"`
	OrgID        string    `json:"org_id"`
	AssignmentID string    `json:"assignment_id"`
	Score        *int      `json:"score,omitempty"`
	Passed       bool      `json:"passed"`
	CompletedAt  time.Time `json:"completed_at"`
}

// CompleteModuleInput carries the quiz answers submitted by the employee.
type CompleteModuleInput struct {
	Answers []int `json:"answers"` // for quiz modules
}

// CompleteAssignmentInput carries the quiz answers submitted when completing an assignment.
type CompleteAssignmentInput struct {
	Answers []int `json:"answers"`
}

// --- Feature 5: Phish-Button add-in ---

// PhishReport records a phishing email report submitted via the Outlook/Gmail add-in.
type PhishReport struct {
	ID            string    `json:"id"`
	OrgID         string    `json:"org_id"`
	CampaignID    *string   `json:"campaign_id,omitempty"`
	ReporterEmail string    `json:"reporter_email"`
	ReportedAt    time.Time `json:"reported_at"`
	Subject       string    `json:"subject,omitempty"`
	Sender        string    `json:"sender,omitempty"`
	IsSimulation  bool      `json:"is_simulation"`
	CreatedAt     time.Time `json:"created_at"`
}

// PhishReportWebhookInput is the body accepted by the public phish-report webhook.
type PhishReportWebhookInput struct {
	OrgToken      string `json:"org_token"      validate:"required"`
	ReporterEmail string `json:"reporter_email" validate:"required,email"`
	Subject       string `json:"subject"`
	Sender        string `json:"sender"`
}

// PhishReportStats holds aggregate counts for an org's phishing reports.
type PhishReportStats struct {
	Total       int `json:"total"`
	Simulations int `json:"simulations"`
	RealThreats int `json:"real_threats"`
}
