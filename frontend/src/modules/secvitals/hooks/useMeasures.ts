import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { apiFetch } from '../../../api/client'

export interface ControlMeasure {
  id: string
  control_id: string
  org_id: string
  title: string
  description: string
  difficulty: 'easy' | 'medium' | 'hard'
  step_order: number
  is_builtin: boolean
  created_at: string
}

export interface CreateMeasureInput {
  title: string
  description: string
  difficulty: 'easy' | 'medium' | 'hard'
  step_order?: number
}

export interface UpdateMeasureInput {
  title?: string
  description?: string
  difficulty?: 'easy' | 'medium' | 'hard'
  step_order?: number
}

export function useMeasures(controlId: string | undefined) {
  return useQuery<ControlMeasure[]>({
    queryKey: ['measures', controlId],
    queryFn: () => apiFetch<ControlMeasure[]>(`/secvitals/controls/${controlId!}/measures`),
    enabled: !!controlId,
    staleTime: 5 * 60 * 1000,
  })
}

export function useCreateMeasure(controlId: string) {
  const queryClient = useQueryClient()
  return useMutation<ControlMeasure, Error, CreateMeasureInput>({
    mutationFn: (input) =>
      apiFetch<ControlMeasure>(`/secvitals/controls/${controlId}/measures`, {
        method: 'POST',
        body: JSON.stringify(input),
      }),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: ['measures', controlId] })
    },
  })
}

export function useUpdateMeasure(controlId: string) {
  const queryClient = useQueryClient()
  return useMutation<ControlMeasure, Error, { measureId: string; input: UpdateMeasureInput }>({
    mutationFn: ({ measureId, input }) =>
      apiFetch<ControlMeasure>(`/secvitals/controls/${controlId}/measures/${measureId}`, {
        method: 'PATCH',
        body: JSON.stringify(input),
      }),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: ['measures', controlId] })
    },
  })
}

export function useDeleteMeasure(controlId: string) {
  const queryClient = useQueryClient()
  return useMutation<void, Error, string>({
    mutationFn: (measureId) =>
      apiFetch<void>(`/secvitals/controls/${controlId}/measures/${measureId}`, { method: 'DELETE' }),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: ['measures', controlId] })
    },
  })
}
