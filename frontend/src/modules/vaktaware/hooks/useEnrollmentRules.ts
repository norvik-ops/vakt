import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { apiFetch } from '../../../api/client'
import type { EnrollmentRule, CreateEnrollmentRuleInput } from '../types'

const BASE = '/vaktaware'

export function useEnrollmentRules() {
  return useQuery<EnrollmentRule[]>({
    queryKey: ['vaktaware', 'enrollment-rules'],
    queryFn: () => apiFetch<EnrollmentRule[]>(`${BASE}/enrollment-rules`),
    staleTime: 60_000,
  })
}

export function useCreateEnrollmentRule() {
  const queryClient = useQueryClient()
  return useMutation<EnrollmentRule, Error, CreateEnrollmentRuleInput>({
    mutationFn: (data) =>
      apiFetch<EnrollmentRule>(`${BASE}/enrollment-rules`, {
        method: 'POST',
        body: JSON.stringify(data),
      }),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: ['vaktaware', 'enrollment-rules'] })
    },
  })
}

export function useUpdateEnrollmentRule() {
  const queryClient = useQueryClient()
  return useMutation<void, Error, { id: string; isActive: boolean }>({
    mutationFn: ({ id, isActive }) =>
      apiFetch<void>(`${BASE}/enrollment-rules/${id}`, {
        method: 'PUT',
        body: JSON.stringify({ is_active: isActive }),
      }),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: ['vaktaware', 'enrollment-rules'] })
    },
  })
}

export function useDeleteEnrollmentRule() {
  const queryClient = useQueryClient()
  return useMutation<void, Error, string>({
    mutationFn: (id) =>
      apiFetch<void>(`${BASE}/enrollment-rules/${id}`, { method: 'DELETE' }),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: ['vaktaware', 'enrollment-rules'] })
    },
  })
}
