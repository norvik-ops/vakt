import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { apiFetch } from '../../../api/client'
import type { GitScan, ScanResult } from '../types'

const BASE = '/secvault'

export function useGitScans() {
  return useQuery<GitScan[]>({
    queryKey: ['secvault', 'git-scans'],
    queryFn: async () => {
      const resp = await apiFetch<{ data: GitScan[] } | GitScan[]>(`${BASE}/git-scans`)
      return Array.isArray(resp) ? resp : (resp.data ?? [])
    },
    staleTime: 15_000,
  })
}

export function useGitScan(id: string) {
  return useQuery<GitScan>({
    queryKey: ['secvault', 'git-scans', id],
    queryFn: () => apiFetch<GitScan>(`${BASE}/git-scans/${id}`),
    staleTime: 10_000,
    enabled: Boolean(id),
  })
}

export function useGitScanResults(scanId: string, enabled: boolean) {
  return useQuery<ScanResult[]>({
    queryKey: ['secvault', 'git-scans', scanId, 'results'],
    queryFn: () => apiFetch<ScanResult[]>(`${BASE}/git-scans/${scanId}/results`),
    staleTime: 30_000,
    enabled: enabled && Boolean(scanId),
  })
}

export function useTriggerGitScan() {
  const queryClient = useQueryClient()
  return useMutation<GitScan, Error, { repo_url: string }>({
    mutationFn: (data) =>
      apiFetch<GitScan>(`${BASE}/git-scans`, {
        method: 'POST',
        body: JSON.stringify(data),
      }),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: ['secvault', 'git-scans'] })
    },
  })
}

export function useDismissScanResult() {
  const queryClient = useQueryClient()
  return useMutation<undefined, Error, { resultId: string; reason: string }>({
    mutationFn: ({ resultId, reason }) =>
      apiFetch<undefined>(`${BASE}/git-scans/results/${resultId}/dismiss`, {
        method: 'POST',
        body: JSON.stringify({ reason }),
      }),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: ['secvault', 'git-scans'] })
    },
  })
}
