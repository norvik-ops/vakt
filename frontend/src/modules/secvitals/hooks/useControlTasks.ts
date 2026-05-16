import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { apiFetch } from '../../../api/client'
import type { ControlTask } from '../types'

export function useControlTasks(controlId: string) {
  return useQuery<ControlTask[]>({
    queryKey: ['secvitals', 'controls', controlId, 'tasks'],
    queryFn: () => apiFetch<ControlTask[]>(`/secvitals/controls/${controlId}/tasks`),
    enabled: !!controlId,
    staleTime: 5 * 60 * 1000,
  })
}

export function useCreateControlTask(controlId: string) {
  const queryClient = useQueryClient()
  return useMutation<ControlTask, Error, { text: string }>({
    mutationFn: (input) =>
      apiFetch<ControlTask>(`/secvitals/controls/${controlId}/tasks`, {
        method: 'POST',
        body: JSON.stringify(input),
      }),
    onSuccess: () => {
      void queryClient.invalidateQueries({
        queryKey: ['secvitals', 'controls', controlId, 'tasks'],
      })
    },
  })
}

export function useToggleControlTask(controlId: string) {
  const queryClient = useQueryClient()
  return useMutation<ControlTask, Error, { taskId: string; completed: boolean }>({
    mutationFn: ({ taskId, completed }) =>
      apiFetch<ControlTask>(`/secvitals/controls/${controlId}/tasks/${taskId}`, {
        method: 'PATCH',
        body: JSON.stringify({ completed }),
      }),
    onSuccess: () => {
      void queryClient.invalidateQueries({
        queryKey: ['secvitals', 'controls', controlId, 'tasks'],
      })
    },
  })
}

export function useDeleteControlTask(controlId: string) {
  const queryClient = useQueryClient()
  return useMutation<void, Error, string>({
    mutationFn: (taskId) =>
      apiFetch<void>(`/secvitals/controls/${controlId}/tasks/${taskId}`, { method: 'DELETE' }),
    onSuccess: () => {
      void queryClient.invalidateQueries({
        queryKey: ['secvitals', 'controls', controlId, 'tasks'],
      })
    },
  })
}
