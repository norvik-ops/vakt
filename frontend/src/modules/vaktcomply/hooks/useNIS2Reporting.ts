import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { apiFetch } from '../../../api/client'
import type { NIS2ReportStatus, NIS2ReportabilityCheck, NIS2ReportInput, NIS2StageReport, AuthorityContact } from '../types'

export function useNIS2Status(incidentId: string) {
  return useQuery<NIS2ReportStatus>({
    queryKey: ['vaktcomply', 'incidents', incidentId, 'nis2-status'],
    queryFn: () => apiFetch<NIS2ReportStatus>(`/vaktcomply/incidents/${incidentId}/nis2-status`),
    enabled: !!incidentId,
    staleTime: 2 * 60 * 1000,
  })
}

interface NIS2AssessPayload {
  detected_at: string
  check: NIS2ReportabilityCheck
}

export function useNIS2AssessReportability(incidentId: string) {
  const queryClient = useQueryClient()
  return useMutation<undefined, Error, NIS2AssessPayload>({
    mutationFn: (payload) =>
      apiFetch<undefined>(`/vaktcomply/incidents/${incidentId}/nis2/assess`, {
        method: 'POST',
        body: JSON.stringify(payload),
      }),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: ['vaktcomply', 'incidents', incidentId, 'nis2-status'] })
      void queryClient.invalidateQueries({ queryKey: ['vaktcomply', 'incidents', incidentId] })
    },
  })
}

interface NIS2SubmitStagePayload {
  stage: 'early_warning' | 'full_report' | 'final_report'
  input: NIS2ReportInput
}

export function useNIS2SubmitStage(incidentId: string) {
  const queryClient = useQueryClient()
  return useMutation<NIS2StageReport, Error, NIS2SubmitStagePayload>({
    mutationFn: ({ stage, input }) =>
      apiFetch<NIS2StageReport>(`/vaktcomply/incidents/${incidentId}/nis2/submit/${stage}`, {
        method: 'POST',
        body: JSON.stringify(input),
      }),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: ['vaktcomply', 'incidents', incidentId, 'nis2-status'] })
    },
  })
}

export function useAuthorityContacts() {
  return useQuery<AuthorityContact[]>({
    queryKey: ['vaktcomply', 'authority-contacts'],
    queryFn: () => apiFetch<AuthorityContact[]>('/vaktcomply/authority-contacts'),
    staleTime: 10 * 60 * 1000,
  })
}

export function useCreateAuthorityContact() {
  const queryClient = useQueryClient()
  return useMutation<AuthorityContact, Error, Partial<AuthorityContact>>({
    mutationFn: (payload) =>
      apiFetch<AuthorityContact>('/vaktcomply/authority-contacts', {
        method: 'POST',
        body: JSON.stringify(payload),
      }),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: ['vaktcomply', 'authority-contacts'] })
    },
  })
}
