/** A SecVault project — the top-level namespace that groups environments and secrets. */
export interface Project {
  id: string
  name: string
  description?: string
  /** Organisation this project belongs to; used for multi-tenant RBAC enforcement. */
  org_id: string
  created_at: string
}

/** A named environment within a project (e.g. `production`, `staging`). */
export interface Environment {
  id: string
  project_id: string
  name: string
  created_at: string
}

/**
 * A decrypted key/value secret pair.
 * Values are never persisted in plaintext — this shape is only returned
 * from the API after AES-256-GCM decryption server-side.
 */
export interface Secret {
  key: string
  value: string
}

/**
 * Metadata for an async Git repository scan for leaked credentials.
 *
 * K5-11: mirrors vaktvault.GitScan (backend/internal/modules/vaktvault/models.go).
 * The previous shape declared `result_count`, `started_at` and `completed_at` —
 * none of which the backend emits — so the red "N findings" badge could never
 * appear, no matter how many cleartext secrets a scan turned up.
 */
export interface GitScan {
  id: string
  org_id: string
  repo_url: string
  branch: string
  /** Current scan lifecycle state; transitions from `pending` → `running` → `completed` | `failed`. */
  status: 'pending' | 'running' | 'completed' | 'failed'
  /** Total number of potential secrets found. */
  finding_count: number
  open_count: number
  dismissed_count: number
  error_message?: string
  scanned_at?: string
  created_at: string
}

/**
 * A single leaked-credential finding from a Git repository scan.
 *
 * K5-12: mirrors vaktvault.ScanResult. `dismissed` (a boolean the backend never
 * sends) used to decide the active/dismissed split — `!undefined` is `true`, so
 * every finding stayed in the red block and Dismiss looked like a no-op. The real
 * discriminator is `status`.
 */
export interface ScanResult {
  id: string
  org_id: string
  scan_id: string
  repo_url: string
  commit_hash?: string
  file_path: string
  line_number: number
  /** Classifier label for the detected secret (e.g. `"AWS_ACCESS_KEY"`, `"GH_TOKEN"`). */
  pattern_name: string
  /** Redacted preview of the match (first4…last4); never contains the full secret. */
  match_preview: string
  severity: string
  /** `open` | `dismissed` — `dismissed` is what the Dismiss endpoint persists. */
  status: string
  dismiss_reason?: string
  dismiss_count: number
  created_at: string
}

/** A programmatic API access token scoped to specific SecVault operations. */
export interface AccessToken {
  id: string
  name: string
  /** List of permission scopes granted to this token (e.g. `["secrets:read"]`). */
  scopes: string[]
  last_used_at?: string
  created_at: string
}

/** Aggregated health summary for a single SecVault project. */
export interface ProjectHealth {
  /** Health score from 0–100; deductions applied for exposed secrets and failed scans. */
  score: number
  /** Human-readable descriptions of the issues that reduced the score. */
  issues: string[]
}

/** A single audit-log event recording one secret access within a project. */
export interface ProjectAccessLogEntry {
  id: string
  /** The secret key that was accessed (never the value). */
  secret_key: string
  /** Access method: `"api"`, `"ui"`, `"cli"`, or `"ci"`. */
  access_via: string
  /** Username or token name of the accessor; absent for unauthenticated or anonymous access. */
  accessed_by?: string
  ip_address?: string
  /** RFC 3339 UTC timestamp of the access event. */
  accessed_at: string
}

/**
 * Paginated response envelope for the project access log.
 * Consumers must include both `page` and `limit` in the query key so that
 * different pages are cached independently and do not overwrite each other.
 */
export interface AccessLogPage {
  /** Slice of log entries for the requested page. */
  entries: ProjectAccessLogEntry[]
  /** Total number of entries across all pages — used to compute page count in the UI. */
  total: number
  /** 1-based page number that was returned. */
  page: number
  /** Maximum number of entries per page as requested by the caller. */
  limit: number
}

// S70-5: Vault Access Review (quarterly)
export interface ReviewDecision {
  env_id: string
  secret_key: string
  action: 'keep' | 'revoke'
}

export interface AccessReview {
  id: string
  org_id: string
  period_label: string
  status: 'open' | 'completed'
  reviewed_by?: string
  completed_at?: string
  total_entries: number
  stale_entries: number
  revoked_entries: number
  created_at: string
}

export interface AccessReviewItem {
  secret_key: string
  env_id: string
  project_name?: string
  last_accessed_at?: string
  is_stale: boolean
}

export interface AccessReviewDetail extends AccessReview {
  notes?: string
  items: AccessReviewItem[]
}
