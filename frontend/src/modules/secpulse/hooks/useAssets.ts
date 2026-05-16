import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { apiFetch, getAuthToken } from '../../../api/client'
import type { Asset, SLAEntry } from '../types'
import type { PaginatedResponse } from '../../../shared/types/pagination'

export interface CreateAssetInput {
  name: string
  type: Asset['type']
  target: string
  criticality: Asset['criticality']
  tags: string[]
}

export function useAssets(page = 1, limit = 25, tag?: string) {
  const params = new URLSearchParams({ page: String(page), limit: String(limit) })
  if (tag) params.set('tag', tag)
  const query = useQuery<PaginatedResponse<Asset>>({
    queryKey: ['secpulse', 'assets', page, limit, tag],
    queryFn: () => apiFetch<PaginatedResponse<Asset>>(`/secpulse/assets?${params.toString()}`),
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
    queryKey: ['secpulse', 'assets', id],
    queryFn: () => apiFetch<Asset>(`/secpulse/assets/${id}`),
    staleTime: 30_000,
    enabled: Boolean(id),
  })
}

export function useCreateAsset() {
  const queryClient = useQueryClient()
  return useMutation<Asset, Error, CreateAssetInput>({
    mutationFn: (data) =>
      apiFetch<Asset>('/secpulse/assets', {
        method: 'POST',
        body: JSON.stringify(data),
      }),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: ['secpulse', 'assets'] })
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
      const token = getAuthToken()
      return fetch('/api/v1/secpulse/assets/import', {
        method: 'POST',
        headers: token ? { Authorization: `Bearer ${token}` } : {},
        body: formData,
      }).then(async (res) => {
        if (!res.ok) {
          const err = await res.json().catch(() => ({ error: res.statusText }))
          throw new Error((err as { error?: string }).error ?? res.statusText)
        }
        return res.json() as Promise<ImportAssetsResult>
      })
    },
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: ['secpulse', 'assets'] })
    },
  })
}

export function useSLADashboard() {
  return useQuery<SLAEntry[]>({
    queryKey: ['secpulse', 'sla-dashboard'],
    queryFn: () => apiFetch<SLAEntry[]>('/secpulse/sla-dashboard'),
    staleTime: 60_000,
  })
}

export function useTriggerScan(assetId: string) {
  const queryClient = useQueryClient()
  return useMutation<void, Error, void>({
    mutationFn: () =>
      apiFetch<void>(`/secpulse/assets/${assetId}/scans`, { method: 'POST' }),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: ['secpulse', 'assets', assetId] })
      void queryClient.invalidateQueries({ queryKey: ['secpulse', 'findings'] })
    },
  })
}
