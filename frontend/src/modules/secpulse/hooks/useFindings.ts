import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { apiFetch } from '../../../api/client'
import type { Finding, FindingsListResponse } from '../types'

export interface FindingsFilter {
  severity?: string
  status?: string
  asset_id?: string
  search?: string
}

export interface PatchFindingInput {
  status?: Finding['status']
  notes?: string
  assigned_to?: string
}

export interface BulkUpdateInput {
  ids: string[]
  status: Finding['status']
}

export function useFindings(filter: FindingsFilter = {}, page = 1, limit = 25) {
  const params = new URLSearchParams()
  if (filter.severity) params.set('severity', filter.severity)
  if (filter.status) params.set('status', filter.status)
  if (filter.asset_id) params.set('asset_id', filter.asset_id)
  if (filter.search) params.set('search', filter.search)
  params.set('page', String(page))
  params.set('limit', String(limit))

  const path = `/secpulse/findings?${params.toString()}`

  return useQuery<FindingsListResponse>({
    queryKey: ['secpulse', 'findings', filter, page, limit],
    queryFn: () => apiFetch<FindingsListResponse>(path),
    staleTime: 30_000,
  })
}

export function useFinding(id: string) {
  return useQuery<Finding>({
    queryKey: ['secpulse', 'findings', id],
    queryFn: () => apiFetch<Finding>(`/secpulse/findings/${id}`),
    staleTime: 30_000,
    enabled: Boolean(id),
  })
}

export function usePatchFinding(id: string) {
  const queryClient = useQueryClient()
  return useMutation<Finding, Error, PatchFindingInput>({
    mutationFn: (data) =>
      apiFetch<Finding>(`/secpulse/findings/${id}`, {
        method: 'PATCH',
        body: JSON.stringify(data),
      }),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: ['secpulse', 'findings', id] })
      void queryClient.invalidateQueries({ queryKey: ['secpulse', 'findings'] })
    },
  })
}

export function useBulkUpdateFindings() {
  const queryClient = useQueryClient()
  return useMutation<void, Error, BulkUpdateInput>({
    mutationFn: (data) =>
      apiFetch<void>('/secpulse/findings/bulk', {
        method: 'POST',
        body: JSON.stringify(data),
      }),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: ['secpulse', 'findings'] })
    },
  })
}

export async function exportFindingsCsv() {
  // Use fetch with Authorization header + blob download to avoid 401 on <a href> navigation
  const blob = await apiFetch<Blob>('/secpulse/findings/export.csv', {
    headers: { Accept: 'text/csv' },
  })
  const url = URL.createObjectURL(blob)
  const a = document.createElement('a')
  a.href = url
  a.download = 'findings.csv'
  document.body.appendChild(a)
  a.click()
  document.body.removeChild(a)
  URL.revokeObjectURL(url)
}
