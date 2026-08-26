import { useQuery } from '@tanstack/react-query'

// ---------------------------------------------------------------------------
// Types
// ---------------------------------------------------------------------------

export interface AuditorFramework {
  id: string
  name: string
  version: string
  is_builtin: boolean
  readiness_score: number
  created_at: string
}

export interface AuditorControl {
  id: string
  control_id: string
  title: string
  description: string
  domain: string
  status: string
  manual_status: string
}

export interface AuditorRisk {
  id: string
  title: string
  description: string
  likelihood: number
  impact: number
  treatment_status: string
  created_at: string
}

export interface AuditorIncident {
  id: string
  title: string
  description: string
  severity: string
  status: string
  created_at: string
}

export interface AuditorPolicy {
  id: string
  title: string
  category: string
  status: string
  created_at: string
}

// Shape rules for these endpoints — read the handler, do not generalise:
//
//   * MOST list endpoints wrap their rows in a pagination envelope
//     (`pagination.Wrap` / `pagination.WrapCursor` on the Go side). Typing one
//     of those as `T[]` is not merely inaccurate: the envelope is truthy, so a
//     `= []` / `?? []` default never applies and `.map()` throws (R1-11-D01).
//   * BUT NOT ALL. `ListFrameworks` returns `c.JSON(200, frameworks)` — a bare
//     array — so `useAuditorFrameworks` below is correctly typed `T[]`.
//     `ListAuditRecords` is bare too (no hook here). There is no blanket rule.
//   * Which META a wrapper carries depends on the handler's branching:
//     `ListControls` and `ListRisks` are dual-branch and default to the CURSOR
//     branch when no `page` param is sent (these hooks never send one), so they
//     carry CursorMeta. `ListIncidents`/`ListPolicies` wrap unconditionally with
//     offset metadata, so they carry OffsetMeta.
export interface OffsetMeta {
  page: number
  limit: number
  total: number
  total_pages: number
}

export interface CursorMeta {
  limit: number
  next_cursor?: string
  has_more: boolean
}

export interface Paginated<T, M = OffsetMeta> {
  data: T[]
  pagination: M
}

// ---------------------------------------------------------------------------
// Fetch helper — uses auditor session token instead of user Paseto token
// ---------------------------------------------------------------------------

async function auditorFetch<T>(path: string, token: string): Promise<T> {
  const res = await fetch(`/api/v1/auditor/vaktcomply${path}`, {
    headers: { Authorization: `Bearer ${token}` },
  })
  if (!res.ok) {
    if (res.status === 401) throw new Error('AUDITOR_UNAUTHORIZED')
    throw new Error(`AUDITOR_FETCH_FAILED:${res.status}`)
  }
  return res.json() as Promise<T>
}

// ---------------------------------------------------------------------------
// Hooks
// ---------------------------------------------------------------------------

export function useAuditorFrameworks(token: string | null) {
  return useQuery<AuditorFramework[]>({
    queryKey: ['auditor-portal', 'frameworks', token],
    queryFn: () => auditorFetch<AuditorFramework[]>('/frameworks', token ?? ''),
    enabled: !!token,
    retry: false,
  })
}

// Dual-branch handler, cursor branch is the default (no `?page=` sent here).
export function useAuditorControls(frameworkId: string | null, token: string | null) {
  return useQuery<Paginated<AuditorControl, CursorMeta>>({
    queryKey: ['auditor-portal', 'controls', frameworkId, token],
    queryFn: () =>
      auditorFetch<Paginated<AuditorControl, CursorMeta>>(
        `/frameworks/${frameworkId ?? ''}/controls`,
        token ?? '',
      ),
    enabled: !!token && !!frameworkId,
    retry: false,
  })
}

// ListRisks (handler_risks.go:65) is dual-branch exactly like ListControls —
// no `?page=` is sent here, so the CURSOR branch runs and the envelope carries
// CursorMeta. Typing it OffsetMeta would promise a `pagination.total` that is
// `undefined` at runtime: the same truthy-but-wrong-shape class this file fixes.
export function useAuditorRisks(token: string | null) {
  return useQuery<Paginated<AuditorRisk, CursorMeta>>({
    queryKey: ['auditor-portal', 'risks', token],
    queryFn: () => auditorFetch<Paginated<AuditorRisk, CursorMeta>>('/risks', token ?? ''),
    enabled: !!token,
    retry: false,
  })
}

// Unconditional offset wrapper (handler_ops.go:832) — OffsetMeta is correct.
export function useAuditorIncidents(token: string | null) {
  return useQuery<Paginated<AuditorIncident>>({
    queryKey: ['auditor-portal', 'incidents', token],
    queryFn: () => auditorFetch<Paginated<AuditorIncident>>('/incidents', token ?? ''),
    enabled: !!token,
    retry: false,
  })
}

// Unconditional offset wrapper (handler_reporting.go:502) — OffsetMeta is correct.
export function useAuditorPolicies(token: string | null) {
  return useQuery<Paginated<AuditorPolicy>>({
    queryKey: ['auditor-portal', 'policies', token],
    queryFn: () => auditorFetch<Paginated<AuditorPolicy>>('/policies', token ?? ''),
    enabled: !!token,
    retry: false,
  })
}

export function downloadAuditorZip(token: string) {
  const a = document.createElement('a')
  a.href = '#'
  document.body.appendChild(a)
  void fetch('/api/v1/auditor/vaktcomply/export.zip', {
    headers: { Authorization: `Bearer ${token}` },
  })
    .then((r) => r.blob())
    .then((blob) => {
      const url = URL.createObjectURL(blob)
      a.href = url
      a.download = 'vakt-audit-export.zip'
      a.click()
      URL.revokeObjectURL(url)
      a.remove()
    })
    .catch(() => { a.remove() })
}

export function downloadAuditorFrameworkPDF(token: string, frameworkId: string, frameworkName?: string) {
  const a = document.createElement('a')
  a.href = '#'
  document.body.appendChild(a)
  void fetch(`/api/v1/auditor/vaktcomply/frameworks/${frameworkId}/report.pdf`, {
    headers: { Authorization: `Bearer ${token}` },
  })
    .then((r) => {
      if (!r.ok) throw new Error('PDF_FAILED')
      return r.blob()
    })
    .then((blob) => {
      const url = URL.createObjectURL(blob)
      a.href = url
      a.download = frameworkName ? `${frameworkName} Compliance.pdf` : `framework-${frameworkId.slice(0, 8)}.pdf`
      a.click()
      URL.revokeObjectURL(url)
      a.remove()
    })
    .catch(() => { a.remove() })
}
