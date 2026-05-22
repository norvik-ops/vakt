// Centralised status/severity → visual mapping.
// Keeping this here avoids duplicating these records across 10+ pages.

// ── Finding severity ──────────────────────────────────────────────────────────

export type FindingSeverity = 'info' | 'low' | 'medium' | 'high' | 'critical'

/** CSS classes for severity badge-style chips (border + bg + text). */
export const findingSeverityClass: Record<FindingSeverity, string> = {
  info:     'bg-surface2 text-muted border-transparent',
  low:      'bg-severity-info-bg text-severity-info border-transparent',
  medium:   'bg-severity-medium-bg text-severity-medium border-transparent',
  high:     'bg-severity-high-bg text-severity-high border-transparent',
  critical: 'bg-severity-critical-bg text-severity-critical border-transparent',
}

/** shadcn Badge `variant` for severity. */
export const findingSeverityVariant: Record<FindingSeverity, 'secondary' | 'outline' | 'warning' | 'destructive'> = {
  info:     'secondary',
  low:      'outline',
  medium:   'warning',
  high:     'outline',
  critical: 'destructive',
}

/** Numeric sort weight — higher is more severe. */
export const findingSeverityOrder: Record<FindingSeverity, number> = {
  critical: 5, high: 4, medium: 3, low: 2, info: 1,
}

// ── Campaign status ───────────────────────────────────────────────────────────

export type CampaignStatus = 'draft' | 'scheduled' | 'running' | 'completed' | 'aborted'

export const campaignStatusVariant: Record<CampaignStatus, 'secondary' | 'default' | 'success' | 'destructive'> = {
  draft:     'secondary',
  scheduled: 'default',
  running:   'default',
  completed: 'success',
  aborted:   'destructive',
}

// ── Generic job / scan status ─────────────────────────────────────────────────
// Covers Report.status (pending/processing/completed/failed)
// and GitScan.status (pending/running/completed/failed).

export type JobStatus = 'pending' | 'processing' | 'running' | 'completed' | 'failed'

export const jobStatusVariant: Record<JobStatus, 'secondary' | 'default' | 'success' | 'destructive'> = {
  pending:    'secondary',
  processing: 'default',
  running:    'default',
  completed:  'success',
  failed:     'destructive',
}

// ── Assignment status (Vakt Aware) ────────────────────────────────────────────

export type AssignmentStatus = 'assigned' | 'completed' | 'failed'

export const assignmentStatusVariant: Record<AssignmentStatus, 'secondary' | 'success' | 'destructive'> = {
  assigned:  'secondary',
  completed: 'success',
  failed:    'destructive',
}
