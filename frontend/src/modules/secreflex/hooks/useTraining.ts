import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { apiFetch } from '../../../api/client'
import type { TrainingModule, Assignment } from '../types'

const BASE = '/secreflex'

export function useTrainingModules() {
  return useQuery<TrainingModule[]>({
    queryKey: ['secreflex', 'training'],
    queryFn: () => apiFetch<TrainingModule[]>(`${BASE}/training`),
    staleTime: 60_000,
  })
}

export function useAssignments(moduleId: string) {
  return useQuery<Assignment[]>({
    queryKey: ['secreflex', 'training', moduleId, 'assignments'],
    queryFn: () => apiFetch<Assignment[]>(`${BASE}/training/${moduleId}/assignments`),
    staleTime: 30_000,
    enabled: Boolean(moduleId),
  })
}

export interface AssignModuleInput {
  user_emails: string[]
}

export function useAssignModule(moduleId: string) {
  const queryClient = useQueryClient()
  return useMutation<void, Error, AssignModuleInput>({
    mutationFn: (data) =>
      apiFetch<void>(`${BASE}/training/${moduleId}/assign`, {
        method: 'POST',
        body: JSON.stringify(data),
      }),
    onSuccess: () => {
      void queryClient.invalidateQueries({
        queryKey: ['secreflex', 'training', moduleId, 'assignments'],
      })
    },
  })
}
