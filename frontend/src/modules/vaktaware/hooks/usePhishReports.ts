import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { apiFetch } from '../../../api/client'
import type { PhishReport, PhishReportStats } from '../types'

const BASE = '/vaktaware'

export function usePhishReports() {
  return useQuery<PhishReport[]>({
    queryKey: ['vaktaware', 'phish-reports'],
    queryFn: () => apiFetch<PhishReport[]>(`${BASE}/phish-reports`),
    staleTime: 30_000,
  })
}

export function usePhishReportStats() {
  return useQuery<PhishReportStats>({
    queryKey: ['vaktaware', 'phish-reports', 'stats'],
    queryFn: () => apiFetch<PhishReportStats>(`${BASE}/phish-reports/stats`),
    staleTime: 30_000,
  })
}

export function useRegeneratePhishToken() {
  const queryClient = useQueryClient()
  return useMutation<{ token: string }>({
    mutationFn: () =>
      apiFetch<{ token: string }>(`${BASE}/phish-report-token/regenerate`, {
        method: 'POST',
      }),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: ['vaktaware', 'phish-reports'] })
    },
  })
}
