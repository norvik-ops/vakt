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
