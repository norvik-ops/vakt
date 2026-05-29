import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { apiFetch } from '../../../api/client'
import type { Incident, CreateIncidentInput, UpdateIncidentInput, MarkDeadlineReportedInput, AssessReportabilityInput, ReportabilityResult, IncidentReport, GenerateReportInput, ClassifyReportingInput, ClassificationResult } from '../types'
import type { PaginatedResponse } from '../../../shared/types/pagination'

export function useIncidents(page = 1, limit = 25) {
  const query = useQuery<PaginatedResponse<Incident>>({
    queryKey: ['vaktcomply', 'incidents', page, limit],
    queryFn: () => apiFetch<PaginatedResponse<Incident>>(`/vaktcomply/incidents?page=${String(page)}&limit=${String(limit)}`),
    staleTime: 5 * 60 * 1000,
  })
  return {
    ...query,
    data: query.data?.data,
    pagination: query.data?.pagination,
  }
}

export function useIncident(id: string) {
  return useQuery<Incident>({
    queryKey: ['vaktcomply', 'incidents', id],
    queryFn: () => apiFetch<Incident>(`/vaktcomply/incidents/${id}`),
    enabled: !!id,
    staleTime: 5 * 60 * 1000,
  })
}

export function useCreateIncident() {
  const queryClient = useQueryClient()
  return useMutation<Incident, Error, CreateIncidentInput>({
    mutationFn: (input) =>
      apiFetch<Incident>('/vaktcomply/incidents', { method: 'POST', body: JSON.stringify(input) }),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: ['vaktcomply', 'incidents'] })
    },
  })
}

export function useUpdateIncident(id: string) {
  const queryClient = useQueryClient()
  return useMutation<Incident, Error, UpdateIncidentInput>({
    mutationFn: (input) =>
      apiFetch<Incident>(`/vaktcomply/incidents/${id}`, { method: 'PATCH', body: JSON.stringify(input) }),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: ['vaktcomply', 'incidents'] })
      void queryClient.invalidateQueries({ queryKey: ['vaktcomply', 'incidents', id] })
    },
  })
}

export function useMarkDeadlineReported(id: string) {
  const queryClient = useQueryClient()
  return useMutation<Incident, Error, MarkDeadlineReportedInput>({
    mutationFn: (input) =>
      apiFetch<Incident>(`/vaktcomply/incidents/${id}/mark-reported`, { method: 'POST', body: JSON.stringify(input) }),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: ['vaktcomply', 'incidents'] })
      void queryClient.invalidateQueries({ queryKey: ['vaktcomply', 'incidents', id] })
    },
  })
}

export function useAssessReportability(id: string) {
  const queryClient = useQueryClient()
  return useMutation<ReportabilityResult, Error, AssessReportabilityInput>({
    mutationFn: (input) =>
      apiFetch<ReportabilityResult>(`/vaktcomply/incidents/${id}/assess-reportability`, { method: 'POST', body: JSON.stringify(input) }),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: ['vaktcomply', 'incidents', id] })
    },
  })
}

export function useIncidentReports(id: string) {
  return useQuery<IncidentReport[]>({
    queryKey: ['vaktcomply', 'incidents', id, 'reports'],
    queryFn: () => apiFetch<IncidentReport[]>(`/vaktcomply/incidents/${id}/reports`),
    enabled: !!id,
    staleTime: 5 * 60 * 1000,
  })
}

export function useGenerateIncidentReport(id: string) {
  const queryClient = useQueryClient()
  return useMutation<IncidentReport, Error, GenerateReportInput>({
    mutationFn: (input) =>
      apiFetch<IncidentReport>(`/vaktcomply/incidents/${id}/reports`, { method: 'POST', body: JSON.stringify(input) }),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: ['vaktcomply', 'incidents', id, 'reports'] })
    },
  })
}

// S39-1: BSI-Meldepflicht-Klassifizierung
export function useClassifyReportingObligation(id: string) {
  const queryClient = useQueryClient()
  return useMutation<ClassificationResult, Error, ClassifyReportingInput>({
    mutationFn: (input) =>
      apiFetch<ClassificationResult>(`/vaktcomply/incidents/${id}/classify-reporting`, { method: 'POST', body: JSON.stringify(input) }),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: ['vaktcomply', 'incidents', id] })
    },
  })
}
