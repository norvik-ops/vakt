import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { apiFetch } from '../../../api/client'
import type { AccessToken } from '../types'

const BASE = '/secvault'

export function useTokens() {
  return useQuery<AccessToken[]>({
    queryKey: ['secvault', 'tokens'],
    queryFn: () => apiFetch<AccessToken[]>(`${BASE}/tokens`),
    staleTime: 30_000,
  })
}

export interface CreateTokenInput {
  name: string
  scopes: string[]
}

export interface CreateTokenResponse extends AccessToken {
  token: string
}

export function useCreateToken() {
  const queryClient = useQueryClient()
  return useMutation<CreateTokenResponse, Error, CreateTokenInput>({
    mutationFn: (data) =>
      apiFetch<CreateTokenResponse>(`${BASE}/tokens`, {
        method: 'POST',
        body: JSON.stringify(data),
      }),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: ['secvault', 'tokens'] })
    },
  })
}

export function useDeleteToken() {
  const queryClient = useQueryClient()
  return useMutation<void, Error, string>({
    mutationFn: (id) =>
      apiFetch<void>(`${BASE}/tokens/${id}`, { method: 'DELETE' }),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: ['secvault', 'tokens'] })
    },
  })
}
