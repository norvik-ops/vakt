import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { apiFetch } from '../../../api/client'
import type { Asset, SLAEntry, ClassificationSummary, SLAPolicy, SLASummaryFE } from '../types'
import type { PaginatedResponse } from '../../../shared/types/pagination'

export interface CreateAssetInput {
  name: string
  type: Asset['type']
  target: string
  criticality: Asset['criticality']
  tags: string[]
  classification?: Asset['classification']
}

export function useAssets(page = 1, limit = 25, tag?: string) {
  const params = new URLSearchParams({ page: String(page), limit: String(limit) })
  if (tag) params.set('tag', tag)
  const query = useQuery<PaginatedResponse<Asset>>({
    queryKey: ['vaktscan', 'assets', page, limit, tag],
    queryFn: () => apiFetch<PaginatedResponse<Asset>>(`/vaktscan/assets?${params.toString()}`),
    staleTime: 30_000,
  })
  return {
    ...query,
    data: query.data?.data,
    pagination: query.data?.pagination,
  }
}

export function useAsset(id: string) {
  return useQuery<Asset>({
    queryKey: ['vaktscan', 'assets', id],
    queryFn: () => apiFetch<Asset>(`/vaktscan/assets/${id}`),
    staleTime: 30_000,
    enabled: Boolean(id),
  })
}

export function useCreateAsset() {
  const queryClient = useQueryClient()
  return useMutation<Asset, Error, CreateAssetInput>({
    mutationFn: (data) =>
      apiFetch<Asset>('/vaktscan/assets', {
        method: 'POST',
        body: JSON.stringify(data),
      }),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: ['vaktscan', 'assets'] })
    },
  })
}

export interface ImportAssetsResult {
  inserted: number
  errored: number
  errors: string[]
}

export function useImportAssets() {
  const queryClient = useQueryClient()
  return useMutation<ImportAssetsResult, Error, FormData>({
    mutationFn: (formData) => {
      return fetch('/api/v1/vaktscan/assets/import', {
        method: 'POST',
        credentials: 'include',
        body: formData,
      }).then(async (res) => {
        if (!res.ok) {
          const err = await res.json().catch(() => ({ error: res.statusText })) as { error?: string }
          throw new Error(err.error ?? res.statusText)
        }
        return res.json() as Promise<ImportAssetsResult>
      })
    },
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: ['vaktscan', 'assets'] })
    },
  })
}

export function useSLADashboard() {
  return useQuery<SLAEntry[]>({
    queryKey: ['vaktscan', 'sla-dashboard'],
    queryFn: () => apiFetch<SLAEntry[]>('/vaktscan/sla-dashboard'),
    staleTime: 60_000,
  })
}

export function useClassificationSummary() {
  return useQuery<ClassificationSummary>({
    queryKey: ['vaktscan', 'assets', 'classification-summary'],
    queryFn: () => apiFetch<ClassificationSummary>('/vaktscan/assets/classification-summary'),
    staleTime: 5 * 60 * 1000,
  })
}

export function useDeleteAsset() {
  const queryClient = useQueryClient()
  return useMutation<undefined, Error, string>({
    mutationFn: (id) =>
      apiFetch<undefined>(`/vaktscan/assets/${id}`, { method: 'DELETE' }),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: ['vaktscan', 'assets'] })
    },
  })
}

export function useTriggerScan(assetId: string) {
  const queryClient = useQueryClient()
  return useMutation<undefined>({
    mutationFn: () =>
      apiFetch<undefined>(`/vaktscan/assets/${assetId}/scans`, { method: 'POST' }),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: ['vaktscan', 'assets', assetId] })
      void queryClient.invalidateQueries({ queryKey: ['vaktscan', 'findings'] })
    },
  })
}

// ── S69-3: SLA Policies ───────────────────────────────────────────────────────

export function useSLAPolicies() {
  return useQuery<SLAPolicy[]>({
    queryKey: ['vaktscan', 'sla-policies'],
    queryFn: () => apiFetch<SLAPolicy[]>('/vaktscan/sla-policies'),
    staleTime: 5 * 60 * 1000,
  })
}

export function useSLASummaryNew() {
  return useQuery<SLASummaryFE>({
    queryKey: ['vaktscan', 'sla-summary'],
    queryFn: () => apiFetch<SLASummaryFE>('/vaktscan/sla/summary'),
    staleTime: 60_000,
  })
}

export function useUpsertSLAPolicy() {
  const queryClient = useQueryClient()
  return useMutation<SLAPolicy, Error, { severity: string; remediation_days: number; notification_advance_days: number }>({
    mutationFn: ({ severity, ...body }) =>
      apiFetch<SLAPolicy>(`/vaktscan/sla-policies/${severity}`, { method: 'PUT', body: JSON.stringify(body) }),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: ['vaktscan', 'sla-policies'] })
      void queryClient.invalidateQueries({ queryKey: ['vaktscan', 'sla-summary'] })
    },
  })
}

export function useResetSLAPolicies() {
  const queryClient = useQueryClient()
  return useMutation<void>({
    mutationFn: () => apiFetch('/vaktscan/sla-policies/reset', { method: 'POST' }),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: ['vaktscan', 'sla-policies'] })
    },
  })
}
