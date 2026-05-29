import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { apiFetch } from '../../../api/client'
import type { DORAThirdParty, CreateDORAThirdPartyInput, UpdateDORAThirdPartyInput } from '../types'

export function useDORAThirdParties(criticality?: string) {
  const params = criticality ? `?criticality=${criticality}` : ''
  return useQuery<DORAThirdParty[]>({
    queryKey: ['vaktcomply', 'dora-third-parties', criticality ?? 'all'],
    queryFn: () => apiFetch<DORAThirdParty[]>(`/vaktcomply/dora/third-parties${params}`),
    staleTime: 5 * 60 * 1000,
  })
}

export function useCreateDORAThirdParty() {
  const queryClient = useQueryClient()
  return useMutation<DORAThirdParty, Error, CreateDORAThirdPartyInput>({
    mutationFn: (input) =>
      apiFetch<DORAThirdParty>('/vaktcomply/dora/third-parties', {
        method: 'POST',
        body: JSON.stringify(input),
      }),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: ['vaktcomply', 'dora-third-parties'] })
    },
  })
}

export function useUpdateDORAThirdParty(id: string) {
  const queryClient = useQueryClient()
  return useMutation<DORAThirdParty, Error, UpdateDORAThirdPartyInput>({
    mutationFn: (input) =>
      apiFetch<DORAThirdParty>(`/vaktcomply/dora/third-parties/${id}`, {
        method: 'PATCH',
        body: JSON.stringify(input),
      }),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: ['vaktcomply', 'dora-third-parties'] })
    },
  })
}

export function useDeleteDORAThirdParty() {
  const queryClient = useQueryClient()
  return useMutation<undefined, Error, string>({
    mutationFn: (id) =>
      apiFetch<undefined>(`/vaktcomply/dora/third-parties/${id}`, { method: 'DELETE' }),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: ['vaktcomply', 'dora-third-parties'] })
    },
  })
}
