import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { apiFetch } from '../../../api/client'
import type { CCMCheck, CCMResult, CreateCCMCheckInput } from '../types'

export function useCCMChecks() {
  return useQuery<CCMCheck[]>({
    queryKey: ['vaktcomply', 'ccm', 'checks'],
    queryFn: () => apiFetch<CCMCheck[]>('/vaktcomply/ccm/checks'),
    staleTime: 30 * 1000,
  })
}

export function useCreateCCMCheck() {
  const queryClient = useQueryClient()
  return useMutation<CCMCheck, Error, CreateCCMCheckInput>({
    mutationFn: (input) =>
      apiFetch<CCMCheck>('/vaktcomply/ccm/checks', {
        method: 'POST',
        body: JSON.stringify(input),
      }),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: ['vaktcomply', 'ccm', 'checks'] })
    },
  })
}

export function useDeleteCCMCheck() {
  const queryClient = useQueryClient()
  return useMutation<undefined, Error, string>({
    mutationFn: (id) =>
      apiFetch<undefined>(`/vaktcomply/ccm/checks/${id}`, { method: 'DELETE' }),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: ['vaktcomply', 'ccm', 'checks'] })
    },
  })
}

export function useToggleCCMCheck() {
  const queryClient = useQueryClient()
  return useMutation<undefined, Error, { id: string; enabled: boolean }>({
    mutationFn: ({ id, enabled }) =>
      apiFetch<undefined>(`/vaktcomply/ccm/checks/${id}/toggle`, {
        method: 'PATCH',
        body: JSON.stringify({ enabled }),
      }),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: ['vaktcomply', 'ccm', 'checks'] })
    },
  })
}

export function useTriggerCCMCheck() {
  const queryClient = useQueryClient()
  return useMutation<CCMResult, Error, string>({
    mutationFn: (id) =>
      apiFetch<CCMResult>(`/vaktcomply/ccm/checks/${id}/run`, { method: 'POST' }),
    onSuccess: (_, id) => {
      void queryClient.invalidateQueries({ queryKey: ['vaktcomply', 'ccm', 'checks'] })
      void queryClient.invalidateQueries({ queryKey: ['vaktcomply', 'ccm', 'results', id] })
    },
  })
}

export function useCCMResults(checkId: string) {
  return useQuery<CCMResult[]>({
    queryKey: ['vaktcomply', 'ccm', 'results', checkId],
    queryFn: () => apiFetch<CCMResult[]>(`/vaktcomply/ccm/checks/${checkId}/results`),
    enabled: !!checkId,
    staleTime: 30 * 1000,
  })
}
