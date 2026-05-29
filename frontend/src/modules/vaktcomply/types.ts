export interface Framework {
  id: string
  name: string
  version: string
  created_at: string
  control_count?: number
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
  created_at: string
  updated_at: string
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
  created_at: string
  updated_at: string
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
