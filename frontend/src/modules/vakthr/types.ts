export interface Employee {
  id: string
  org_id: string
  first_name: string
  last_name: string
  email: string
  department?: string
  role?: string
  status: 'active' | 'offboarding' | 'terminated'
  start_date?: string
  end_date?: string
  notes?: string
  created_at: string
  updated_at: string
}

export interface CreateEmployeeInput {
  first_name: string
  last_name: string
  email: string
  department?: string
  role?: string
  start_date?: string
  notes?: string
}

export interface UpdateEmployeeInput {
  first_name: string
  last_name: string
  department?: string
  role?: string
  end_date?: string
  status: 'active' | 'offboarding' | 'terminated'
  notes?: string
}

export interface ChecklistItem {
  id: string
  label: string
  required: boolean
}

export interface Checklist {
  id: string
  org_id: string
  type: 'onboarding' | 'offboarding'
  name: string
  items: ChecklistItem[]
  created_at: string
  updated_at: string
}

export interface CreateChecklistInput {
  type: 'onboarding' | 'offboarding'
  name: string
  items: ChecklistItem[]
}

export interface ChecklistRun {
  id: string
  org_id: string
  employee_id: string
  checklist_id: string
  status: 'in_progress' | 'completed'
  completed_items: string[]
  started_at: string
  completed_at?: string
  created_at: string
  updated_at: string
}

export interface StartChecklistRunInput {
  employee_id: string
  checklist_id: string
}

export interface UpdateChecklistRunInput {
  completed_items: string[]
  status: 'in_progress' | 'completed'
}

// --- Berechtigungskonzept (Migration 158) ---

export type AccessLevel = 'read' | 'write' | 'admin' | 'no_access'

export interface AccessRole {
  id: string
  concept_id: string
  org_id: string
  role_name: string
  system_name: string
  access_level: AccessLevel
  justification: string
  review_interval_months: number
  created_at: string
  updated_at: string
}

export interface AccessConcept {
  id: string
  org_id: string
  title: string
  scope: string
  owner: string
  current_version: number
  roles?: AccessRole[]
  created_at: string
  updated_at: string
}

export interface CreateAccessConceptInput {
  title: string
  scope?: string
  owner?: string
}

export interface UpdateAccessConceptInput {
  title: string
  scope?: string
  owner?: string
}

export interface CreateAccessRoleInput {
  role_name: string
  system_name: string
  access_level: AccessLevel
  justification?: string
  review_interval_months?: number
}

export type UpdateAccessRoleInput = CreateAccessRoleInput

export interface AccessConceptVersionSummary {
  id: string
  concept_id: string
  version_number: number
  created_at: string
}
