import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { apiFetch } from '../../../api/client'
import type { Incident, CreateIncidentInput, UpdateIncidentInput, MarkDeadlineReportedInput, AssessReportabilityInput, ReportabilityResult, IncidentReport, GenerateReportInput } from '../types'
import type { PaginatedResponse } from '../../../shared/types/pagination'

export function useIncidents(page = 1, limit = 25) {
  const query = useQuery<PaginatedResponse<Incident>>({
    queryKey: ['secvitals', 'incidents', page, limit],
    queryFn: () => apiFetch<PaginatedResponse<Incident>>(`/secvitals/incidents?page=${page}&limit=${limit}`),
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
    queryKey: ['secvitals', 'incidents', id],
    queryFn: () => apiFetch<Incident>(`/secvitals/incidents/${id}`),
    enabled: !!id,
    staleTime: 5 * 60 * 1000,
  })
}

export function useCreateIncident() {
  const queryClient = useQueryClient()
  return useMutation<Incident, Error, CreateIncidentInput>({
    mutationFn: (input) =>
      apiFetch<Incident>('/secvitals/incidents', { method: 'POST', body: JSON.stringify(input) }),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: ['secvitals', 'incidents'] })
    },
  })
}

export function useUpdateIncident(id: string) {
  const queryClient = useQueryClient()
  return useMutation<Incident, Error, UpdateIncidentInput>({
    mutationFn: (input) =>
      apiFetch<Incident>(`/secvitals/incidents/${id}`, { method: 'PATCH', body: JSON.stringify(input) }),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: ['secvitals', 'incidents'] })
      void queryClient.invalidateQueries({ queryKey: ['secvitals', 'incidents', id] })
    },
  })
}

export function useMarkDeadlineReported(id: string) {
  const queryClient = useQueryClient()
  return useMutation<Incident, Error, MarkDeadlineReportedInput>({
    mutationFn: (input) =>
      apiFetch<Incident>(`/secvitals/incidents/${id}/mark-reported`, { method: 'POST', body: JSON.stringify(input) }),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: ['secvitals', 'incidents'] })
      void queryClient.invalidateQueries({ queryKey: ['secvitals', 'incidents', id] })
    },
  })
}

export function useAssessReportability(id: string) {
  const queryClient = useQueryClient()
  return useMutation<ReportabilityResult, Error, AssessReportabilityInput>({
    mutationFn: (input) =>
      apiFetch<ReportabilityResult>(`/secvitals/incidents/${id}/assess-reportability`, { method: 'POST', body: JSON.stringify(input) }),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: ['secvitals', 'incidents', id] })
    },
  })
}

export function useIncidentReports(id: string) {
  return useQuery<IncidentReport[]>({
    queryKey: ['secvitals', 'incidents', id, 'reports'],
    queryFn: () => apiFetch<IncidentReport[]>(`/secvitals/incidents/${id}/reports`),
    enabled: !!id,
    staleTime: 5 * 60 * 1000,
  })
}

export function useGenerateIncidentReport(id: string) {
  const queryClient = useQueryClient()
  return useMutation<IncidentReport, Error, GenerateReportInput>({
    mutationFn: (input) =>
      apiFetch<IncidentReport>(`/secvitals/incidents/${id}/reports`, { method: 'POST', body: JSON.stringify(input) }),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: ['secvitals', 'incidents', id, 'reports'] })
    },
  })
}
