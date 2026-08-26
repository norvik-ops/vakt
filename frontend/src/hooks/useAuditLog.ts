import { useQuery } from '@tanstack/react-query'
import { apiFetch } from '../api/client'
import { fetchAllPages } from '../api/fetchAllPages'

export interface AuditLogEntry {
  id: string
  org_id: string
  user_id?: string
  user_email?: string
  action: string
  resource_type: string
  resource_id?: string
  resource_name?: string
  details?: Record<string, string>
  ip_address?: string
  created_at: string
}

export interface AuditLogResult {
  entries: AuditLogEntry[]
  total: number
}

export interface AuditLogFilters {
  /** Max entries to return (default 25, max 500) */
  limit?: number
  /** Entries to skip — drives server-side pagination */
  offset?: number
  /** RFC3339 timestamp — filter created_at >= from */
  from?: string
  /** RFC3339 timestamp — filter created_at <= to */
  to?: string
  /** Substring match on user_email (case-insensitive) */
  userEmail?: string
  /** Exact match on action field */
  action?: string
}

function buildQuery(filters: AuditLogFilters): string {
  const params = new URLSearchParams()

  if (filters.limit)     params.set('limit',      String(filters.limit))
  if (filters.offset)    params.set('offset',     String(filters.offset))
  if (filters.from)      params.set('from',       filters.from)
  if (filters.to)        params.set('to',         filters.to)
  if (filters.userEmail) params.set('user_email', filters.userEmail)
  if (filters.action)    params.set('action',     filters.action)

  const qs = params.toString()
  return qs ? `?${qs}` : ''
}

export function useAuditLog(filters: AuditLogFilters = {}) {
  return useQuery<AuditLogResult>({
    queryKey: ['audit-log', filters],
    queryFn: () => apiFetch<AuditLogResult>(`/audit-log${buildQuery(filters)}`),
    staleTime: 30_000,
  })
}

/** Largest page the endpoint will serve — audit/query.go:50-52 caps here. */
const SERVER_MAX_LIMIT = 500

/**
 * R1-20-A7 — fetch EVERY entry matching `filters`, not just the page on screen.
 *
 * The CSV export used to serialise the 25 rows the table happened to be
 * holding, with nothing anywhere saying so. An auditor who took that file as
 * evidence of an 81-entry log got a third of it and no reason to doubt the
 * rest — which is the worst shape a wrong number can take, because it is
 * indistinguishable from a right one.
 *
 * It walks /audit-log rather than /admin/audit-logs?format=csv on purpose,
 * even though the latter already emits CSV server-side. That route filters by
 * user_id, not user_email, and knows no from/to at all (admin/handler.go:39-44),
 * so pointing the button at it would produce a complete export of the WRONG
 * rows — the date range the user had just set would be dropped without notice.
 * Trading a visible truncation for an invisible one is not a fix.
 *
 * `complete` comes back with the rows so the caller can tell the user when the
 * cap did bite, instead of repeating the original mistake at a larger number.
 */
export async function fetchAllAuditLogEntries(
  filters: AuditLogFilters = {},
  maxItems = 50_000,
): Promise<{ entries: AuditLogEntry[]; total: number; complete: boolean }> {
  const res = await fetchAllPages<AuditLogEntry>(async (page, limit) => {
    const body = await apiFetch<AuditLogResult>(
      `/audit-log${buildQuery({ ...filters, limit, offset: (page - 1) * limit })}`,
    )
    return { items: body.entries, total: body.total }
  }, { pageSize: SERVER_MAX_LIMIT, maxItems })

  return { entries: res.items, total: res.total, complete: res.complete }
}
