import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { apiFetch } from '../../../api/client'
import type { TargetGroup, Target } from '../types'

const BASE = '/vaktaware'

export function useTargetGroups() {
  return useQuery<TargetGroup[]>({
    queryKey: ['vaktaware', 'target-groups'],
    queryFn: () => apiFetch<TargetGroup[]>(`${BASE}/groups`),
    staleTime: 30_000,
  })
}

export function useTargets(groupId: string) {
  return useQuery<Target[]>({
    queryKey: ['vaktaware', 'target-groups', groupId, 'targets'],
    queryFn: () => apiFetch<Target[]>(`${BASE}/groups/${groupId}/targets`),
    staleTime: 30_000,
    enabled: Boolean(groupId),
  })
}

export interface CreateTargetGroupInput {
  name: string
  source: string
}

export function useCreateTargetGroup() {
  const queryClient = useQueryClient()
  return useMutation<TargetGroup, Error, CreateTargetGroupInput>({
    mutationFn: (data) =>
      apiFetch<TargetGroup>(`${BASE}/groups`, {
        method: 'POST',
        body: JSON.stringify(data),
      }),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: ['vaktaware', 'target-groups'] })
    },
  })
}

export function useDeleteTargetGroup() {
  const queryClient = useQueryClient()
  return useMutation<undefined, Error, string>({
    mutationFn: (id) =>
      apiFetch<undefined>(`${BASE}/target-groups/${id}`, { method: 'DELETE' }),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: ['vaktaware', 'target-groups'] })
    },
  })
}

export function useAddTarget(groupId: string) {
  const queryClient = useQueryClient()
  return useMutation<Target, Error, Omit<Target, 'id'>>({
    mutationFn: (data) =>
      apiFetch<Target>(`${BASE}/target-groups/${groupId}/targets`, {
        method: 'POST',
        body: JSON.stringify(data),
      }),
    onSuccess: () => {
      void queryClient.invalidateQueries({
        queryKey: ['vaktaware', 'target-groups', groupId, 'targets'],
      })
      void queryClient.invalidateQueries({ queryKey: ['vaktaware', 'target-groups'] })
    },
  })
}
