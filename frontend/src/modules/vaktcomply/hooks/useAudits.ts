import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { apiFetch } from '../../../api/client'
import type { AuditRecord, CreateAuditRecordInput, UpdateAuditRecordInput } from '../types'

export function useAuditRecords() {
  return useQuery<AuditRecord[]>({
    queryKey: ['vaktcomply', 'audits'],
    queryFn: () => apiFetch<AuditRecord[]>('/vaktcomply/audits'),
    staleTime: 5 * 60 * 1000,
  })
}

export function useAuditRecord(id: string) {
  return useQuery<AuditRecord>({
    queryKey: ['vaktcomply', 'audits', id],
    queryFn: () => apiFetch<AuditRecord>(`/vaktcomply/audits/${id}`),
    enabled: !!id,
    staleTime: 5 * 60 * 1000,
  })
}

export function useCreateAuditRecord() {
  const queryClient = useQueryClient()
  return useMutation<AuditRecord, Error, CreateAuditRecordInput>({
    mutationFn: (input) =>
      apiFetch<AuditRecord>('/vaktcomply/audits', { method: 'POST', body: JSON.stringify(input) }),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: ['vaktcomply', 'audits'] })
    },
  })
}

export function useUpdateAuditRecord(id: string) {
  const queryClient = useQueryClient()
  return useMutation<AuditRecord, Error, UpdateAuditRecordInput>({
    mutationFn: (input) =>
      apiFetch<AuditRecord>(`/vaktcomply/audits/${id}`, { method: 'PATCH', body: JSON.stringify(input) }),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: ['vaktcomply', 'audits'] })
      void queryClient.invalidateQueries({ queryKey: ['vaktcomply', 'audits', id] })
    },
  })
}
