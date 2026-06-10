export interface VVTEntry {
  id: string
  org_id: string
  name: string
  purpose: string
  legal_basis: string
  data_categories: string[]
  data_subjects: string[]
  recipients: string[]
  retention_period?: string
  third_country_transfer: boolean
  safeguards?: string
  responsible_person?: string
  status: 'active' | 'archived'
  created_at: string
  updated_at: string
}

export interface CreateVVTInput {
  name: string
  purpose: string
  legal_basis: string
  data_categories: string[]
  data_subjects: string[]
  recipients: string[]
  retention_period?: string
  third_country_transfer: boolean
  safeguards?: string
  responsible_person?: string
}

export interface DPIA {
  id: string
  org_id: string
  vvt_entry_id?: string
  title: string
  description?: string
  necessity_assessment?: string
  risk_assessment?: string
  mitigation_measures?: string
  residual_risk?: string
  dpo_consultation: boolean
  status: 'draft' | 'in_review' | 'approved'
  reviewed_by?: string
  reviewed_at?: string
  created_at: string
  updated_at: string
}

export interface CreateDPIAInput {
  vvt_entry_id?: string
  title: string
  description?: string
  necessity_assessment?: string
  risk_assessment?: string
  mitigation_measures?: string
  residual_risk?: string
  dpo_consultation: boolean
}

export interface AVV {
  id: string
  org_id: string
  processor_name: string
  service_description: string
  contract_date?: string
  review_date?: string
  status: 'active' | 'expired' | 'terminated'
  notes?: string
  template_id?: string
  body?: string
  scc_module?: 'module_1' | 'module_2' | 'module_3' | 'module_4'
  scc_annex_i?: string
  scc_annex_ii?: string
  scc_annex_iii?: string
  created_at: string
  updated_at: string
}

export interface AVVTemplate {
  id: string
  title: string
  description: string
  body: string
  variables: string[]
}

export interface SCCModule {
  id: string
  title: string
  description: string
}

export interface CreateAVVFromTemplateInput {
  template_id: string
  vars: Record<string, string>
}

export interface UpdateAVVSCCInput {
  scc_module?: 'module_1' | 'module_2' | 'module_3' | 'module_4'
  annex_i?: string
  annex_ii?: string
  annex_iii?: string
}

export interface CreateAVVInput {
  processor_name: string
  service_description: string
  contract_date?: string
  review_date?: string
  notes?: string
}

export interface Breach {
  id: string
  org_id: string
  title: string
  description: string
  discovered_at: string
  authority_deadline_at: string
  authority_notified_at?: string
  subjects_notification_required: boolean
  subjects_notified_at?: string
  affected_count?: number
  data_categories: string[]
  status: 'open' | 'authority_notified' | 'closed'
  created_at: string
  updated_at: string
}

export interface CreateBreachInput {
  title: string
  description: string
  discovered_at: string
  subjects_notification_required: boolean
  affected_count?: number
  data_categories: string[]
}

export interface UpdateVVTInput extends CreateVVTInput {
  status: 'active' | 'archived'
}

export interface UpdateDPIAInput {
  title: string
  description?: string
  necessity_assessment?: string
  risk_assessment?: string
  mitigation_measures?: string
  residual_risk?: string
  dpo_consultation: boolean
}

export interface UpdateAVVInput {
  processor_name: string
  service_description: string
  contract_date?: string
  review_date?: string
  status: 'active' | 'expired' | 'terminated'
  notes?: string
}

export interface UpdateBreachInput {
  title: string
  description: string
  subjects_notification_required: boolean
  affected_count?: number
  data_categories: string[]
}

export type DSRType = 'access' | 'erasure' | 'portability' | 'objection' | 'rectification' | 'restriction' | 'no_profiling'
export type DSRStatus = 'open' | 'in_progress' | 'completed' | 'rejected' | 'extended' | 'overdue'

export interface DSR {
  id: string
  org_id: string
  requester_name: string
  requester_email: string
  type: DSRType
  description?: string
  status: DSRStatus
  due_date?: string
  received_at: string
  completed_at?: string
  notes?: string
  channel?: string
  reference_id?: string
  extension_due_at?: string
  extension_reason?: string
  assigned_to?: string
  resolved_by?: string
  created_at: string
  updated_at: string
}

export interface DSRSummary {
  open_count: number
  overdue_count: number
  fulfilled_last_12m: number
  rejected_last_12m: number
  on_time_rate_pct: number
}

export interface CreateDSRInput {
  requester_name: string
  requester_email: string
  type: DSRType
  description?: string
  notes?: string
  channel?: string
  reference_id?: string
}

export interface UpdateDSRInput {
  status: DSRStatus
  notes?: string
}

export interface ResolveDSRInput {
  resolution_type: DSRStatus
  resolution_notes?: string
  extension_reason?: string
}

// ── S69-6: Transfer Impact Assessment ────────────────────────────────────────

export interface AdequacyDecision {
  country_code: string
  country_name: string
  has_adequacy: boolean
  decision_date?: string
  decision_reference?: string
  notes?: string
  last_updated: string
}

export interface DataTransfer {
  id: string
  org_id: string
  processing_activity_id?: string
  recipient_name: string
  recipient_country: string
  recipient_country_name: string
  data_categories: string[]
  transfer_mechanism: string
  scc_version?: string
  status: string
  is_active: boolean
  created_at: string
  updated_at: string
}

export interface TransferImpactAssessment {
  id: string
  org_id: string
  transfer_id: string
  legal_system_notes: string
  surveillance_risk: string
  data_subject_rights_available: boolean
  encryption_in_transit: boolean
  encryption_at_rest: boolean
  pseudonymization_applied: boolean
  access_controls_documented: boolean
  supplementary_measures?: string
  outcome: string
  reviewed_by?: string
  reviewed_at?: string
  valid_until?: string
  created_at: string
}

export interface TransferComplianceStatus {
  total_transfers: number
  adequate: number
  requires_tia: number
  tia_adequate: number
  tia_adequate_with_measures: number
  tia_inadequate: number
  under_review: number
}
