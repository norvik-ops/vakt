import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { apiFetch } from '../../../api/client'
import type { Framework, ReadinessReport, GapAnalysis, Control } from '../types'
import type { CursorPaginatedResponse } from '../../../shared/types/pagination'

export function useTISAXReport(frameworkId: string) {
  return useQuery<ReadinessReport>({
    queryKey: ['vaktcomply', 'frameworks', frameworkId, 'report', 'tisax'],
    queryFn: () => apiFetch<ReadinessReport>(`/vaktcomply/frameworks/${frameworkId}/report`),
    enabled: !!frameworkId,
    staleTime: 5 * 60 * 1000,
  })
}

export function useFrameworks() {
  return useQuery<Framework[]>({
    queryKey: ['vaktcomply', 'frameworks'],
    queryFn: () => apiFetch<Framework[]>('/vaktcomply/frameworks'),
    staleTime: 5 * 60 * 1000,
  })
}

export function useFramework(frameworkId: string) {
  return useQuery<Framework>({
    queryKey: ['vaktcomply', 'frameworks', frameworkId],
    queryFn: () => apiFetch<Framework>(`/vaktcomply/frameworks/${frameworkId}`),
    enabled: !!frameworkId,
    staleTime: 5 * 60 * 1000,
  })
}

export function useEnableFramework() {
  const queryClient = useQueryClient()
  return useMutation<Framework, Error, { name: string; variant?: 'full' | 'simplified' }>({
    mutationFn: ({ name, variant }) =>
      apiFetch<Framework>(`/vaktcomply/frameworks/${name}/enable`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: variant ? JSON.stringify({ variant }) : undefined,
      }),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: ['vaktcomply', 'frameworks'] })
    },
  })
}

export function useSwitchDORAVariant() {
  const queryClient = useQueryClient()
  return useMutation<Framework, Error, 'full' | 'simplified'>({
    mutationFn: (variant) =>
      apiFetch<Framework>('/vaktcomply/frameworks/dora/variant', {
        method: 'PUT',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ variant }),
      }),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: ['vaktcomply', 'frameworks'] })
    },
  })
}

export function useDeleteFramework() {
  const queryClient = useQueryClient()
  return useMutation<undefined, Error, string>({
    mutationFn: (id: string) =>
      apiFetch<undefined>(`/vaktcomply/frameworks/${id}`, { method: 'DELETE' }),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: ['vaktcomply', 'frameworks'] })
    },
  })
}

export function useReadinessReport(frameworkId: string) {
  return useQuery<ReadinessReport>({
    queryKey: ['vaktcomply', 'frameworks', frameworkId, 'report'],
    queryFn: () => apiFetch<ReadinessReport>(`/vaktcomply/frameworks/${frameworkId}/report`),
    enabled: !!frameworkId,
    staleTime: 5 * 60 * 1000,
  })
}

export function useGapAnalysis(frameworkId: string) {
  return useQuery<GapAnalysis>({
    queryKey: ['vaktcomply', 'frameworks', frameworkId, 'gaps'],
    queryFn: () => apiFetch<GapAnalysis>(`/vaktcomply/frameworks/${frameworkId}/gaps`),
    enabled: !!frameworkId,
    staleTime: 5 * 60 * 1000,
  })
}

// Backend pagination.MaxLimit — larger values fall back to 25 on the server.
const CONTROLS_PAGE_SIZE = 100
const CONTROLS_MAX_PAGES = 50

/**
 * Loads ALL controls of a framework by following the cursor to the end.
 *
 * R1-KW-5: this used to fetch only page 1 with limit=25 and never paged on —
 * the framework detail showed 25 of 93 ISO controls, computed its tab
 * percentage on 25, and the control picker on a risk could not offer A.6–A.8.
 * Every consumer (framework detail, control picker, CIS/DORA pages) needs the
 * complete list, not a page: they group, count and search across it. A
 * framework has at most a few hundred controls, so loading them all in pages
 * of 100 stays small — and unlike a bigger magic limit it cannot silently cut
 * off again when a catalogue grows. The cursor branch is used because the
 * offset branch is deprecated (Sunset header) and reports no real total.
 */
async function fetchAllFrameworkControls(frameworkId: string): Promise<Control[]> {
  const all: Control[] = []
  let cursor = ''
  // Obergrenze: 50 × 100 = 5000 Kontrollen, weit über jedem Katalog. Ein Server,
  // der immer neue Cursor liefert, darf den Browser nicht endlos laden lassen.
  for (let pageNo = 1; ; pageNo++) {
    if (pageNo > CONTROLS_MAX_PAGES) {
      throw new Error(`controls pagination: more than ${String(CONTROLS_MAX_PAGES)} pages`)
    }
    const qs = new URLSearchParams({ limit: String(CONTROLS_PAGE_SIZE) })
    if (cursor) qs.set('cursor', cursor)
    const page = await apiFetch<CursorPaginatedResponse<Control>>(
      `/vaktcomply/frameworks/${frameworkId}/controls?${qs.toString()}`,
    )
    all.push(...page.data)
    const next = page.pagination.next_cursor ?? ''
    if (!page.pagination.has_more || !next) return all
    // A repeated cursor would loop forever — fail loudly instead.
    if (next === cursor) throw new Error('controls pagination: cursor did not advance')
    cursor = next
  }
}

export function useFrameworkControls(frameworkId: string) {
  return useQuery<Control[]>({
    queryKey: ['vaktcomply', 'frameworks', frameworkId, 'controls', 'all'],
    queryFn: () => fetchAllFrameworkControls(frameworkId),
    enabled: !!frameworkId,
    staleTime: 5 * 60 * 1000,
  })
}

export function useDownloadFrameworkPDF() {
  return (frameworkId: string, frameworkName?: string) => {
    void fetch(`/api/v1/vaktcomply/frameworks/${frameworkId}/export-pdf`, {
      credentials: 'include',
    })
      .then((r) => r.blob())
      .then((blob) => {
        const url = URL.createObjectURL(blob)
        const a = document.createElement('a')
        a.href = url
        a.download = frameworkName
          ? `${frameworkName} Compliance.pdf`
          : `framework-${frameworkId.slice(0, 8)}.pdf`
        document.body.appendChild(a)
        a.click()
        a.remove()
        URL.revokeObjectURL(url)
      })
  }
}

export function useDownloadSoAPDF() {
  return (frameworkId: string, frameworkName?: string) => {
    void fetch(`/api/v1/vaktcomply/frameworks/${frameworkId}/soa.pdf`, {
      credentials: 'include',
    })
      .then((r) => r.blob())
      .then((blob) => {
        const url = URL.createObjectURL(blob)
        const a = document.createElement('a')
        a.href = url
        a.download = frameworkName
          ? `${frameworkName} — Statement of Applicability.pdf`
          : `soa-${frameworkId.slice(0, 8)}.pdf`
        document.body.appendChild(a)
        a.click()
        a.remove()
        URL.revokeObjectURL(url)
      })
  }
}

export function useDownloadAuditPackage() {
  return (frameworkId: string, frameworkName?: string) => {
    void fetch(`/api/v1/vaktcomply/frameworks/${frameworkId}/audit-package.zip`, {
      credentials: 'include',
    })
      .then((r) => r.blob())
      .then((blob) => {
        const url = URL.createObjectURL(blob)
        const a = document.createElement('a')
        a.href = url
        a.download = frameworkName
          ? `audit-package-${frameworkName}.zip`
          : `audit-package-${frameworkId.slice(0, 8)}.zip`
        document.body.appendChild(a)
        a.click()
        a.remove()
        URL.revokeObjectURL(url)
      })
  }
}
