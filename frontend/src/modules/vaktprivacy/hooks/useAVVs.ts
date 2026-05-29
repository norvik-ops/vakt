import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { apiFetch } from '../../../api/client'
import type { AVV, CreateAVVInput, UpdateAVVInput } from '../types'

export function useAVVs() {
  return useQuery<AVV[]>({
    queryKey: ['vaktprivacy', 'avvs'],
    queryFn: () => apiFetch<AVV[]>('/vaktprivacy/avvs'),
    staleTime: 5 * 60 * 1000,
  })
}

export function useCreateAVV() {
  const queryClient = useQueryClient()
  return useMutation<AVV, Error, CreateAVVInput>({
    mutationFn: (input) =>
      apiFetch<AVV>('/vaktprivacy/avvs', { method: 'POST', body: JSON.stringify(input) }),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: ['vaktprivacy', 'avvs'] })
    },
  })
}

export function useUpdateAVV() {
  const queryClient = useQueryClient()
  return useMutation<AVV, Error, { id: string; input: UpdateAVVInput }>({
    mutationFn: ({ id, input }) =>
      apiFetch<AVV>(`/vaktprivacy/avvs/${id}`, { method: 'PUT', body: JSON.stringify(input) }),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: ['vaktprivacy', 'avvs'] })
    },
  })
}

export function useDeleteAVV() {
  const queryClient = useQueryClient()
  return useMutation<undefined, Error, string>({
    mutationFn: (id) => apiFetch<undefined>(`/vaktprivacy/avvs/${id}`, { method: 'DELETE' }),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: ['vaktprivacy', 'avvs'] })
    },
  })
}
