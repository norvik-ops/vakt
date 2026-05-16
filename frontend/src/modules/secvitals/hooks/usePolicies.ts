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
    queryKey: ['secvitals', 'policies', page, limit],
    queryFn: () => apiFetch<PaginatedResponse<Policy>>(`/secvitals/policies?page=${page}&limit=${limit}`),
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
    queryKey: ['secvitals', 'policies', id],
    queryFn: () => apiFetch<Policy>(`/secvitals/policies/${id}`),
    enabled: !!id,
    staleTime: 5 * 60 * 1000,
  })
}

export function useCreatePolicy() {
  const queryClient = useQueryClient()
  return useMutation<Policy, Error, CreatePolicyInput>({
    mutationFn: (input) =>
      apiFetch<Policy>('/secvitals/policies', { method: 'POST', body: JSON.stringify(input) }),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: ['secvitals', 'policies'] })
    },
  })
}

export function useUpdatePolicy(id: string) {
  const queryClient = useQueryClient()
  return useMutation<Policy, Error, UpdatePolicyInput>({
    mutationFn: (input) =>
      apiFetch<Policy>(`/secvitals/policies/${id}`, { method: 'PATCH', body: JSON.stringify(input) }),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: ['secvitals', 'policies'] })
      void queryClient.invalidateQueries({ queryKey: ['secvitals', 'policies', id] })
    },
  })
}

export function useGeneratePolicyDraft() {
  return useMutation<{ draft: string }, Error, GeneratePolicyDraftInput>({
    mutationFn: (input) =>
      apiFetch<{ draft: string }>('/secvitals/policies/generate-draft', {
        method: 'POST',
        body: JSON.stringify(input),
      }),
  })
}
