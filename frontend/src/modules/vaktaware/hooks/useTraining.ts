import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { apiFetch } from '../../../api/client'
import type { TrainingModule, Assignment } from '../types'

const BASE = '/vaktaware'

export function useTrainingModules() {
  return useQuery<TrainingModule[]>({
    queryKey: ['vaktaware', 'training'],
    queryFn: () => apiFetch<TrainingModule[]>(`${BASE}/training-modules`),
    staleTime: 60_000,
  })
}

export function useAssignments(moduleId: string) {
  return useQuery<Assignment[]>({
    queryKey: ['vaktaware', 'training', moduleId, 'assignments'],
    queryFn: () => apiFetch<Assignment[]>(`${BASE}/training-modules/${moduleId}/assignments`),
    staleTime: 30_000,
    enabled: Boolean(moduleId),
  })
}

export interface AssignModuleInput {
  user_emails: string[]
}

export function useAssignModule(moduleId: string) {
  const queryClient = useQueryClient()
  return useMutation<undefined, Error, AssignModuleInput>({
    mutationFn: (data) =>
      apiFetch<undefined>(`${BASE}/training-modules/${moduleId}/assign`, {
        method: 'POST',
        body: JSON.stringify(data),
      }),
    onSuccess: () => {
      void queryClient.invalidateQueries({
        queryKey: ['vaktaware', 'training', moduleId, 'assignments'],
      })
    },
  })
}
