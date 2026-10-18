import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { apiFetch } from '../../../api/client'
import type { SLAEntry, SLAPolicy, SLASummaryFE } from '../types'

export function useSLADashboard() {
  return useQuery<SLAEntry[]>({
    queryKey: ['vaktscan', 'sla-dashboard'],
    queryFn: () => apiFetch<SLAEntry[]>('/vaktscan/sla-dashboard'),
    staleTime: 60_000,
  })
}

/** The scanners CreateScanInput accepts — `oneof=trivy nuclei openvas`. */
export type ScannerName = 'trivy' | 'nuclei' | 'openvas'

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
