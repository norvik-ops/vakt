import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { apiFetch } from '../../../api/client'
import type {
  BIAProcess,
  BIASummary,
  CreateBIAProcessInput,
  UpdateBIAProcessInput,
} from '../types'

const QK = ['vaktcomply', 'bia'] as const

export function useBIASummary() {
  return useQuery<BIASummary>({
    queryKey: [...QK, 'summary'],
    queryFn: () => apiFetch<BIASummary>('/vaktcomply/bia/summary'),
    staleTime: 5 * 60 * 1000,
  })
}

export function useBIAProcesses() {
  return useQuery<BIAProcess[]>({
    queryKey: [...QK, 'processes'],
    queryFn: () => apiFetch<BIAProcess[]>('/vaktcomply/bia/processes'),
    staleTime: 5 * 60 * 1000,
  })
}

export function useCreateBIAProcess() {
  const queryClient = useQueryClient()
  return useMutation<BIAProcess, Error, CreateBIAProcessInput>({
    mutationFn: (input) =>
      apiFetch<BIAProcess>('/vaktcomply/bia/processes', {
        method: 'POST',
        body: JSON.stringify(input),
      }),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: [...QK, 'processes'] })
      void queryClient.invalidateQueries({ queryKey: [...QK, 'summary'] })
      void queryClient.invalidateQueries({ queryKey: ['vaktcomply', 'bcm', 'score'] })
    },
  })
}

export function useUpdateBIAProcess(id: string) {
  const queryClient = useQueryClient()
  return useMutation<BIAProcess, Error, UpdateBIAProcessInput>({
    mutationFn: (input) =>
      apiFetch<BIAProcess>(`/vaktcomply/bia/processes/${id}`, {
        method: 'PUT',
        body: JSON.stringify(input),
      }),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: [...QK, 'processes'] })
      void queryClient.invalidateQueries({ queryKey: [...QK, 'summary'] })
      void queryClient.invalidateQueries({ queryKey: ['vaktcomply', 'bcm', 'score'] })
    },
  })
}

export function useDeleteBIAProcess() {
  const queryClient = useQueryClient()
  return useMutation<undefined, Error, string>({
    mutationFn: (id) =>
      apiFetch<undefined>(`/vaktcomply/bia/processes/${id}`, { method: 'DELETE' }),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: [...QK, 'processes'] })
      void queryClient.invalidateQueries({ queryKey: [...QK, 'summary'] })
      void queryClient.invalidateQueries({ queryKey: ['vaktcomply', 'bcm', 'score'] })
    },
  })
}
