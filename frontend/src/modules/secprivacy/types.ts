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

export type DSRType = 'access' | 'erasure' | 'portability' | 'objection' | 'rectification'
export type DSRStatus = 'open' | 'in_progress' | 'completed' | 'rejected'

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
  created_at: string
  updated_at: string
}

export interface CreateDSRInput {
  requester_name: string
  requester_email: string
  type: DSRType
  description?: string
  notes?: string
}

export interface UpdateDSRInput {
  status: DSRStatus
  notes?: string
}
