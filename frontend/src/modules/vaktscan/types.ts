export type ClassificationLevel = 'public' | 'internal' | 'confidential' | 'restricted'

export interface Asset {
  id: string
  name: string
  type: 'web_app' | 'server' | 'database' | 'container' | 'repo'
  target: string
  criticality: 'low' | 'medium' | 'high' | 'critical'
  tags: string[]
  org_id: string
  protection_need_id?: string | null
  classification?: ClassificationLevel
  created_at: string
}

export interface ClassificationSummary {
  total_count: number
  classified_count: number
  unclassified_count: number
  by_level: Record<ClassificationLevel, number>
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

export interface Certificate {
  id: string
  org_id: string
  domain: string
  issuer: string
  subject: string
  sans: string[]
  not_before?: string | null
  not_after?: string | null
  asset_id?: string | null
  source: 'manual' | 'scan'
  status: 'valid' | 'expiring' | 'expired' | 'error' | 'unknown'
  last_checked_at?: string | null
  error_msg?: string | null
  created_at: string
  updated_at: string
}

// ── S69-3: SLA Policies ───────────────────────────────────────────────────────

export interface SLAPolicy {
  id: string
  org_id: string
  severity: string
  remediation_days: number
  notification_advance_days: number
  is_default: boolean
  created_at: string
  updated_at: string
}

export interface SLASummaryFE {
  total_open: number
  overdue: number
  at_risk: number
  on_track: number
  by_severity: Record<string, number>
  overdue_by_severity: Record<string, number>
}
