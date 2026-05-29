import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { apiFetch } from '../../../api/client'
import type { Framework, ReadinessReport, GapAnalysis, Control } from '../types'
import type { PaginatedResponse } from '../../../shared/types/pagination'

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
  return useMutation<Framework, Error, string>({
    mutationFn: (name: string) =>
      apiFetch<Framework>(`/vaktcomply/frameworks/${name}/enable`, { method: 'POST' }),
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

export function useFrameworkControls(frameworkId: string, page = 1, limit = 25) {
  const query = useQuery<PaginatedResponse<Control>>({
    queryKey: ['vaktcomply', 'frameworks', frameworkId, 'controls', page, limit],
    queryFn: () =>
      apiFetch<PaginatedResponse<Control>>(
        `/vaktcomply/frameworks/${frameworkId}/controls?page=${page}&limit=${limit}`,
      ),
    enabled: !!frameworkId,
    staleTime: 5 * 60 * 1000,
  })
  return {
    ...query,
    // Expose items directly for backward-compat consumers that spread into data: Control[]
    data: query.data?.data,
    pagination: query.data?.pagination,
  }
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
