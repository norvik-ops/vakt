import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { apiFetch } from '../../../api/client'
import type { Policy, CreatePolicyInput, UpdatePolicyInput } from '../types'
import type { PaginatedResponse } from '../../../shared/types/pagination'

export interface GeneratePolicyDraftInput {
  policy_type: string
  framework_id?: string
  custom_context?: string
}

export function usePolicies(page = 1, limit = 25) {
  const query = useQuery<PaginatedResponse<Policy>>({
    queryKey: ['vaktcomply', 'policies', page, limit],
    queryFn: () => apiFetch<PaginatedResponse<Policy>>(`/vaktcomply/policies?page=${String(page)}&limit=${String(limit)}`),
    staleTime: 5 * 60 * 1000,
  })
  return {
    ...query,
    data: query.data?.data,
    pagination: query.data?.pagination,
  }
}

export function usePolicy(id: string) {
  return useQuery<Policy>({
    queryKey: ['vaktcomply', 'policies', id],
    queryFn: () => apiFetch<Policy>(`/vaktcomply/policies/${id}`),
    enabled: !!id,
    staleTime: 5 * 60 * 1000,
  })
}

export function useCreatePolicy() {
  const queryClient = useQueryClient()
  return useMutation<Policy, Error, CreatePolicyInput>({
    mutationFn: (input) =>
      apiFetch<Policy>('/vaktcomply/policies', { method: 'POST', body: JSON.stringify(input) }),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: ['vaktcomply', 'policies'] })
    },
  })
}

export function useUpdatePolicy(id: string) {
  const queryClient = useQueryClient()
  return useMutation<Policy, Error, UpdatePolicyInput>({
    mutationFn: (input) =>
      apiFetch<Policy>(`/vaktcomply/policies/${id}`, { method: 'PATCH', body: JSON.stringify(input) }),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: ['vaktcomply', 'policies'] })
      void queryClient.invalidateQueries({ queryKey: ['vaktcomply', 'policies', id] })
    },
  })
}

export function useGeneratePolicyDraft() {
  return useMutation<{ draft: string }, Error, GeneratePolicyDraftInput>({
    mutationFn: (input) =>
      apiFetch<{ draft: string }>('/vaktcomply/policies/generate-draft', {
        method: 'POST',
        body: JSON.stringify(input),
      }),
  })
}
