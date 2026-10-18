import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { apiFetch } from '../../../api/client'
import type { Asset, ClassificationSummary } from './types'
import type { PaginatedResponse } from '../../../shared/types/pagination'

// Mirrors assets.CreateAssetInput. `external_url`, not `target` (K5-16).
export interface CreateAssetInput {
  name: string
  type: Asset['type']
  external_url: string
  criticality: Asset['criticality']
  tags: string[]
  classification?: Asset['classification']
}

export function useAssets(page = 1, limit = 25, tag?: string) {
  const params = new URLSearchParams({ page: String(page), limit: String(limit) })
  if (tag) params.set('tag', tag)
  const query = useQuery<PaginatedResponse<Asset>>({
    queryKey: ['vaktcomply', 'assets', page, limit, tag],
    queryFn: () => apiFetch<PaginatedResponse<Asset>>(`/vaktcomply/assets?${params.toString()}`),
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
    queryKey: ['vaktcomply', 'assets', id],
    queryFn: () => apiFetch<Asset>(`/vaktcomply/assets/${id}`),
    staleTime: 30_000,
    enabled: Boolean(id),
  })
}

export function useCreateAsset() {
  const queryClient = useQueryClient()
  return useMutation<Asset, Error, CreateAssetInput>({
    mutationFn: (data) =>
      apiFetch<Asset>('/vaktcomply/assets', {
        method: 'POST',
        body: JSON.stringify(data),
      }),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: ['vaktcomply', 'assets'] })
    },
  })
}

export interface ImportAssetsResult {
  inserted: number
  errored: number
  errors: string[]
}

/**
 * R23-02: AssetsPage no longer calls this — the page's only import path is
 * CSVImportDialog, which posts through apiFetch itself and picks the Community
 * or Pro endpoint by licence. The hook stays because it is the subject of the
 * CSRF regression test for POST /vaktcomply/assets/import
 * (api/csrf-write-paths.test.tsx case 6, R1-18-D4); removing it would quietly
 * remove that endpoint's coverage along with it.
 */
export function useImportAssets() {
  const queryClient = useQueryClient()
  return useMutation<ImportAssetsResult, Error, FormData>({
    mutationFn: (formData) =>
      apiFetch<ImportAssetsResult>('/vaktcomply/assets/import', {
        method: 'POST',
        body: formData,
      }),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: ['vaktcomply', 'assets'] })
    },
  })
}

export function useClassificationSummary() {
  return useQuery<ClassificationSummary>({
    queryKey: ['vaktcomply', 'assets', 'classification-summary'],
    queryFn: () => apiFetch<ClassificationSummary>('/vaktcomply/assets/classification-summary'),
    staleTime: 5 * 60 * 1000,
  })
}

export function useDeleteAsset() {
  const queryClient = useQueryClient()
  return useMutation<undefined, Error, string>({
    mutationFn: (id) =>
      apiFetch<undefined>(`/vaktcomply/assets/${id}`, { method: 'DELETE' }),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: ['vaktcomply', 'assets'] })
    },
  })
}
