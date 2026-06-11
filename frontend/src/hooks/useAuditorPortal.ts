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

export function useAuditorControls(frameworkId: string | null, token: string | null) {
  return useQuery<AuditorControl[]>({
    queryKey: ['auditor-portal', 'controls', frameworkId, token],
    queryFn: () => auditorFetch<AuditorControl[]>(`/frameworks/${frameworkId ?? ''}/controls`, token ?? ''),
    enabled: !!token && !!frameworkId,
    retry: false,
  })
}

export function useAuditorRisks(token: string | null) {
  return useQuery<{ data: AuditorRisk[]; total: number }>({
    queryKey: ['auditor-portal', 'risks', token],
    queryFn: () => auditorFetch<{ data: AuditorRisk[]; total: number }>('/risks', token ?? ''),
    enabled: !!token,
    retry: false,
  })
}

export function useAuditorIncidents(token: string | null) {
  return useQuery<{ data: AuditorIncident[]; total: number }>({
    queryKey: ['auditor-portal', 'incidents', token],
    queryFn: () => auditorFetch<{ data: AuditorIncident[]; total: number }>('/incidents', token ?? ''),
    enabled: !!token,
    retry: false,
  })
}

export function useAuditorPolicies(token: string | null) {
  return useQuery<{ data: AuditorPolicy[]; total: number }>({
    queryKey: ['auditor-portal', 'policies', token],
    queryFn: () => auditorFetch<{ data: AuditorPolicy[]; total: number }>('/policies', token ?? ''),
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
