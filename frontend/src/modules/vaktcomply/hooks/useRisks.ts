import { useQuery, useMutation, useQueryClient, type QueryKey } from '@tanstack/react-query'
import { apiFetch } from '../../../api/client'
import type { Risk, CreateRiskInput, UpdateRiskInput, UpdateRiskTreatmentInput, Control } from '../types'
import type { PaginatedResponse } from '../../../shared/types/pagination'

export function useDeleteRisk() {
  const queryClient = useQueryClient()
  return useMutation<void, Error, string>({
    mutationFn: (id) => apiFetch<void>(`/vaktcomply/risks/${id}`, { method: 'DELETE' }),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: ['vaktcomply', 'risks'] })
    },
  })
}

export function useUpdateRiskStatus() {
  const queryClient = useQueryClient()
  return useMutation<Risk, Error, { risk: Risk; status: Risk['status'] }, { prevQueries: [QueryKey, PaginatedResponse<Risk> | undefined][] }>({
    mutationFn: ({ risk, status }) =>
      apiFetch<Risk>(`/vaktcomply/risks/${risk.id}`, {
        method: 'PATCH',
        body: JSON.stringify({
          title: risk.title,
          description: risk.description ?? '',
          category: risk.category ?? '',
          likelihood: risk.likelihood,
          impact: risk.impact,
          owner: risk.owner ?? '',
          status,
          treatment: risk.treatment,
          treatment_notes: risk.treatment_notes ?? '',
        }),
      }),
    onMutate: async ({ risk, status }) => {
      await queryClient.cancelQueries({ queryKey: ['vaktcomply', 'risks'] })
      const prevQueries = queryClient.getQueriesData<PaginatedResponse<Risk>>({ queryKey: ['vaktcomply', 'risks'] })
      queryClient.setQueriesData<PaginatedResponse<Risk>>(
        { queryKey: ['vaktcomply', 'risks'] },
        (old) => old ? { ...old, data: old.data.map((r) => r.id === risk.id ? { ...r, status } : r) } : old,
      )
      return { prevQueries }
    },
    onError: (_err, _vars, ctx) => {
      if (ctx?.prevQueries) {
        for (const [key, data] of ctx.prevQueries) queryClient.setQueryData(key, data)
      }
    },
    onSettled: () => {
      void queryClient.invalidateQueries({ queryKey: ['vaktcomply', 'risks'] })
    },
  })
}

export function useRisks(page = 1, limit = 25) {
  const query = useQuery<PaginatedResponse<Risk>>({
    queryKey: ['vaktcomply', 'risks', page, limit],
    queryFn: () => apiFetch<PaginatedResponse<Risk>>(`/vaktcomply/risks?page=${String(page)}&limit=${String(limit)}`),
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
    queryKey: ['vaktcomply', 'risks', id],
    queryFn: () => apiFetch<Risk>(`/vaktcomply/risks/${id}`),
    enabled: !!id,
    staleTime: 5 * 60 * 1000,
  })
}

export function useCreateRisk() {
  const queryClient = useQueryClient()
  return useMutation<Risk, Error, CreateRiskInput>({
    mutationFn: (input) =>
      apiFetch<Risk>('/vaktcomply/risks', { method: 'POST', body: JSON.stringify(input) }),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: ['vaktcomply', 'risks'] })
    },
  })
}

export function useUpdateRisk(id: string) {
  const queryClient = useQueryClient()
  return useMutation<Risk, Error, UpdateRiskInput>({
    mutationFn: (input) =>
      apiFetch<Risk>(`/vaktcomply/risks/${id}`, { method: 'PATCH', body: JSON.stringify(input) }),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: ['vaktcomply', 'risks'] })
      void queryClient.invalidateQueries({ queryKey: ['vaktcomply', 'risks', id] })
    },
  })
}

export function useUpdateRiskTreatment(id: string) {
  const queryClient = useQueryClient()
  return useMutation<Risk, Error, UpdateRiskTreatmentInput>({
    mutationFn: (input) =>
      apiFetch<Risk>(`/vaktcomply/risks/${id}/treatment`, { method: 'PATCH', body: JSON.stringify(input) }),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: ['vaktcomply', 'risks'] })
      void queryClient.invalidateQueries({ queryKey: ['vaktcomply', 'risks', id] })
    },
  })
}

export function useRiskControls(riskId: string) {
  return useQuery<Control[]>({
    queryKey: ['vaktcomply', 'risks', riskId, 'controls'],
    queryFn: () => apiFetch<Control[]>(`/vaktcomply/risks/${riskId}/controls`),
    enabled: !!riskId,
    staleTime: 5 * 60 * 1000,
  })
}

export function useLinkRiskControl(riskId: string) {
  const queryClient = useQueryClient()
  return useMutation<{ status: string }, Error, string>({
    mutationFn: (controlId) =>
      apiFetch<{ status: string }>(`/vaktcomply/risks/${riskId}/controls`, {
        method: 'POST',
        body: JSON.stringify({ control_id: controlId }),
      }),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: ['vaktcomply', 'risks', riskId, 'controls'] })
    },
  })
}

export function useUnlinkRiskControl(riskId: string) {
  const queryClient = useQueryClient()
  return useMutation<void, Error, string>({
    mutationFn: (controlId) =>
      apiFetch<void>(`/vaktcomply/risks/${riskId}/controls/${controlId}`, { method: 'DELETE' }),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: ['vaktcomply', 'risks', riskId, 'controls'] })
    },
  })
}
