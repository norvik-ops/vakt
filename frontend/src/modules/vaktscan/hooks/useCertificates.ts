import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { apiFetch } from '../../../api/client'
import type { Certificate } from '../types'

const QK = ['vaktscan', 'certificates'] as const

export function useCertificates() {
  return useQuery<{ data: Certificate[] }>({
    queryKey: [...QK],
    queryFn: () => apiFetch<{ data: Certificate[] }>('/vaktscan/certificates'),
    staleTime: 30_000,
  })
}

export interface CreateCertificateInput {
  domain: string
  asset_id?: string | null
  source?: 'manual' | 'scan'
}

export function useCreateCertificate() {
  const queryClient = useQueryClient()
  return useMutation<Certificate, Error, CreateCertificateInput>({
    mutationFn: (data) =>
      apiFetch<Certificate>('/vaktscan/certificates', {
        method: 'POST',
        body: JSON.stringify(data),
      }),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: [...QK] })
    },
  })
}

export function useDeleteCertificate() {
  const queryClient = useQueryClient()
  return useMutation<undefined, Error, string>({
    mutationFn: (id) =>
      apiFetch<undefined>(`/vaktscan/certificates/${id}`, { method: 'DELETE' }),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: [...QK] })
    },
  })
}

export function useScanCertificate() {
  const queryClient = useQueryClient()
  return useMutation<Certificate, Error, string>({
    mutationFn: (id) =>
      apiFetch<Certificate>(`/vaktscan/certificates/${id}/scan`, { method: 'POST' }),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: [...QK] })
    },
  })
}
