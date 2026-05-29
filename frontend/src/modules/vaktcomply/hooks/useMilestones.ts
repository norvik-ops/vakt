import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { apiFetch } from '../../../api/client'
import type { AuditMilestone, CreateMilestoneInput, UpdateMilestoneInput } from '../types'

const QK = ['vaktcomply', 'milestones'] as const

export function useMilestones(statusFilter?: string) {
  const params = statusFilter ? `?status=${statusFilter}` : ''
  return useQuery<AuditMilestone[]>({
    queryKey: [...QK, statusFilter ?? 'all'],
    queryFn: () => apiFetch<AuditMilestone[]>(`/vaktcomply/milestones${params}`),
    staleTime: 2 * 60 * 1000,
  })
}

export function useNextMilestone() {
  return useQuery<AuditMilestone | null>({
    queryKey: [...QK, 'next'],
    queryFn: () => apiFetch<AuditMilestone | null>('/vaktcomply/milestones/next'),
    staleTime: 5 * 60 * 1000,
  })
}

export function useCreateMilestone() {
  const queryClient = useQueryClient()
  return useMutation<AuditMilestone, Error, CreateMilestoneInput>({
    mutationFn: (input) =>
      apiFetch<AuditMilestone>('/vaktcomply/milestones', { method: 'POST', body: JSON.stringify(input) }),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: QK })
    },
  })
}

export function useUpdateMilestone(id: string) {
  const queryClient = useQueryClient()
  return useMutation<AuditMilestone, Error, UpdateMilestoneInput>({
    mutationFn: (input) =>
      apiFetch<AuditMilestone>(`/vaktcomply/milestones/${id}`, { method: 'PUT', body: JSON.stringify(input) }),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: QK })
    },
  })
}

export function useDeleteMilestone() {
  const queryClient = useQueryClient()
  return useMutation<undefined, Error, string>({
    mutationFn: (id) => apiFetch<undefined>(`/vaktcomply/milestones/${id}`, { method: 'DELETE' }),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: QK })
    },
  })
}
