import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { apiFetch } from '../../../api/client'
import type { CCMCheck, CCMResult, CreateCCMCheckInput } from '../types'

export function useCCMChecks() {
  return useQuery<CCMCheck[]>({
    queryKey: ['secvitals', 'ccm', 'checks'],
    queryFn: () => apiFetch<CCMCheck[]>('/secvitals/ccm/checks'),
    staleTime: 30 * 1000,
  })
}

export function useCreateCCMCheck() {
  const queryClient = useQueryClient()
  return useMutation<CCMCheck, Error, CreateCCMCheckInput>({
    mutationFn: (input) =>
      apiFetch<CCMCheck>('/secvitals/ccm/checks', {
        method: 'POST',
        body: JSON.stringify(input),
      }),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: ['secvitals', 'ccm', 'checks'] })
    },
  })
}

export function useDeleteCCMCheck() {
  const queryClient = useQueryClient()
  return useMutation<void, Error, string>({
    mutationFn: (id) =>
      apiFetch<void>(`/secvitals/ccm/checks/${id}`, { method: 'DELETE' }),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: ['secvitals', 'ccm', 'checks'] })
    },
  })
}

export function useToggleCCMCheck() {
  const queryClient = useQueryClient()
  return useMutation<void, Error, { id: string; enabled: boolean }>({
    mutationFn: ({ id, enabled }) =>
      apiFetch<void>(`/secvitals/ccm/checks/${id}/toggle`, {
        method: 'PATCH',
        body: JSON.stringify({ enabled }),
      }),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: ['secvitals', 'ccm', 'checks'] })
    },
  })
}

export function useTriggerCCMCheck() {
  const queryClient = useQueryClient()
  return useMutation<CCMResult, Error, string>({
    mutationFn: (id) =>
      apiFetch<CCMResult>(`/secvitals/ccm/checks/${id}/run`, { method: 'POST' }),
    onSuccess: (_, id) => {
      void queryClient.invalidateQueries({ queryKey: ['secvitals', 'ccm', 'checks'] })
      void queryClient.invalidateQueries({ queryKey: ['secvitals', 'ccm', 'results', id] })
    },
  })
}

export function useCCMResults(checkId: string) {
  return useQuery<CCMResult[]>({
    queryKey: ['secvitals', 'ccm', 'results', checkId],
    queryFn: () => apiFetch<CCMResult[]>(`/secvitals/ccm/checks/${checkId}/results`),
    enabled: !!checkId,
    staleTime: 30 * 1000,
  })
}
