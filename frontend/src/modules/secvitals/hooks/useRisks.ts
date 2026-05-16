import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { apiFetch } from '../../../api/client'
import type { Risk, CreateRiskInput, UpdateRiskInput, UpdateRiskTreatmentInput, Control } from '../types'
import type { PaginatedResponse } from '../../../shared/types/pagination'

export function useRisks(page = 1, limit = 25) {
  const query = useQuery<PaginatedResponse<Risk>>({
    queryKey: ['secvitals', 'risks', page, limit],
    queryFn: () => apiFetch<PaginatedResponse<Risk>>(`/secvitals/risks?page=${page}&limit=${limit}`),
    staleTime: 5 * 60 * 1000,
  })
  return {
    ...query,
    data: query.data?.data,
    pagination: query.data?.pagination,
  }
}

export function useRisk(id: string) {
  return useQuery<Risk>({
    queryKey: ['secvitals', 'risks', id],
    queryFn: () => apiFetch<Risk>(`/secvitals/risks/${id}`),
    enabled: !!id,
    staleTime: 5 * 60 * 1000,
  })
}

export function useCreateRisk() {
  const queryClient = useQueryClient()
  return useMutation<Risk, Error, CreateRiskInput>({
    mutationFn: (input) =>
      apiFetch<Risk>('/secvitals/risks', { method: 'POST', body: JSON.stringify(input) }),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: ['secvitals', 'risks'] })
    },
  })
}

export function useUpdateRisk(id: string) {
  const queryClient = useQueryClient()
  return useMutation<Risk, Error, UpdateRiskInput>({
    mutationFn: (input) =>
      apiFetch<Risk>(`/secvitals/risks/${id}`, { method: 'PATCH', body: JSON.stringify(input) }),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: ['secvitals', 'risks'] })
      void queryClient.invalidateQueries({ queryKey: ['secvitals', 'risks', id] })
    },
  })
}

export function useUpdateRiskTreatment(id: string) {
  const queryClient = useQueryClient()
  return useMutation<Risk, Error, UpdateRiskTreatmentInput>({
    mutationFn: (input) =>
      apiFetch<Risk>(`/secvitals/risks/${id}/treatment`, { method: 'PATCH', body: JSON.stringify(input) }),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: ['secvitals', 'risks'] })
      void queryClient.invalidateQueries({ queryKey: ['secvitals', 'risks', id] })
    },
  })
}

export function useRiskControls(riskId: string) {
  return useQuery<Control[]>({
    queryKey: ['secvitals', 'risks', riskId, 'controls'],
    queryFn: () => apiFetch<Control[]>(`/secvitals/risks/${riskId}/controls`),
    enabled: !!riskId,
    staleTime: 5 * 60 * 1000,
  })
}

export function useLinkRiskControl(riskId: string) {
  const queryClient = useQueryClient()
  return useMutation<{ status: string }, Error, string>({
    mutationFn: (controlId) =>
      apiFetch<{ status: string }>(`/secvitals/risks/${riskId}/controls`, {
        method: 'POST',
        body: JSON.stringify({ control_id: controlId }),
      }),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: ['secvitals', 'risks', riskId, 'controls'] })
    },
  })
}

export function useUnlinkRiskControl(riskId: string) {
  const queryClient = useQueryClient()
  return useMutation<void, Error, string>({
    mutationFn: (controlId) =>
      apiFetch<void>(`/secvitals/risks/${riskId}/controls/${controlId}`, { method: 'DELETE' }),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: ['secvitals', 'risks', riskId, 'controls'] })
    },
  })
}
