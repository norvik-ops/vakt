import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { apiFetch } from '../../../api/client'
import type { Framework, ReadinessReport, GapAnalysis, Control } from '../types'
import type { PaginatedResponse } from '../../../shared/types/pagination'

export function useTISAXReport(frameworkId: string) {
  return useQuery<ReadinessReport>({
    queryKey: ['secvitals', 'frameworks', frameworkId, 'report', 'tisax'],
    queryFn: () => apiFetch<ReadinessReport>(`/secvitals/frameworks/${frameworkId}/report`),
    enabled: !!frameworkId,
    staleTime: 5 * 60 * 1000,
  })
}

export function useFrameworks() {
  return useQuery<Framework[]>({
    queryKey: ['secvitals', 'frameworks'],
    queryFn: () => apiFetch<Framework[]>('/secvitals/frameworks'),
    staleTime: 5 * 60 * 1000,
  })
}

export function useFramework(frameworkId: string) {
  return useQuery<Framework>({
    queryKey: ['secvitals', 'frameworks', frameworkId],
    queryFn: () => apiFetch<Framework>(`/secvitals/frameworks/${frameworkId}`),
    enabled: !!frameworkId,
    staleTime: 5 * 60 * 1000,
  })
}

export function useEnableFramework() {
  const queryClient = useQueryClient()
  return useMutation<Framework, Error, string>({
    mutationFn: (name: string) =>
      apiFetch<Framework>(`/secvitals/frameworks/${name}/enable`, { method: 'POST' }),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: ['secvitals', 'frameworks'] })
    },
  })
}

export function useDeleteFramework() {
  const queryClient = useQueryClient()
  return useMutation<void, Error, string>({
    mutationFn: (id: string) =>
      apiFetch<void>(`/secvitals/frameworks/${id}`, { method: 'DELETE' }),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: ['secvitals', 'frameworks'] })
    },
  })
}

export function useReadinessReport(frameworkId: string) {
  return useQuery<ReadinessReport>({
    queryKey: ['secvitals', 'frameworks', frameworkId, 'report'],
    queryFn: () => apiFetch<ReadinessReport>(`/secvitals/frameworks/${frameworkId}/report`),
    enabled: !!frameworkId,
    staleTime: 5 * 60 * 1000,
  })
}

export function useGapAnalysis(frameworkId: string) {
  return useQuery<GapAnalysis>({
    queryKey: ['secvitals', 'frameworks', frameworkId, 'gaps'],
    queryFn: () => apiFetch<GapAnalysis>(`/secvitals/frameworks/${frameworkId}/gaps`),
    enabled: !!frameworkId,
    staleTime: 5 * 60 * 1000,
  })
}

export function useFrameworkControls(frameworkId: string, page = 1, limit = 25) {
  const query = useQuery<PaginatedResponse<Control>>({
    queryKey: ['secvitals', 'frameworks', frameworkId, 'controls', page, limit],
    queryFn: () =>
      apiFetch<PaginatedResponse<Control>>(
        `/secvitals/frameworks/${frameworkId}/controls?page=${page}&limit=${limit}`,
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
    fetch(`/api/v1/secvitals/frameworks/${frameworkId}/export-pdf`, {
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
    fetch(`/api/v1/secvitals/frameworks/${frameworkId}/soa.pdf`, {
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
    fetch(`/api/v1/secvitals/frameworks/${frameworkId}/audit-package.zip`, {
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
