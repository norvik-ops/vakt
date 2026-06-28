package vaktcomply

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/matharnica/vakt/internal/modules/vaktcomply/policy"
	"github.com/matharnica/vakt/internal/modules/vaktcomply/reporting"
	"github.com/matharnica/vakt/internal/modules/vaktcomply/risk"
)

type (
	Framework                = policy.Framework
	EnableFrameworkInput     = policy.EnableFrameworkInput
	SwitchDORAVariantInput   = policy.SwitchDORAVariantInput
	Control                  = policy.Control
	ControlReview            = policy.ControlReview
	RecordReviewInput        = policy.RecordReviewInput
	UpdateControlInput       = policy.UpdateControlInput
	BulkUpdateControlsInput  = policy.BulkUpdateControlsInput
	UpdateSoAMetadataInput   = policy.UpdateSoAMetadataInput
	SoARow                   = policy.SoARow
	SoAEntry                 = policy.SoAEntry
	TISAXControlGap          = policy.TISAXControlGap
	TISAXGapAnalysis         = policy.TISAXGapAnalysis
	TISAXMaturitySummary     = policy.TISAXMaturitySummary
	ChapterMaturity          = policy.ChapterMaturity
	Policy                   = policy.Policy
	PolicyVersion            = policy.PolicyVersion
	CreatePolicyInput        = policy.CreatePolicyInput
	UpdatePolicyInput        = policy.UpdatePolicyInput
	GeneratePolicyDraftInput = policy.GeneratePolicyDraftInput
	ControlMeasure           = policy.ControlMeasure
	CreateMeasureInput       = policy.CreateMeasureInput
	UpdateMeasureInput       = policy.UpdateMeasureInput
	FrameworkMapping         = policy.FrameworkMapping
	ControlMapping           = policy.ControlMapping
	FrameworkPairCountRow    = policy.FrameworkPairCountRow
	ControlPrerequisiteRow   = policy.ControlPrerequisiteRow
	MappingResult            = policy.MappingResult
	ReadinessReport          = policy.ReadinessReport
	DomainScore              = policy.DomainScore
	GapAnalysis              = policy.GapAnalysis
	ControlGap               = policy.ControlGap
	ControlTask              = policy.ControlTask
	CreateControlTaskInput   = policy.CreateControlTaskInput
	UpdateControlTaskInput   = policy.UpdateControlTaskInput
	Evidence                 = policy.Evidence

	// SoA-dedicated types (defined in policy/repository_soa_dedicated.go).
	SoADedicatedEntry   = policy.SoADedicatedEntry
	SoAVersion          = policy.SoAVersion
	SoASummary          = policy.SoASummary
	UpdateSoAEntryInput = policy.UpdateSoAEntryInput

	// Physical-control templates (defined in policy/service_physical_templates.go).
	PhysicalControlTemplate = policy.PhysicalControlTemplate

	// Framework plugins (defined in policy/plugins.go).
	FrameworkPlugin    = policy.FrameworkPlugin
	PluginControl      = policy.PluginControl
	AvailableFramework = policy.AvailableFramework

	// Policy templates (defined in policy/policy_templates_seed.go).
	DBPolicyTemplate = policy.DBPolicyTemplate
	PolicyTemplate   = policy.PolicyTemplate

	// Policy-acceptance types (defined in policy/policy_acceptance.go).
	CreateCampaignInput        = policy.CreateCampaignInput
	PolicyAcceptanceCampaign   = policy.PolicyAcceptanceCampaign
	PolicyAcceptanceRequest    = policy.PolicyAcceptanceRequest
	PolicyAcceptanceSMTPConfig = policy.PolicyAcceptanceSMTPConfig

	// Framework-mapping return types (defined in policy/service_framework_mappings.go).
	MappingCoverageResponse = policy.MappingCoverageResponse
	ImplementationStep      = policy.ImplementationStep
)

// Re-exported package-level symbols moved into the policy/ sub-package that
// root-staying handlers still reference unqualified.
var (
	// ErrSoANotInitialized signals a dedicated SoA has not been initialised yet.
	ErrSoANotInitialized = policy.ErrSoANotInitialized
	// BuiltinPolicyTemplates returns the built-in policy template catalogue.
	BuiltinPolicyTemplates = policy.BuiltinPolicyTemplates
	// ISO27001AnnexAControls is the embedded ISO 27001:2022 Annex A control template list.
	ISO27001AnnexAControls = policy.ISO27001AnnexAControls
	// ErrExclusionReasonRequired signals a SoA exclusion lacks a justification.
	ErrExclusionReasonRequired = policy.ErrExclusionReasonRequired
)

func (s *Service) ReseedBuiltinControls(ctx context.Context) {
	s.Policy.ReseedBuiltinControls(ctx)
}

// SeedFrameworkMappings idempotently seeds the global cross-framework control mappings.
func (s *Service) SeedFrameworkMappings(ctx context.Context) error {
	return s.Policy.SeedFrameworkMappings(ctx)
}

// SeedPrerequisiteChains seeds the global control prerequisite chains.
func (s *Service) SeedPrerequisiteChains(ctx context.Context) error {
	return s.Policy.SeedPrerequisiteChains(ctx)
}

// SeedPolicyTemplates re-exports the policy-template seeder for startup wiring.
func SeedPolicyTemplates(ctx context.Context, db *pgxpool.Pool) error {
	return policy.SeedPolicyTemplates(ctx, db)
}

// GetControl returns a single control by ID for the given org.
func (s *Service) GetControl(ctx context.Context, orgID, controlID string) (*Control, error) {
	return s.Policy.GetControl(ctx, orgID, controlID)
}

// GetReadinessReport returns the readiness report for a framework.
func (s *Service) GetReadinessReport(ctx context.Context, orgID, frameworkID string) (*ReadinessReport, error) {
	return s.Policy.GetReadinessReport(ctx, orgID, frameworkID)
}

// ListFrameworks returns the list of frameworks for the given org.
func (s *Service) ListFrameworks(ctx context.Context, orgID string) ([]Framework, error) {
	return s.Policy.ListFrameworks(ctx, orgID)
}

// ListControls returns all controls for a framework in the given org.
func (s *Service) ListControls(ctx context.Context, orgID, frameworkID string) ([]Control, error) {
	return s.Policy.ListControls(ctx, orgID, frameworkID)
}

// UpdateControl updates a control's applicability, status, owner and maturity.
func (s *Service) UpdateControl(ctx context.Context, orgID, controlID string, input UpdateControlInput) (*Control, error) {
	return s.Policy.UpdateControl(ctx, orgID, controlID, input)
}

// BulkUpdateControlStatus sets the status for a batch of controls at once.
func (s *Service) BulkUpdateControlStatus(ctx context.Context, orgID string, ids []string, status string) error {
	return s.Policy.BulkUpdateControlStatus(ctx, orgID, ids, status)
}

// GetControlMappings returns cross-framework mappings for a control.
func (s *Service) GetControlMappings(ctx context.Context, orgID, controlID string) ([]ControlMapping, error) {
	return s.Policy.GetControlMappings(ctx, orgID, controlID)
}

// ListControlTasks returns the tasks attached to a control.
func (s *Service) ListControlTasks(ctx context.Context, orgID, controlID string) ([]ControlTask, error) {
	return s.Policy.ListControlTasks(ctx, orgID, controlID)
}

// CreateControlTask creates a new task for a control.
func (s *Service) CreateControlTask(ctx context.Context, orgID, controlID string, in CreateControlTaskInput) (*ControlTask, error) {
	return s.Policy.CreateControlTask(ctx, orgID, controlID, in)
}

// UpdateControlTask updates an existing control task.
func (s *Service) UpdateControlTask(ctx context.Context, orgID, controlID, taskID string, in UpdateControlTaskInput) (*ControlTask, error) {
	return s.Policy.UpdateControlTask(ctx, orgID, controlID, taskID, in)
}

// DeleteControlTask removes a task from a control.
func (s *Service) DeleteControlTask(ctx context.Context, orgID, controlID, taskID string) error {
	return s.Policy.DeleteControlTask(ctx, orgID, controlID, taskID)
}

// ListDedicatedSoAEntries returns the dedicated SoA entries for the org.
func (s *Service) ListDedicatedSoAEntries(ctx context.Context, orgID string) ([]SoADedicatedEntry, error) {
	return s.Policy.ListDedicatedSoAEntries(ctx, orgID)
}

// GetDedicatedSoASummary returns the SoA summary for the org.
func (s *Service) GetDedicatedSoASummary(ctx context.Context, orgID string) (*SoASummary, error) {
	return s.Policy.GetDedicatedSoASummary(ctx, orgID)
}

// ── Framework methods ────────────────────────────────────────────────────────

// EnableFramework enables a compliance framework for the org.
func (s *Service) EnableFramework(ctx context.Context, orgID, name, variant string) (*Framework, error) {
	return s.Policy.EnableFramework(ctx, orgID, name, variant)
}

// DeleteFramework removes a framework from the org.
func (s *Service) DeleteFramework(ctx context.Context, orgID, frameworkID string) error {
	return s.Policy.DeleteFramework(ctx, orgID, frameworkID)
}

// GetFramework returns a single framework by ID.
func (s *Service) GetFramework(ctx context.Context, orgID, frameworkID string) (*Framework, error) {
	return s.Policy.GetFramework(ctx, orgID, frameworkID)
}

// FindFrameworkByName returns a framework by its name.
func (s *Service) FindFrameworkByName(ctx context.Context, orgID, name string) (*Framework, error) {
	return s.Policy.FindFrameworkByName(ctx, orgID, name)
}

// SwitchDORAVariant changes the DORA variant for a framework.
func (s *Service) SwitchDORAVariant(ctx context.Context, orgID, frameworkID, newVariant string) (*Framework, error) {
	return s.Policy.SwitchDORAVariant(ctx, orgID, frameworkID, newVariant)
}

// GetGapAnalysis returns the gap analysis for a framework.
func (s *Service) GetGapAnalysis(ctx context.Context, orgID, frameworkID string) (*GapAnalysis, error) {
	return s.Policy.GetGapAnalysis(ctx, orgID, frameworkID)
}

// ListAvailableFrameworks returns all available frameworks with their enabled status.
func (s *Service) ListAvailableFrameworks(ctx context.Context, orgID string) ([]AvailableFramework, error) {
	return s.Policy.ListAvailableFrameworks(ctx, orgID)
}

// InstallFrameworkPlugin installs a user-provided framework plugin.
func (s *Service) InstallFrameworkPlugin(ctx context.Context, orgID string, plugin *FrameworkPlugin) (*Framework, error) {
	return s.Policy.InstallFrameworkPlugin(ctx, orgID, plugin)
}

// ListFrameworkMappings returns the cross-framework mappings for the org.
func (s *Service) ListFrameworkMappings(ctx context.Context, orgID string) ([]FrameworkMapping, error) {
	return s.Policy.ListFrameworkMappings(ctx, orgID)
}

// DeleteFrameworkMapping removes a framework mapping.
func (s *Service) DeleteFrameworkMapping(ctx context.Context, orgID, mappingID string) error {
	return s.Policy.DeleteFrameworkMapping(ctx, orgID, mappingID)
}

// ListTISAXControls returns TISAX controls filtered by protection level.
func (s *Service) ListTISAXControls(ctx context.Context, orgID, frameworkID, protectionLevel string) ([]Control, error) {
	return s.Policy.ListTISAXControls(ctx, orgID, frameworkID, protectionLevel)
}

// GetTISAXGapAnalysis returns the TISAX gap analysis.
func (s *Service) GetTISAXGapAnalysis(ctx context.Context, orgID, frameworkID string) (*TISAXGapAnalysis, error) {
	return s.Policy.GetTISAXGapAnalysis(ctx, orgID, frameworkID)
}

// GetTISAXCoverageByISO returns TISAX coverage mapped against ISO controls.
func (s *Service) GetTISAXCoverageByISO(ctx context.Context, orgID, tisaxFrameworkID string) ([]MappingResult, error) {
	return s.Policy.GetTISAXCoverageByISO(ctx, orgID, tisaxFrameworkID)
}

// GetTISAXGapsAfterISO returns TISAX controls not covered by ISO implementation.
func (s *Service) GetTISAXGapsAfterISO(ctx context.Context, orgID, tisaxFrameworkID string) ([]Control, error) {
	return s.Policy.GetTISAXGapsAfterISO(ctx, orgID, tisaxFrameworkID)
}

// GetMappingCoverage returns the cross-framework mapping coverage matrix.
func (s *Service) GetMappingCoverage(ctx context.Context, orgID string) (*MappingCoverageResponse, error) {
	return s.Policy.GetMappingCoverage(ctx, orgID)
}

// GetImplementationPath returns controls in topological order for implementation.
func (s *Service) GetImplementationPath(ctx context.Context, orgID, frameworkID string) ([]ImplementationStep, error) {
	return s.Policy.GetImplementationPath(ctx, orgID, frameworkID)
}

// ── Policy methods ───────────────────────────────────────────────────────────

// GetPolicy returns a single policy document by ID.
func (s *Service) GetPolicy(ctx context.Context, orgID, id string) (*Policy, error) {
	return s.Policy.GetPolicy(ctx, orgID, id)
}

// CreatePolicy creates a new policy document.
func (s *Service) CreatePolicy(ctx context.Context, orgID string, in CreatePolicyInput) (*Policy, error) {
	return s.Policy.CreatePolicy(ctx, orgID, in)
}

// UpdatePolicy updates a policy document.
func (s *Service) UpdatePolicy(ctx context.Context, orgID, id string, in UpdatePolicyInput) (*Policy, error) {
	return s.Policy.UpdatePolicy(ctx, orgID, id, in)
}

// ListPolicyVersions returns all versions of a policy.
func (s *Service) ListPolicyVersions(ctx context.Context, orgID, policyID string) ([]PolicyVersion, error) {
	return s.Policy.ListPolicyVersions(ctx, orgID, policyID)
}

// GetPolicyVersion returns a specific version of a policy.
func (s *Service) GetPolicyVersion(ctx context.Context, orgID, policyID string, version int) (PolicyVersion, error) {
	return s.Policy.GetPolicyVersion(ctx, orgID, policyID, version)
}

// GeneratePolicyDraft generates an AI-assisted draft for a policy.
func (s *Service) GeneratePolicyDraft(ctx context.Context, orgID string, in GeneratePolicyDraftInput) (string, error) {
	return s.Policy.GeneratePolicyDraft(ctx, orgID, in)
}

// ── SoA methods ──────────────────────────────────────────────────────────────

// GetSoAEntries returns the SoA entries for the org.
func (s *Service) GetSoAEntries(ctx context.Context, orgID string) ([]SoAEntry, error) {
	return s.Policy.GetSoAEntries(ctx, orgID)
}

// UpdateSoAApplicability updates the applicability of a SoA control.
func (s *Service) UpdateSoAApplicability(ctx context.Context, orgID, controlID string, applicable bool, justYes, justNo string) error {
	return s.Policy.UpdateSoAApplicability(ctx, orgID, controlID, applicable, justYes, justNo)
}

// InitDedicatedSoA initialises the dedicated SoA for the org.
func (s *Service) InitDedicatedSoA(ctx context.Context, orgID string) error {
	return s.Policy.InitDedicatedSoA(ctx, orgID)
}

// GetDedicatedSoAEntry returns a single dedicated SoA entry by control ref.
func (s *Service) GetDedicatedSoAEntry(ctx context.Context, orgID, controlRef string) (*SoADedicatedEntry, error) {
	return s.Policy.GetDedicatedSoAEntry(ctx, orgID, controlRef)
}

// UpdateDedicatedSoAEntry updates a dedicated SoA entry.
func (s *Service) UpdateDedicatedSoAEntry(ctx context.Context, orgID, controlRef string, in UpdateSoAEntryInput) error {
	return s.Policy.UpdateDedicatedSoAEntry(ctx, orgID, controlRef, in)
}

// ApproveDedicatedSoA approves the current draft SoA version.
func (s *Service) ApproveDedicatedSoA(ctx context.Context, orgID, approverID string) error {
	return s.Policy.ApproveDedicatedSoA(ctx, orgID, approverID)
}

// GetDedicatedSoAVersions returns all SoA versions for the org.
func (s *Service) GetDedicatedSoAVersions(ctx context.Context, orgID string) ([]SoAVersion, error) {
	return s.Policy.GetDedicatedSoAVersions(ctx, orgID)
}

// ExportDedicatedSoAPDF exports the dedicated SoA as a PDF document.
func (s *Service) ExportDedicatedSoAPDF(ctx context.Context, orgID string) ([]byte, error) {
	return s.Policy.ExportDedicatedSoAPDF(ctx, orgID)
}

// ExportDedicatedSoACSV exports the dedicated SoA as CSV rows.
func (s *Service) ExportDedicatedSoACSV(ctx context.Context, orgID string) ([][]string, error) {
	return s.Policy.ExportDedicatedSoACSV(ctx, orgID)
}

// ── Physical control templates ────────────────────────────────────────────────

// ListPhysicalControlTemplates returns all available physical control templates.
func (s *Service) ListPhysicalControlTemplates() []PhysicalControlTemplate {
	return s.Policy.ListPhysicalControlTemplates()
}

// ApplyPhysicalControlTemplate applies a template, creating evidence for the control.
func (s *Service) ApplyPhysicalControlTemplate(ctx context.Context, orgID, controlCode, userID string) (*Evidence, error) {
	return s.Policy.ApplyPhysicalControlTemplate(ctx, orgID, controlCode, userID)
}

// ── Handler helpers (package-level) ─────────────────────────────────────────

// filterControlsByScope delegates scope filtering to the policy sub-package.
// ponytail: thin shim so handler_frameworks.go compiles without importing policy.
func filterControlsByScope(controls []Control, scope string) []Control {
	return policy.FilterControlsByScope(controls, scope)
}

// yamlUnmarshal delegates YAML parsing to the policy sub-package.
func yamlUnmarshal(data []byte, v any) error {
	return policy.YAMLUnmarshal(data, v)
}

// enrichControlsWithNIS2Meta delegates NIS2 metadata enrichment to the policy sub-package.
// ponytail: shim for handler_frameworks.go which previously called a root-package func.
func enrichControlsWithNIS2Meta(cs []Control) {
	policy.EnrichControlsWithNIS2Meta(cs)
}

type KPISnapshot = reporting.KPISnapshot
type KPIDashboard = reporting.KPIDashboard

// NIS2 types — defined in reporting sub-package, aliased here for API compatibility.
type NIS2ReportabilityCheck = reporting.NIS2ReportabilityCheck
type NIS2ReportInput = reporting.NIS2ReportInput
type NIS2ReportStatus = reporting.NIS2ReportStatus
type NIS2Deadlines = reporting.NIS2Deadlines
type NIS2StageReport = reporting.NIS2StageReport
type AuthorityContact = reporting.AuthorityContact

func (s *Service) CalculateAndStoreKPIs(ctx context.Context, orgID string) error {
	return s.Reporting.CalculateAndStoreKPIs(ctx, orgID)
}

// GetKPIDashboard returns the latest KPI snapshot and the 90-day history.
func (s *Service) GetKPIDashboard(ctx context.Context, orgID string) (KPIDashboard, error) {
	return s.Reporting.GetKPIDashboard(ctx, orgID)
}

// ── NIS2 Art.23 forwarders ────────────────────────────────────────────────────

// MarkIncidentReportable marks an incident as NIS2-meldepflichtig and sets the three deadlines.
func (s *Service) MarkIncidentReportable(ctx context.Context, orgID string, incidentID uuid.UUID, detectedAt time.Time, check NIS2ReportabilityCheck) error {
	return s.Reporting.MarkIncidentReportable(ctx, orgID, incidentID, detectedAt, check)
}

// SubmitNIS2Stage saves report content for a stage and marks it submitted.
func (s *Service) SubmitNIS2Stage(ctx context.Context, orgID, incidentID, userID, stage string, input NIS2ReportInput) (*NIS2StageReport, error) {
	return s.Reporting.SubmitNIS2Stage(ctx, orgID, incidentID, userID, stage, input)
}

// GetNIS2Status returns the full NIS2 reporting status for an incident.
func (s *Service) GetNIS2Status(ctx context.Context, orgID, incidentID string) (*NIS2ReportStatus, error) {
	return s.Reporting.GetNIS2Status(ctx, orgID, incidentID)
}

// CheckNIS2StagingDeadlines checks all open NIS2 incidents for upcoming deadlines.
func (s *Service) CheckNIS2StagingDeadlines(ctx context.Context, orgID string) error {
	return s.Reporting.CheckNIS2StagingDeadlines(ctx, orgID)
}

// ListAuthorityContacts returns authority contacts for the given org (including built-ins).
func (s *Service) ListAuthorityContacts(ctx context.Context, orgID string) ([]AuthorityContact, error) {
	return s.Reporting.ListAuthorityContacts(ctx, orgID)
}

// CreateAuthorityContact creates a custom authority contact for an org.
func (s *Service) CreateAuthorityContact(ctx context.Context, orgID string, in AuthorityContact) (*AuthorityContact, error) {
	return s.Reporting.CreateAuthorityContact(ctx, orgID, in)
}

type (
	Risk                        = risk.Risk
	CreateRiskInput             = risk.CreateRiskInput
	UpdateRiskInput             = risk.UpdateRiskInput
	UpdateRiskTreatmentInput    = risk.UpdateRiskTreatmentInput
	UpdateRiskResidualInput     = risk.UpdateRiskResidualInput
	AcceptRiskInput             = risk.AcceptRiskInput
	DORAThirdParty              = risk.DORAThirdParty
	CreateDORAThirdPartyInput   = risk.CreateDORAThirdPartyInput
	UpdateDORAThirdPartyInput   = risk.UpdateDORAThirdPartyInput
	ProtectionNeedAssessment    = risk.ProtectionNeedAssessment
	CreateProtectionNeedInput   = risk.CreateProtectionNeedInput
	UpdateProtectionNeedInput   = risk.UpdateProtectionNeedInput
	CAPA                        = risk.CAPA
	CreateCAPAInput             = risk.CreateCAPAInput
	UpdateCAPAInput             = risk.UpdateCAPAInput
	BulkUpdateCAPAsInput        = risk.BulkUpdateCAPAsInput
	CAPANCFields                = risk.CAPANCFields
	EffectivenessCheckInput     = risk.EffectivenessCheckInput
	ControlException            = risk.ControlException
	CreateControlExceptionInput = risk.CreateControlExceptionInput
	UpdateControlExceptionInput = risk.UpdateControlExceptionInput
)

// CalculateOverallProtectionNeed re-exports the risk-domain helper so existing
// root callers/tests keep working.
var CalculateOverallProtectionNeed = risk.CalculateOverallProtectionNeed
