import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { apiFetch } from '../../../api/client'
import type { AuditRecord, CreateAuditRecordInput, UpdateAuditRecordInput } from '../types'

export function useAuditRecords() {
  return useQuery<AuditRecord[]>({
    queryKey: ['secvitals', 'audits'],
    queryFn: () => apiFetch<AuditRecord[]>('/secvitals/audits'),
    staleTime: 5 * 60 * 1000,
  })
}

export function useAuditRecord(id: string) {
  return useQuery<AuditRecord>({
    queryKey: ['secvitals', 'audits', id],
    queryFn: () => apiFetch<AuditRecord>(`/secvitals/audits/${id}`),
    enabled: !!id,
    staleTime: 5 * 60 * 1000,
  })
}

export function useCreateAuditRecord() {
  const queryClient = useQueryClient()
  return useMutation<AuditRecord, Error, CreateAuditRecordInput>({
    mutationFn: (input) =>
      apiFetch<AuditRecord>('/secvitals/audits', { method: 'POST', body: JSON.stringify(input) }),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: ['secvitals', 'audits'] })
    },
  })
}

export function useUpdateAuditRecord(id: string) {
  const queryClient = useQueryClient()
  return useMutation<AuditRecord, Error, UpdateAuditRecordInput>({
    mutationFn: (input) =>
      apiFetch<AuditRecord>(`/secvitals/audits/${id}`, { method: 'PATCH', body: JSON.stringify(input) }),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: ['secvitals', 'audits'] })
      void queryClient.invalidateQueries({ queryKey: ['secvitals', 'audits', id] })
    },
  })
}
