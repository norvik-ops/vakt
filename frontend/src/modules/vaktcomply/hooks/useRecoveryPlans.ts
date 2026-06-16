import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { apiFetch } from '../../../api/client'
import type {
  RecoveryPlan,
  CreateRecoveryPlanInput,
  UpdateRecoveryPlanInput,
} from '../types'

const QK = ['vaktcomply', 'bcm', 'recovery-plans'] as const

export function useRecoveryPlans(biaId?: string) {
  const url = biaId
    ? `/vaktcomply/bcm/recovery-plans?bia_id=${biaId}`
    : '/vaktcomply/bcm/recovery-plans'
  return useQuery<RecoveryPlan[]>({
    queryKey: biaId ? [...QK, 'byBia', biaId] : [...QK, 'all'],
    queryFn: () => apiFetch<RecoveryPlan[]>(url),
    staleTime: 5 * 60 * 1000,
  })
}

export function useCreateRecoveryPlan() {
  const queryClient = useQueryClient()
  return useMutation<RecoveryPlan, Error, CreateRecoveryPlanInput>({
    mutationFn: (input) =>
      apiFetch<RecoveryPlan>('/vaktcomply/bcm/recovery-plans', {
        method: 'POST',
        body: JSON.stringify(input),
      }),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: QK })
      void queryClient.invalidateQueries({ queryKey: ['vaktcomply', 'bcm', 'score'] })
    },
  })
}

export function useUpdateRecoveryPlan(id: string) {
  const queryClient = useQueryClient()
  return useMutation<RecoveryPlan, Error, UpdateRecoveryPlanInput>({
    mutationFn: (input) =>
      apiFetch<RecoveryPlan>(`/vaktcomply/bcm/recovery-plans/${id}`, {
        method: 'PUT',
        body: JSON.stringify(input),
      }),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: QK })
      void queryClient.invalidateQueries({ queryKey: ['vaktcomply', 'bcm', 'score'] })
    },
  })
}

export function useDeleteRecoveryPlan() {
  const queryClient = useQueryClient()
  return useMutation<undefined, Error, string>({
    mutationFn: (id) =>
      apiFetch<undefined>(`/vaktcomply/bcm/recovery-plans/${id}`, { method: 'DELETE' }),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: QK })
      void queryClient.invalidateQueries({ queryKey: ['vaktcomply', 'bcm', 'score'] })
    },
  })
}
