export interface Framework {
  id: string
  name: string
  version: string
  created_at: string
  control_count?: number
  framework_variant?: 'full' | 'simplified'
}

export interface ReadinessReport {
  framework_id: string
  framework_name: string
  readiness_score: number // 0-100
  total_controls: number
  covered: number
  partial: number
  missing: number
  by_domain: Array<{ domain: string; score: number; total: number; covered: number }>
  tisax_maturity?: TISAXMaturitySummary
}

export interface GapAnalysis {
  framework_id: string
  gaps: Array<{
    control: Control
    reason: 'no_evidence' | 'evidence_expiring'
    expires_at?: string
  }>
}

export interface Control {
  id: string
  framework_id: string
  control_id: string // e.g. "NIS2-A.1"
  title: string
  description: string
  domain: string
  status: 'covered' | 'partial' | 'missing' | 'not_applicable' | 'in_progress' | 'implemented'
  not_applicable: boolean
  not_applicable_reason?: string
  evidence_count?: number
  iso27001_mapping?: string
  maturity_score?: number // 0–3 (TISAX VDA ISA maturity level)
  owner?: string // Migration 106
  // Review tracking (Migration 075)
  last_reviewed_at?: string
  review_interval_days?: number
  next_review_due?: string
  last_reviewed_by?: string
  review_note?: string
  is_review_overdue?: boolean
  due_date?: string | null // ISO date "2026-01-31"
  manual_status?: '' | 'in_progress' | 'implemented'
}

export interface UpdateControlInput {
  not_applicable: boolean
  reason: string
  manual_status: '' | 'in_progress' | 'implemented'
  maturity_score?: number
  owner?: string
  due_date?: string | null
}

// --- TISAX types (Story 28.1 + 28.3) ---

export interface ChapterMaturity {
  domain: string
  avg_score: number
  total_controls: number
  fully_mature: number
  color: 'green' | 'yellow' | 'red'
}

export interface TISAXMaturitySummary {
  avg_score: number
  readiness_percent: number
  by_chapter: ChapterMaturity[]
}

export interface TISAXControlGap {
  control: Control
  maturity_gap: number
  current_score: number
}

export interface TISAXGapAnalysis {
  framework_id: string
  target_score: number
  gaps: TISAXControlGap[]
}

export interface Evidence {
  id: string
  control_id: string
  title: string
  type: 'manual' | 'automated' | 'document'
  notes?: string
  status: 'pending_review' | 'approved' | 'rejected' | 'expired'
  expires_at?: string | null
  expiry_notified_at?: string | null
  created_at: string
}

export interface AuditorLink {
  id: string
  label?: string
  expires_at: string
  last_accessed_at?: string
  access_count: number
  revoked_at?: string
}

export type TreatmentOption = 'accept' | 'mitigate' | 'transfer' | 'avoid'
export type TreatmentStatus = 'pending' | 'in_progress' | 'implemented' | 'verified'

export interface Risk {
  id: string
  org_id: string
  title: string
  description?: string
  category?: string
  likelihood: number
  impact: number
  risk_score: number
  owner?: string
  status: 'open' | 'mitigated' | 'accepted' | 'closed'
  treatment: 'avoid' | 'mitigate' | 'transfer' | 'accept'
  treatment_notes?: string
  // Treatment workflow fields (Migration 071)
  treatment_option?: TreatmentOption
  treatment_plan?: string
  treatment_owner?: string
  treatment_due_date?: string | null
  treatment_status?: TreatmentStatus
  residual_likelihood?: number | null
  residual_impact?: number | null
  ai_narrative?: string | null
  // Residualrisiko-Berechnung (S61-4, Migration 164)
  inherent_likelihood?: number
  inherent_impact?: number
  residual_score?: number
  inherent_score?: number
  risk_accepted_by?: string
  risk_accepted_at?: string
  risk_acceptance_justification?: string
  created_at: string
  updated_at: string
}

export interface AcceptRiskInput {
  justification: string
}

export interface AIInsight {
  id: string
  type: 'evidence_stale' | 'evidence_suggestion' | 'gap_explain_saved'
  title: string
  message: string
  control_id?: string | null
  risk_id?: string | null
  finding_id?: string | null
  urgency: 1 | 2 | 3
  created_at: string
}

export interface UpdateRiskTreatmentInput {
  treatment_option?: TreatmentOption
  treatment_plan?: string
  treatment_owner?: string
  treatment_due_date?: string | null
  treatment_status?: TreatmentStatus
  residual_likelihood?: number | null
  residual_impact?: number | null
}

export interface CreateRiskInput {
  title: string
  description?: string
  category?: string
  likelihood: number
  impact: number
  owner?: string
  treatment: 'avoid' | 'mitigate' | 'transfer' | 'accept'
  treatment_notes?: string
}

export type IncidentType = 'general' | 'nis2' | 'dora'
export type ReportingObligation = 'unknown' | 'required' | 'not_required'
export type DeadlineStatus = 'green' | 'yellow' | 'red' | 'done'

export interface DeadlineInfo {
  deadline: string
  reported_at?: string
  status: DeadlineStatus
  hours_left: number
}

export interface IncidentDeadlineStatus {
  has_4h: boolean
  has_24h: boolean
  has_72h: boolean
  has_30d: boolean
  d_4h?: DeadlineInfo
  d_24h?: DeadlineInfo
  d_72h?: DeadlineInfo
  d_30d?: DeadlineInfo
}

export interface Incident {
  id: string
  org_id: string
  title: string
  description?: string
  severity: 'low' | 'medium' | 'high' | 'critical'
  status: 'open' | 'investigating' | 'resolved' | 'closed'
  discovered_at: string
  resolved_at?: string
  affected_systems: string[]
  breach_id?: string
  incident_type: IncidentType
  reporting_obligation: ReportingObligation
  notification_authority?: string
  deadline_4h?: string
  deadline_24h?: string
  deadline_72h?: string
  deadline_30d?: string
  reported_4h_at?: string
  reported_24h_at?: string
  reported_72h_at?: string
  reported_30d_at?: string
  // DORA-specific fields (Migration 041)
  affected_customers?: number
  financial_impact_estimate?: string
  is_major_incident: boolean
  deadline_status?: IncidentDeadlineStatus
  // NIS2 Art.23 stage-based reporting (Migration 175, S67-1)
  nis2_reportable?: boolean
  nis2_reporting_stage?: 'none' | 'early_warning' | 'full_report' | 'final_report'
  nis2_detected_at?: string
  nis2_early_warning_due?: string
  nis2_full_report_due?: string
  nis2_final_report_due?: string
  nis2_early_warning_submitted_at?: string
  nis2_full_report_submitted_at?: string
  nis2_final_report_submitted_at?: string
  created_at: string
  updated_at: string
}

// S67-1: NIS2 Art.23 reporting types
export interface NIS2ReportabilityCheck {
  causes_significant_disruption: boolean
  affects_third_parties: boolean
  causes_financial_damage: boolean
}

export interface NIS2ReportInput {
  affected_services?: string
  initial_assessment?: string
  root_cause?: string
  affected_users_estimate?: number
  measures_taken?: string
  estimated_recovery?: string
  full_root_cause_analysis?: string
  permanent_measures?: string
  effectiveness_evidence?: string
}

export interface NIS2StageReport {
  id: string
  stage: 'early_warning' | 'full_report' | 'final_report'
  submitted_at?: string
  pdf_path?: string
}

export interface NIS2ReportStatus {
  is_reportable: boolean
  reporting_stage: string
  detected_at?: string
  deadlines: {
    early_warning?: string
    full_report?: string
    final_report?: string
  }
  completed_stages: string[]
  reports: NIS2StageReport[]
}

export interface AuthorityContact {
  id: string
  org_id?: string
  country: 'de' | 'at' | 'ch' | 'eu'
  sector?: string
  authority_name: string
  report_url?: string
  email?: string
  phone?: string
  notes?: string
  is_primary: boolean
  is_builtin: boolean
  created_at: string
}

// S67-4: Evidence staleness types
export interface ComplianceScore {
  total_controls: number
  ok_count: number
  stale_count: number
  missing_count: number
  na_count: number
  score_pct: number
  as_of: string
}

// S67-3: Classification types
export type ClassificationLevel = 'public' | 'internal' | 'confidential' | 'restricted'

export interface ClassificationSummary {
  total_count: number
  classified_count: number
  by_level: Record<ClassificationLevel, number>
  unclassified_count: number
}

// S67-6: Crypto key types
export type CryptoKeyType = 'symmetric' | 'asymmetric' | 'certificate' | 'hmac' | 'signing' | 'other'
export type RotationStatus = 'ok' | 'due_soon' | 'overdue' | 'none'

export interface CryptoKey {
  id: string
  org_id: string
  name: string
  key_type: CryptoKeyType
  algorithm: string
  key_length?: number
  purpose: string
  location?: string
  rotation_interval_days?: number
  last_rotation_date?: string
  next_rotation_due?: string
  expiry_date?: string
  is_weak_algorithm: boolean
  rotation_status: RotationStatus
  notes?: string
  created_at: string
  updated_at: string
}

export interface CreateCryptoKeyInput {
  name: string
  key_type: CryptoKeyType
  algorithm: string
  key_length?: number
  purpose: string
  location?: string
  rotation_interval_days?: number
  last_rotation_date?: string
  expiry_date?: string
  notes?: string
}

export interface CreateIncidentInput {
  title: string
  description: string
  severity: 'low' | 'medium' | 'high' | 'critical'
  discovered_at: string
  affected_systems: string[]
  breach_id?: string
  incident_type?: IncidentType
  reporting_obligation?: ReportingObligation
  notification_authority?: string
  // DORA-specific fields (Migration 041)
  affected_customers?: number
  financial_impact_estimate?: string
  is_major_incident?: boolean
}

export interface Policy {
  id: string
  org_id: string
  title: string
  description?: string
  category?: string
  status: 'draft' | 'active' | 'archived'
  version: string       // user-editable version label, e.g. "1.0"
  version_num: number   // auto-incremented integer version counter (Migration 076)
  version_note: string
  last_updated_by: string
  reviewed_at?: string
  next_review_due?: string
  effective_date?: string
  review_date?: string
  owner?: string
  created_at: string
  updated_at: string
}

export interface CreatePolicyInput {
  title: string
  description?: string
  category?: string
  version?: string
  effective_date?: string
  review_date?: string
  owner?: string
}

export interface AuditRecord {
  id: string
  org_id: string
  title: string
  scope?: string
  auditor?: string
  audit_date: string
  status: 'planned' | 'in_progress' | 'completed'
  findings?: string
  recommendations?: string
  created_at: string
  updated_at: string
}

export interface CreateAuditRecordInput {
  title: string
  scope?: string
  auditor?: string
  audit_date: string
  findings?: string
  recommendations?: string
}

export interface UpdateRiskInput {
  title: string
  description?: string
  category?: string
  likelihood: number
  impact: number
  owner?: string
  status: Risk['status']
  treatment: Risk['treatment']
  treatment_notes?: string
}

export interface UpdateIncidentInput {
  title: string
  description: string
  severity: Incident['severity']
  status: Incident['status']
  affected_systems: string[]
  incident_type?: IncidentType
  reporting_obligation?: ReportingObligation
  notification_authority?: string
  // DORA-specific fields (Migration 041)
  affected_customers?: number
  financial_impact_estimate?: string
  is_major_incident?: boolean
}

export interface MarkDeadlineReportedInput {
  deadline: '4h' | '24h' | '72h' | '30d'
}

// --- Supplier Register ---

export interface Supplier {
  id: string
  org_id: string
  name: string
  contact_name?: string
  contact_email?: string
  service_type?: string
  criticality: 'standard' | 'important' | 'critical'
  nis2_relevant: boolean
  dora_relevant: boolean
  contract_end?: string
  notes?: string
  // DORA-specific fields (Migration 042)
  sub_suppliers?: string[]
  data_location?: string
  exit_strategy_exists?: boolean
  // Assessment fields (Migration 046)
  assessment_status?: 'none' | 'pending' | 'completed'
  last_assessment_at?: string
  // Computed by service layer
  contract_status?: string
  created_at: string
  updated_at: string
}

export interface CSVImportError {
  row: number
  message: string
}

export interface CSVImportResult {
  imported: number
  skipped: number
  errors: CSVImportError[]
}

export interface CreateSupplierInput {
  name: string
  contact_name?: string
  contact_email?: string
  service_type?: string
  criticality?: 'standard' | 'important' | 'critical'
  nis2_relevant?: boolean
  dora_relevant?: boolean
  contract_end?: string
  notes?: string
  // DORA-specific fields (Migration 042)
  sub_suppliers?: string[]
  data_location?: string
  exit_strategy_exists?: boolean
  // Assessment fields (Migration 046)
  assessment_status?: 'none' | 'pending' | 'completed'
  last_assessment_at?: string
}

export type UpdateSupplierInput = CreateSupplierInput

// --- AI System Inventory ---

export interface AISystem {
  id: string
  org_id: string
  name: string
  description?: string
  provider?: string
  use_case?: string
  affected_groups?: string
  autonomy_level: 'assistive' | 'partial' | 'full'
  in_production_since?: string
  status: 'under_review' | 'approved' | 'prohibited' | 'decommissioned'
  risk_class?: 'minimal' | 'limited' | 'high' | 'unacceptable'
  classification_rationale?: string
  classified_at?: string
  classified_by?: string
  created_at: string
  updated_at: string
}

export interface CreateAISystemInput {
  name: string
  description?: string
  provider?: string
  use_case?: string
  affected_groups?: string
  autonomy_level?: 'assistive' | 'partial' | 'full'
  in_production_since?: string
  risk_class?: 'minimal' | 'limited' | 'high' | 'unacceptable'
  classification_rationale?: string
}

export interface UpdateAISystemInput extends CreateAISystemInput {
  status?: 'under_review' | 'approved' | 'prohibited' | 'decommissioned'
  classified_by?: string
}

export interface AIClassification {
  id: string
  org_id: string
  ai_system_id: string
  risk_class: string
  rationale?: string
  classified_by?: string
  wizard_answers?: Record<string, boolean>
  classified_at: string
}

export interface ClassifyAISystemInput {
  risk_class: string
  rationale?: string
  classified_by?: string
  wizard_answers?: Record<string, boolean>
}

export interface AIDocumentation {
  id: string
  org_id: string
  ai_system_id: string
  version: number
  system_description?: string
  intended_purpose?: string
  training_data?: string
  data_quality?: string
  performance_metrics?: string
  system_limits?: string
  risk_management?: string
  human_oversight?: string
  logging_audit_trail?: string
  authored_by?: string
  status: 'draft' | 'final'
  created_at: string
  updated_at: string
}

export interface UpsertAIDocumentationInput {
  system_description?: string
  intended_purpose?: string
  training_data?: string
  data_quality?: string
  performance_metrics?: string
  system_limits?: string
  risk_management?: string
  human_oversight?: string
  logging_audit_trail?: string
  authored_by?: string
  status?: 'draft' | 'final'
}

export interface UpdatePolicyInput {
  title: string
  description?: string
  category?: string
  status: Policy['status']
  version?: string
  effective_date?: string
  review_date?: string
  owner?: string
  // Versioning fields (Migration 076)
  version_note?: string
  updated_by?: string
  next_review_due?: string
}

export interface UpdateAuditRecordInput {
  title: string
  scope?: string
  auditor?: string
  audit_date: string
  status: AuditRecord['status']
  findings?: string
  recommendations?: string
}

export interface ControlTask {
  id: string
  control_id: string
  org_id: string
  text: string
  completed: boolean
  created_at: string
  updated_at: string
}

// --- Resilience Tests (DORA Art. 24-27) ---

export interface ResilienceTest {
  id: string
  org_id: string
  type: 'tlpt' | 'pentest' | 'scenario_based' | 'vulnerability_assessment'
  scope?: string
  provider?: string
  test_date: string
  summary?: string
  remediation_status: 'open' | 'in_progress' | 'completed' | 'accepted'
  attachment_url?: string
  overdue_warning?: boolean
  created_at: string
  updated_at: string
}

export interface ResilienceTestsResponse {
  tests: ResilienceTest[]
  tlpt_overdue_warning: boolean
}

export interface CreateResilienceTestInput {
  type: string
  scope?: string
  provider?: string
  test_date: string
  summary?: string
  remediation_status?: string
}

export type UpdateResilienceTestInput = Partial<CreateResilienceTestInput> & { remediation_status: string }

// --- Framework Mappings (Story 28.2) ---

export interface FrameworkMapping {
  id: string
  org_id: string
  source_control_id: string
  target_control_id: string
  created_at: string
}

export interface MappingResult {
  tisax_control_id: string
  tisax_control_title: string
  iso_control_id: string
  iso_control_title: string
  covered: boolean
}

// --- Questionnaire Builder (Story 29.2) ---

export type QuestionType = 'yes_no' | 'multiple_choice' | 'free_text' | 'file_upload'

export interface Question {
  id: string
  questionnaire_id: string
  order_idx: number
  question_text: string
  question_type: QuestionType
  options?: string[]
  required: boolean
  control_id?: string
  created_at: string
  updated_at: string
}

export interface Questionnaire {
  id: string
  org_id: string
  name: string
  description?: string
  is_template: boolean
  questions?: Question[]
  created_at: string
  updated_at: string
}

export interface CreateQuestionnaireInput {
  name: string
  description?: string
  is_template?: boolean
  clone_from_id?: string
}

export interface CreateQuestionInput {
  question_text: string
  question_type: QuestionType
  options?: string[]
  required?: boolean
  control_id?: string
}

export interface ReorderQuestionsInput {
  order: string[]
}

// --- DORA Dashboard (Story 27.5) ---

export interface NextDeadline {
  incident_id: string
  title: string
  deadline_type: '4h' | '24h' | '72h' | '30d'
  deadline_at: string
}

export interface DORADashboard {
  readiness_pct: number
  open_critical_controls: number
  next_deadline?: NextDeadline
  expired_suppliers: number
  tlpt_overdue_warning: boolean
  // IKT-Drittanbieter (S38-1/2/3)
  third_party_count: number
  critical_third_parties: number
  missing_exit_strategies: number
}

// --- Assessment Review (Story 29.4) ---

export interface AnswerWithReview {
  id: string
  question_text: string
  answer_text: string
  file_url: string
  review_status?: "accepted" | "needs_rework"
  rework_note?: string
  control_id?: string
  cert_expiry_date?: string
  evidence_id?: string
}

export interface SupplierStatus {
  supplier_id: string
  status: "green" | "yellow" | "red"
  score: number
  details: Record<string, unknown>
}

export interface ReviewAnswerInput {
  review_status: "accepted" | "needs_rework"
  rework_note?: string
}

export interface Assessment {
  id: string
  org_id: string
  supplier_id: string
  questionnaire_id: string
  status: string
  expires_at: string
  submitted_at?: string
  created_at: string
  share_url?: string
}

// --- Story 31.4: Org Sector & Authority Directory ---

export interface OrgSectorSettings {
  sector: string
  federal_state?: string
}

export interface UpdateOrgSectorInput {
  sector: string
  federal_state?: string
}

export interface AuthorityInfo {
  name: string
  portal: string
  phone: string
  submit_note: string
}

export const SECTOR_LABELS: Record<string, string> = {
  energy:       'Energie',
  water:        'Wasser',
  health:       'Gesundheit',
  finance:      'Finanz / Versicherung',
  transport:    'Transport',
  telecom:      'Telekommunikation',
  waste:        'Abfall',
  aerospace:    'Luftfahrt / Raumfahrt',
  public_admin: 'Öffentliche Verwaltung',
  other:        'Sonstige (KRITIS)',
}

// --- Story 31.3: Incident Report Archive ---

export interface IncidentReport {
  id: string
  org_id: string
  incident_id: string
  report_type: '24h' | '72h' | '30d'
  authority: string
  generated_at: string
}

export interface GenerateReportInput {
  report_type: '24h' | '72h' | '30d'
}

// --- Access Review Campaigns ---

export interface AccessReviewCampaign {
  id: string
  org_id: string
  title: string
  description?: string
  status: 'draft' | 'active' | 'completed' | 'cancelled'
  reviewer_email: string
  scope?: string
  due_date?: string
  completed_at?: string
  created_by?: string
  created_at: string
  updated_at: string
}

export interface AccessReviewItem {
  id: string
  campaign_id: string
  org_id: string
  user_email: string
  access_level: string
  decision: 'pending' | 'approved' | 'revoked'
  reviewer_comment?: string
  decided_at?: string
  created_at: string
}

export interface CreateAccessReviewCampaignInput {
  title: string
  description?: string
  reviewer_email: string
  scope?: string
  due_date?: string
}

export interface UpdateAccessReviewCampaignInput {
  title?: string
  description?: string
  reviewer_email?: string
  scope?: string
  due_date?: string
  status?: AccessReviewCampaign['status']
}

export interface CreateAccessReviewItemInput {
  campaign_id: string
  user_email: string
  access_level: string
}

export interface UpdateAccessReviewItemInput {
  decision?: AccessReviewItem['decision']
  reviewer_comment?: string
}

// --- Story 31.1: Reportability Assessment ---

export interface ReportabilityAnswers {
  affects_external_data: boolean
  affects_essential_service: boolean
  personal_data_compromised: boolean
}

// eslint-disable-next-line @typescript-eslint/no-empty-object-type
export interface AssessReportabilityInput extends ReportabilityAnswers {}

export interface ReportabilityResult {
  obligation: 'required' | 'not_required' | 'unknown'
  gdpr_required: boolean
  notification_authority: string
  explanation: string
  answers: ReportabilityAnswers
}

// --- S39-1: BSI-Meldepflicht-Klassifizierung ---

export interface ClassifyReportingInput {
  essential_service: boolean
  customer_data: boolean
  personal_data: boolean
}

export interface ClassificationResult {
  obligation: 'probably' | 'none' | 'unclear'
  authority: string
  reason: string
}

// --- CCM (Continuous Control Monitoring) ---

export type CCMCheckType = 'http_endpoint' | 'trivy_no_critical' | 'evidence_freshness' | 'custom_script'
export type CCMStatus = 'pass' | 'fail' | 'unknown'

export interface CCMCheck {
  id: string
  org_id: string
  control_id: string
  name: string
  check_type: CCMCheckType
  config: Record<string, string>
  interval_hours: number
  last_run_at?: string
  last_status?: CCMStatus
  last_output?: string
  enabled: boolean
  created_at: string
  updated_at: string
}

export interface CreateCCMCheckInput {
  control_id: string
  name: string
  check_type: CCMCheckType
  config: Record<string, string>
  interval_hours: number
}

export interface CCMResult {
  id: string
  check_id: string
  status: CCMStatus
  output?: string
  ran_at: string
}


// --- Collaborative Tasks & Comments ---

export type TaskStatus = 'open' | 'in_progress' | 'done'
export type TaskPriority = 'low' | 'medium' | 'high' | 'critical'

export interface CollabTask {
  id: string
  org_id: string
  entity_type: string
  entity_id: string
  title: string
  description: string
  assignee_email: string
  due_date: string | null
  status: TaskStatus
  priority: TaskPriority
  created_by: string
  created_at: string
  updated_at: string
}

export interface CreateCollabTaskInput {
  title: string
  description?: string
  assignee_email?: string
  due_date?: string
  status?: TaskStatus
  priority?: TaskPriority
}

export interface UpdateCollabTaskInput {
  title?: string
  description?: string
  assignee_email?: string
  due_date?: string
  status?: TaskStatus
  priority?: TaskPriority
}

export interface CollabComment {
  id: string
  org_id: string
  entity_type: string
  entity_id: string
  author_email: string
  body: string
  created_at: string
}

export interface CreateCommentInput {
  body: string
  author_email?: string
}

// --- Audit Milestones / Certification Timeline (Migration 092) ---

export type MilestoneType =
  | 'internal_audit'
  | 'external_audit'
  | 'certification_target'
  | 'review_deadline'
  | 'training_deadline'
  | 'custom'

export type MilestoneStatus = 'upcoming' | 'completed' | 'missed' | 'cancelled'

export interface AuditMilestone {
  id: string
  org_id: string
  framework_id?: string | null
  title: string
  description?: string
  milestone_date: string // YYYY-MM-DD
  milestone_type: MilestoneType
  status: MilestoneStatus
  created_by?: string | null
  created_at: string
  updated_at: string
  days_remaining?: number | null
}

export interface CreateMilestoneInput {
  framework_id?: string
  title: string
  description?: string
  milestone_date: string
  milestone_type: MilestoneType
}

export interface UpdateMilestoneInput {
  title?: string
  description?: string
  milestone_date?: string
  milestone_type?: MilestoneType
  status?: MilestoneStatus
}

// --- DORA IKT-Drittanbieter-Register (S38-1) ---

export type DORAServiceType = 'IT-Outsourcing' | 'Cloud' | 'SaaS' | 'Netzwerk' | 'Sonstiges'
export type DORAThirdPartyCriticality = 'kritisch' | 'wichtig' | 'unkritisch'
export type DORADataLocation = 'EU' | 'Non-EU' | 'Mixed'

export interface DORAThirdParty {
  id: string
  org_id: string
  name: string
  service_type: DORAServiceType
  criticality: DORAThirdPartyCriticality
  contract_start?: string | null
  contract_end?: string | null
  sla_rto_hours?: number | null
  sla_availability?: number | null
  has_subcontractors: boolean
  subcontractor_names?: string
  data_location: DORADataLocation
  exit_strategy: boolean
  exit_notes?: string
  notes?: string
  created_by?: string | null
  created_at: string
  updated_at: string
  control_ids?: string[]
}

export interface CreateDORAThirdPartyInput {
  name: string
  service_type: DORAServiceType
  criticality: DORAThirdPartyCriticality
  contract_start?: string | null
  contract_end?: string | null
  sla_rto_hours?: number | null
  sla_availability?: number | null
  has_subcontractors: boolean
  subcontractor_names?: string
  data_location: DORADataLocation
  exit_strategy: boolean
  exit_notes?: string
  notes?: string
}

export type UpdateDORAThirdPartyInput = CreateDORAThirdPartyInput

// --- BCP / Notfallhandbuch (Migration 156) ---

export interface BCPPlan {
  id: string
  org_id: string
  title: string
  scope: string
  version: string
  status: 'draft' | 'active' | 'archived'
  owner: string
  created_at: string
  updated_at: string
}

export interface CreateBCPPlanInput {
  title: string
  scope?: string
  version?: string
  owner?: string
}

export interface UpdateBCPPlanInput {
  title: string
  scope?: string
  version?: string
  status?: BCPPlan['status']
  owner?: string
}

export interface BCPTest {
  id: string
  org_id: string
  plan_id: string
  test_date: string
  test_type: 'tabletop' | 'walkthrough' | 'fulltest'
  outcome: 'passed' | 'failed' | 'partial'
  findings: string
  created_at: string
}

export interface CreateBCPTestInput {
  plan_id: string
  test_date: string
  test_type: BCPTest['test_type']
  outcome: BCPTest['outcome']
  findings?: string
}

// --- Schutzbedarfsfeststellung (Migration 157) ---

export type ProtectionLevel = 'normal' | 'hoch' | 'sehr_hoch'
export type ProtectionObjectType = 'process' | 'system' | 'information' | 'location'

export interface ProtectionNeedAssessment {
  id: string
  org_id: string
  name: string
  object_type: ProtectionObjectType
  object_name: string
  confidentiality: ProtectionLevel
  integrity: ProtectionLevel
  availability: ProtectionLevel
  overall: ProtectionLevel
  status: 'draft' | 'finalized'
  vb_asset_id?: string | null
  finalized_at?: string | null
  created_at: string
  updated_at: string
}

export interface CreateProtectionNeedInput {
  name: string
  object_type: ProtectionObjectType
  object_name: string
  confidentiality: ProtectionLevel
  integrity: ProtectionLevel
  availability: ProtectionLevel
}

export interface UpdateProtectionNeedInput extends CreateProtectionNeedInput {
  status?: 'draft' | 'finalized'
}

// ── S61-1: ISMS Scope ────────────────────────────────────────────────────────
export interface ISMSScopeExclusion {
  item: string
  justification: string
}

export interface ISMSScope {
  id: string
  org_id: string
  version: number
  status: 'draft' | 'approved'
  scope_definition: string
  exclusions: ISMSScopeExclusion[]
  outsourcing_dependencies: string
  change_note: string
  approved_by?: string
  approved_at?: string
  created_by: string
  created_at: string
  updated_at: string
}

export interface CreateISMSScopeInput {
  scope_definition: string
  exclusions: ISMSScopeExclusion[]
  outsourcing_dependencies: string
  change_note: string
}

// ── S61-6: Pentest Tracking ──
export interface Pentest {
  id: string
  org_id: string
  title: string
  scope: string
  pentest_date: string
  tester_type: 'internal' | 'external'
  tester_name: string
  methodology?: 'blackbox' | 'greybox' | 'whitebox'
  findings_critical: number
  findings_high: number
  findings_medium: number
  findings_low: number
  status: 'in_progress' | 'completed' | 'remediation' | 'closed'
  retest_date?: string
  notes: string
  created_by: string
  created_at: string
  updated_at: string
}

export interface CreatePentestInput {
  title: string
  scope: string
  pentest_date: string
  tester_type: 'internal' | 'external'
  tester_name?: string
  methodology?: 'blackbox' | 'greybox' | 'whitebox'
  findings_critical?: number
  findings_high?: number
  findings_medium?: number
  findings_low?: number
  notes?: string
}

// ── S61-5: BSI Modellierung ──

export interface BSIModelingEntry {
  id: string
  org_id: string
  asset_id: string
  control_id: string
  priority: 'R1' | 'R2' | 'R3'
  justification_for_exclusion: string
  check_status?: 'yes' | 'partial' | 'no' | 'not_applicable'
  interview_notes: string
  site_visit_notes: string
  asset_name: string
  control_title: string
  framework_id: string
  created_by: string
  created_at: string
  updated_at: string
}

export interface CreateBSIModelingInput {
  asset_id: string
  control_id: string
  priority: 'R1' | 'R2' | 'R3'
  justification_for_exclusion?: string
  check_status?: 'yes' | 'partial' | 'no' | 'not_applicable'
  interview_notes?: string
  site_visit_notes?: string
}

export interface UpdateBSIModelingInput {
  priority: 'R1' | 'R2' | 'R3'
  justification_for_exclusion?: string
  check_status?: 'yes' | 'partial' | 'no' | 'not_applicable'
  interview_notes?: string
  site_visit_notes?: string
}

export interface BSIModelingStats {
  total: number
  count_yes: number
  count_partial: number
  count_no: number
  count_na: number
  count_pending: number
}

// ── S61-2: Management Review ─────────────────────────────────────────────────

export interface ImprovementDecision {
  decision: string
  responsible: string
  due_date: string
}

export interface ManagementReview {
  id: string
  org_id: string
  review_date: string
  review_type: 'annual' | 'extraordinary'
  participant_ids: string[]
  status: 'draft' | 'approved'
  audit_findings_summary: string
  incident_summary: string
  risk_status_summary: string
  previous_actions_status: string
  kpi_snapshot?: Record<string, unknown>
  context_changes: string
  customer_feedback: string
  improvement_decisions: ImprovementDecision[]
  resource_decisions: string
  isms_changes: string
  next_review_date?: string
  approved_by?: string
  approved_at?: string
  created_by: string
  created_at: string
  updated_at: string
}

export interface CreateManagementReviewInput {
  review_date: string
  review_type: 'annual' | 'extraordinary'
  participant_ids?: string[]
}

export interface UpdateManagementReviewInputsInput {
  audit_findings_summary: string
  incident_summary: string
  risk_status_summary: string
  previous_actions_status: string
  kpi_snapshot?: Record<string, unknown>
  context_changes: string
  customer_feedback: string
}

export interface UpdateManagementReviewOutputsInput {
  improvement_decisions: ImprovementDecision[]
  resource_decisions: string
  isms_changes: string
  next_review_date?: string
}

// ── S61-7: ISMS KPI Dashboard ─────────────────────────────────────────────────

export interface KPISnapshot {
  id: string
  org_id: string
  snapshot_date: string
  kpi_compliance_score?: number
  kpi_open_critical_controls?: number
  kpi_open_high_risks?: number
  kpi_residual_risk_avg?: number
  kpi_open_incidents?: number
  kpi_incident_mttr_days?: number
  kpi_evidence_coverage?: number
  kpi_expiring_evidence_count?: number
  kpi_finding_sla_compliance?: number
  kpi_open_major_ncs?: number
  kpi_suppliers_overdue_pct?: number
  kpi_phishing_click_rate?: number
  created_at: string
}

export interface KPIDashboard {
  current?: KPISnapshot
  history: KPISnapshot[]
}

// ── S69-1: Cross-Framework Mapping Coverage ───────────────────────────────────

export interface FrameworkPairCoverage {
  framework_a_name: string
  framework_b_name: string
  mapping_count: number
  is_mapped: boolean
}

export interface MappingCoverageResponse {
  pairs: FrameworkPairCoverage[]
  total_meaningful_pairs: number
  mapped_pairs: number
  coverage_pct: number
}

export interface PrereqRef {
  framework: string
  control_code: string
  dependency_type: string
}

export interface ImplementationStep {
  step_nr: number
  framework_id: string
  control_code: string
  control_title: string
  current_status: string
  prerequisites_met: boolean
  blocking_prereqs: PrereqRef[]
}

// ── S74: BSI IT-Grundschutz-Check ────────────────────────────────────────────

export type BSITargetObjectType = 'it_system' | 'application' | 'network' | 'room' | 'process'
export type BSIAbsicherungsniveau = 'basis' | 'standard' | 'kern'
export type BSISchutzbedarf = 'normal' | 'hoch' | 'sehr_hoch'
export type BSIUmsetzungsstatus = 'ja' | 'teilweise' | 'nein' | 'entbehrlich'
export type BSIEintrittshaeufigkeit = 'selten' | 'mittel' | 'haeufig' | 'sehr_haeufig'
export type BSISchadensauswirkung = 'vernachlaessigbar' | 'begrenzt' | 'betraechtlich' | 'existenzbedrohend'
export type BSIRisikokategorie = 'gering' | 'mittel' | 'hoch' | 'sehr_hoch'
export type BSIBehandlungsoption = 'reduzieren' | 'akzeptieren' | 'vermeiden' | 'transferieren'
export type BSIReportType = 'A1' | 'A2' | 'A3' | 'A4' | 'A5' | 'A6' | 'full'

export interface BSITargetObject {
  id: string
  org_id: string
  name: string
  description?: string
  type: BSITargetObjectType
  absicherungsniveau: BSIAbsicherungsniveau
  protection_c?: BSISchutzbedarf
  protection_i?: BSISchutzbedarf
  protection_a?: BSISchutzbedarf
  created_at: string
  updated_at: string
}

export interface CreateBSITargetObjectInput {
  name: string
  description?: string
  type: BSITargetObjectType
  absicherungsniveau?: BSIAbsicherungsniveau
  protection_c?: BSISchutzbedarf
  protection_i?: BSISchutzbedarf
  protection_a?: BSISchutzbedarf
}

export interface BSICheckResult {
  id: string
  org_id: string
  target_object_id: string
  anforderung_id: string
  anforderung_title: string
  baustein_id: string
  umsetzungsstatus: BSIUmsetzungsstatus
  begruendung?: string
  verantwortlicher?: string
  updated_at: string
}

export interface SetCheckResultInput {
  umsetzungsstatus: BSIUmsetzungsstatus
  begruendung?: string
  verantwortlicher?: string
}

export interface BSICheckSummary {
  target_object_id: string
  total: number
  ja: number
  teilweise: number
  entbehrlich: number
  nein: number
  umsetzungsgrad_pct: number
}

export interface BSIHeatmapCell {
  target_object_id: string
  target_object_name: string
  pct: number
}

export interface BSIHeatmapRow {
  baustein_id: string
  baustein_title: string
  cells: BSIHeatmapCell[]
}

export interface BSITopGap {
  anforderung_id: string
  anforderung_title: string
  baustein_id: string
  affected_objects: number
}

export interface BSICockpit {
  org_id: string
  overall_pct: number
  heatmap: BSIHeatmapRow[]
  top_gaps: BSITopGap[]
}

export interface BSIGapDetail {
  baustein_id: string
  anforderung_id: string
  anforderung_title: string
  zielobjekt: string
  umsetzungsstatus: string
}

export interface BSIGapReport {
  org_id: string
  generated_at: string
  gaps: BSIGapDetail[]
}

export interface BSIThreat {
  id: string
  threat_id: string
  title: string
  category: string
  description?: string
}

export interface BSIRiskAssessment {
  id: string
  org_id: string
  target_object_id: string
  threat_id: string
  threat_title?: string
  eintrittshaeufigkeit: BSIEintrittshaeufigkeit
  schadensauswirkung: BSISchadensauswirkung
  risikokategorie: BSIRisikokategorie
  behandlungsoption?: BSIBehandlungsoption
  massnahme: string
  verantwortlicher: string
  zieldatum?: string
  restrisiko?: BSIRisikokategorie
  created_at: string
  updated_at: string
}

export interface CreateBSIRiskInput {
  threat_id: string
  eintrittshaeufigkeit: BSIEintrittshaeufigkeit
  schadensauswirkung: BSISchadensauswirkung
}

export interface UpdateBSIRiskInput {
  eintrittshaeufigkeit?: BSIEintrittshaeufigkeit
  schadensauswirkung?: BSISchadensauswirkung
  behandlungsoption?: BSIBehandlungsoption
  massnahme?: string
  verantwortlicher?: string
  zieldatum?: string
  restrisiko?: BSIRisikokategorie
}

export interface BSIRiskSummary {
  gering: number
  mittel: number
  hoch: number
  sehr_hoch: number
  offen: number
}

export interface BSIReportExport {
  id: string
  org_id: string
  report_type: BSIReportType
  generated_by: string
  sha256: string
  created_at: string
}
