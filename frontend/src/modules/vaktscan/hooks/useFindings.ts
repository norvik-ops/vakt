import { useQuery, useMutation, useQueryClient, type QueryKey } from '@tanstack/react-query'
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

  const path = `/vaktscan/findings?${params.toString()}`

  return useQuery<FindingsListResponse>({
    queryKey: ['vaktscan', 'findings', filter, page, limit],
    queryFn: () => apiFetch<FindingsListResponse>(path),
    staleTime: 30_000,
  })
}

export function useFinding(id: string) {
  return useQuery<Finding>({
    queryKey: ['vaktscan', 'findings', id],
    queryFn: () => apiFetch<Finding>(`/vaktscan/findings/${id}`),
    staleTime: 30_000,
    enabled: Boolean(id),
  })
}

export function usePatchFinding(id: string) {
  const queryClient = useQueryClient()
  return useMutation<Finding, Error, PatchFindingInput, { prevQueries: [QueryKey, FindingsListResponse | undefined][] }>({
    mutationFn: (data) =>
      apiFetch<Finding>(`/vaktscan/findings/${id}`, {
        method: 'PATCH',
        body: JSON.stringify(data),
      }),
    onMutate: async (data) => {
      await queryClient.cancelQueries({ queryKey: ['vaktscan', 'findings'] })
      const prevQueries = queryClient.getQueriesData<FindingsListResponse>({ queryKey: ['vaktscan', 'findings'] })
      queryClient.setQueriesData<FindingsListResponse>(
        { queryKey: ['vaktscan', 'findings'] },
        (old) => old ? { ...old, data: old.data.map((f) => f.id === id ? { ...f, ...data } : f) } : old,
      )
      return { prevQueries }
    },
    onError: (_err, _data, ctx) => {
      if (ctx?.prevQueries) {
        for (const [key, data] of ctx.prevQueries) queryClient.setQueryData(key, data)
      }
    },
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: ['vaktscan', 'findings', id] })
      void queryClient.invalidateQueries({ queryKey: ['vaktscan', 'findings'] })
    },
  })
}

export function useBulkUpdateFindings() {
  const queryClient = useQueryClient()
  return useMutation<undefined, Error, BulkUpdateInput>({
    mutationFn: (data) =>
      apiFetch<undefined>('/vaktscan/findings/bulk', {
        method: 'POST',
        body: JSON.stringify(data),
      }),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: ['vaktscan', 'findings'] })
    },
  })
}

export function useDeleteFinding() {
  const queryClient = useQueryClient()
  return useMutation<undefined, Error, string>({
    mutationFn: (id) =>
      apiFetch<undefined>(`/vaktscan/findings/${id}`, { method: 'DELETE' }),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: ['vaktscan', 'findings'] })
    },
  })
}

export async function exportFindingsCsv() {
  // Use fetch with Authorization header + blob download to avoid 401 on <a href> navigation
  const blob = await apiFetch<Blob>('/vaktscan/findings/export.csv', {
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
