import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { apiFetch } from '../../../api/client'
import type { ISMSScope, CreateISMSScopeInput } from '../types'

const QK = ['vaktcomply', 'isms-scope'] as const

export function useISMSScope() {
  return useQuery<ISMSScope | null>({
    queryKey: [...QK, 'current'],
    queryFn: () => apiFetch<ISMSScope | null>('/vaktcomply/isms-scope'),
    staleTime: 5 * 60 * 1000,
  })
}

export function useISMSScopeVersions() {
  return useQuery<ISMSScope[]>({
    queryKey: [...QK, 'versions'],
    queryFn: () => apiFetch<ISMSScope[]>('/vaktcomply/isms-scope/versions'),
    staleTime: 5 * 60 * 1000,
  })
}

export function useSaveISMSScope() {
  const queryClient = useQueryClient()
  return useMutation<ISMSScope, Error, CreateISMSScopeInput>({
    mutationFn: (input) =>
      apiFetch<ISMSScope>('/vaktcomply/isms-scope', {
        method: 'POST',
        body: JSON.stringify(input),
      }),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: [...QK, 'current'] })
      void queryClient.invalidateQueries({ queryKey: [...QK, 'versions'] })
    },
  })
}

export function useApproveISMSScope() {
  const queryClient = useQueryClient()
  return useMutation<ISMSScope, Error, { id: string }>({
    mutationFn: (body) =>
      apiFetch<ISMSScope>('/vaktcomply/isms-scope/approve', {
        method: 'POST',
        body: JSON.stringify(body),
      }),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: [...QK, 'current'] })
      void queryClient.invalidateQueries({ queryKey: [...QK, 'versions'] })
    },
  })
}
