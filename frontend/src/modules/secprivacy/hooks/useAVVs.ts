import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { apiFetch } from '../../../api/client'
import type { AVV, CreateAVVInput, UpdateAVVInput } from '../types'

export function useAVVs() {
  return useQuery<AVV[]>({
    queryKey: ['secprivacy', 'avvs'],
    queryFn: () => apiFetch<AVV[]>('/secprivacy/avvs'),
    staleTime: 5 * 60 * 1000,
  })
}

export function useCreateAVV() {
  const queryClient = useQueryClient()
  return useMutation<AVV, Error, CreateAVVInput>({
    mutationFn: (input) =>
      apiFetch<AVV>('/secprivacy/avvs', { method: 'POST', body: JSON.stringify(input) }),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: ['secprivacy', 'avvs'] })
    },
  })
}

export function useUpdateAVV() {
  const queryClient = useQueryClient()
  return useMutation<AVV, Error, { id: string; input: UpdateAVVInput }>({
    mutationFn: ({ id, input }) =>
      apiFetch<AVV>(`/secprivacy/avvs/${id}`, { method: 'PUT', body: JSON.stringify(input) }),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: ['secprivacy', 'avvs'] })
    },
  })
}

export function useDeleteAVV() {
  const queryClient = useQueryClient()
  return useMutation<void, Error, string>({
    mutationFn: (id) => apiFetch<void>(`/secprivacy/avvs/${id}`, { method: 'DELETE' }),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: ['secprivacy', 'avvs'] })
    },
  })
}
