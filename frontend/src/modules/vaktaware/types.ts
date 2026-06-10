export interface Campaign {
  id: string
  name: string
  status: 'draft' | 'scheduled' | 'running' | 'completed' | 'aborted'
  template_id: string
  target_group_id: string
  from_name: string
  from_email: string
  subject: string
  scheduled_at?: string
  track_opens: boolean
  betriebsrat_mode: boolean
  created_at: string
}

export interface CreateCampaignInput {
  name: string
  template_id: string
  target_group_id: string
  from_name: string
  from_email: string
  subject: string
  scheduled_at?: string
  track_opens?: boolean
  betriebsrat_mode?: boolean
}

export interface CampaignStats {
  campaign_id: string
  total_targets: number
  emails_sent: number
  open_rate: number
  click_rate: number
  submission_rate: number
}

export interface Template {
  id: string
  name: string
  subject: string
  from_name: string
  from_email: string
  html_body: string
  attack_type: string
  category?: string
  difficulty?: 'easy' | 'medium' | 'hard'
  language?: string
  placeholders?: string[]
  is_preset: boolean
  created_at: string
}

export interface EnrollmentRule {
  id: string
  org_id: string
  name: string
  trigger_type: 'new_employee' | 'phishing_click'
  target_campaign_id?: string
  is_active: boolean
  created_at: string
  updated_at: string
}

export interface CreateEnrollmentRuleInput {
  name: string
  trigger_type: 'new_employee' | 'phishing_click'
  target_campaign_id?: string
}

export interface CampaignSummary {
  id: string
  name: string
  recipient_count: number
  click_rate: number
  completion_rate: number
  started_at?: string
  completed_at?: string
}

export interface AwareStats {
  total_campaigns: number
  total_participants: number
  avg_click_rate: number
  total_trainings_completed: number
}

export interface ORP3Requirement {
  id: string
  title: string
  fulfilled: boolean
  evidence_ids?: string[]
}

export interface BSIOrp3Compliance {
  fulfilled_count: number
  total_count: number
  requirements: ORP3Requirement[]
}

export interface TrainingMatrixReport {
  period: { from: string; to: string }
  org_name: string
  campaigns: CampaignSummary[]
  total_stats: AwareStats
  bsi_compliance: BSIOrp3Compliance
  generated_at: string
}

export interface TargetGroup {
  id: string
  name: string
  source: string
  target_count?: number
  created_at: string
}

export interface Target {
  id: string
  email: string
  first_name?: string
  last_name?: string
  department?: string
}

export interface TrainingModule {
  id: string
  title: string
  description: string
  passing_score: number
  created_at: string
}

export interface Assignment {
  id: string
  module_id: string
  module_title?: string
  user_email: string
  status: 'assigned' | 'completed' | 'failed'
  assigned_at: string
  completed_at?: string
  score?: number
}

// --- Feature 5: Phish-Button ---

export interface PhishReport {
  id: string
  org_id: string
  campaign_id?: string
  reporter_email: string
  reported_at: string
  subject?: string
  sender?: string
  is_simulation: boolean
  created_at: string
}

export interface PhishReportStats {
  total: number
  simulations: number
  real_threats: number
}
