import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { apiFetch } from '../../../api/client'
import type { CryptoKey, CreateCryptoKeyInput } from '../types'

export function useCryptoKeys() {
  return useQuery<CryptoKey[]>({
    queryKey: ['vaktcomply', 'crypto-keys'],
    queryFn: () => apiFetch<CryptoKey[]>('/vaktcomply/crypto-keys'),
    staleTime: 5 * 60 * 1000,
  })
}

export function useCreateCryptoKey() {
  const queryClient = useQueryClient()
  return useMutation<CryptoKey, Error, CreateCryptoKeyInput>({
    mutationFn: (input) =>
      apiFetch<CryptoKey>('/vaktcomply/crypto-keys', {
        method: 'POST',
        body: JSON.stringify(input),
      }),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: ['vaktcomply', 'crypto-keys'] })
    },
  })
}

interface RotateKeyInput {
  rotated_at: string
  rotation_interval_days?: number
  notes?: string
}

export function useRotateCryptoKey(keyId: string) {
  const queryClient = useQueryClient()
  return useMutation<CryptoKey, Error, RotateKeyInput>({
    mutationFn: (input) =>
      apiFetch<CryptoKey>(`/vaktcomply/crypto-keys/${keyId}/rotate`, {
        method: 'POST',
        body: JSON.stringify(input),
      }),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: ['vaktcomply', 'crypto-keys'] })
    },
  })
}

export function useDeleteCryptoKey() {
  const queryClient = useQueryClient()
  return useMutation<undefined, Error, string>({
    mutationFn: (id) =>
      apiFetch<undefined>(`/vaktcomply/crypto-keys/${id}`, { method: 'DELETE' }),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: ['vaktcomply', 'crypto-keys'] })
    },
  })
}
