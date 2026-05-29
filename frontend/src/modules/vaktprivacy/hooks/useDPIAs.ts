import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { apiFetch, FeatureLockedError } from '../../../api/client'
import type { DPIA, CreateDPIAInput, UpdateDPIAInput } from '../types'

export function useDPIAs() {
  return useQuery<DPIA[]>({
    queryKey: ['vaktprivacy', 'dpias'],
    queryFn: () => apiFetch<DPIA[]>('/vaktprivacy/dpias'),
    staleTime: 5 * 60 * 1000,
  })
}

export function useCreateDPIA() {
  const queryClient = useQueryClient()
  return useMutation<DPIA, Error, CreateDPIAInput>({
    mutationFn: (input) =>
      apiFetch<DPIA>('/vaktprivacy/dpias', { method: 'POST', body: JSON.stringify(input) }),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: ['vaktprivacy', 'dpias'] })
    },
  })
}

export function useUpdateDPIA() {
  const queryClient = useQueryClient()
  return useMutation<DPIA, Error, { id: string; input: UpdateDPIAInput }>({
    mutationFn: ({ id, input }) =>
      apiFetch<DPIA>(`/vaktprivacy/dpias/${id}`, { method: 'PUT', body: JSON.stringify(input) }),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: ['vaktprivacy', 'dpias'] })
    },
  })
}

export function useApproveDPIA() {
  const queryClient = useQueryClient()
  return useMutation<DPIA, Error, string>({
    mutationFn: (id) =>
      apiFetch<DPIA>(`/vaktprivacy/dpias/${id}/approve`, { method: 'POST' }),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: ['vaktprivacy', 'dpias'] })
    },
  })
}

export function useDeleteDPIA() {
  const queryClient = useQueryClient()
  return useMutation<undefined, Error, string>({
    mutationFn: (id) => apiFetch<undefined>(`/vaktprivacy/dpias/${id}`, { method: 'DELETE' }),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: ['vaktprivacy', 'dpias'] })
    },
  })
}

export function useExportDPIA() {
  return async (): Promise<void> => {
    const url = '/api/v1/vaktprivacy/dpias/export'
    const res = await fetch(url, { credentials: 'include' })
    if (res.status === 402) {
      const body = (await res.json().catch(() => ({}))) as { feature?: string }
      throw new FeatureLockedError(body.feature ?? 'audit_pdf')
    }
    if (!res.ok) {
      throw new Error(`Export failed: ${res.statusText}`)
    }
    const blob = await res.blob()
    const objectUrl = URL.createObjectURL(blob)
    const a = document.createElement('a')
    a.href = objectUrl
    a.download = `dpia-export-${new Date().toISOString().slice(0, 10)}.pdf`
    document.body.appendChild(a)
    a.click()
    a.remove()
    URL.revokeObjectURL(objectUrl)
  }
}
