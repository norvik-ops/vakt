export interface Asset {
  id: string
  name: string
  type: 'web_app' | 'server' | 'database' | 'container' | 'repo'
  target: string
  criticality: 'low' | 'medium' | 'high' | 'critical'
  tags: string[]
  org_id: string
  created_at: string
}

export interface Finding {
  id: string
  asset_id: string
  asset_name?: string
  title: string
  severity: 'info' | 'low' | 'medium' | 'high' | 'critical'
  status: 'open' | 'in_progress' | 'accepted_risk' | 'false_positive' | 'resolved'
  cve_id?: string
  cvss_score?: number
  description: string
  notes?: string
  assigned_to?: string
  sla_due_at?: string
  created_at: string
  updated_at: string
}

export interface Report {
  id: string
  title: string
  status: 'pending' | 'processing' | 'completed' | 'failed'
  created_at: string
  expires_at?: string
}

export interface SLAEntry {
  asset_id: string
  asset_name: string
  finding_id: string
  finding_title: string
  severity: 'info' | 'low' | 'medium' | 'high' | 'critical'
  status: string
  days_open: number
  sla_days: number
  overdue: boolean
}

export interface FindingsListResponse {
  data: Finding[]
  pagination: {
    page: number
    limit: number
    total: number
    total_pages: number
  }
}

export interface RiskTrendPoint {
  date: string
  total_risk_score: number
  open_count: number
  critical_count: number
}

export type RiskTrendResponse = RiskTrendPoint[]
