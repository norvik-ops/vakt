import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { apiFetch } from '../../../api/client'
import type { DSR, CreateDSRInput, UpdateDSRInput } from '../types'

/**
 * Fetches all Data Subject Requests (Betroffenenanfragen) for the current organisation.
 * DSRs are rights exercised by individuals under DSGVO Art. 15–21 (access, erasure, etc.).
 */
export function useDSRs() {
  return useQuery<DSR[]>({
    queryKey: ['secprivacy', 'dsr'],
    queryFn: () => apiFetch<DSR[]>('/secprivacy/dsr'),
    staleTime: 5 * 60 * 1000,
  })
}

/**
 * Creates a new DSR record.
 * The server automatically sets `due_date` to 30 days after receipt, as required by
 * DSGVO Art. 12 Abs. 3 — callers must not supply a due date themselves.
 */
export function useCreateDSR() {
  const queryClient = useQueryClient()
  return useMutation<DSR, Error, CreateDSRInput>({
    mutationFn: (input) =>
      apiFetch<DSR>('/secprivacy/dsr', { method: 'POST', body: JSON.stringify(input) }),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: ['secprivacy', 'dsr'] })
    },
  })
}

/**
 * Updates the status or internal notes of an existing DSR.
 * Only `status` and `notes` are mutable after creation — requester identity and
 * type are immutable to preserve the audit trail.
 */
export function useUpdateDSR() {
  const queryClient = useQueryClient()
  return useMutation<DSR, Error, { id: string; input: UpdateDSRInput }>({
    mutationFn: ({ id, input }) =>
      apiFetch<DSR>(`/secprivacy/dsr/${id}`, { method: 'PUT', body: JSON.stringify(input) }),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: ['secprivacy', 'dsr'] })
    },
  })
}

/**
 * Permanently deletes a DSR record after user confirmation.
 * DSGVO Art. 5 Abs. 2 (Rechenschaftspflicht) requires retaining evidence of
 * processing, so deletion should only be used for erroneous entries.
 */
export function useDeleteDSR() {
  const queryClient = useQueryClient()
  return useMutation<void, Error, string>({
    mutationFn: (id) => apiFetch<void>(`/secprivacy/dsr/${id}`, { method: 'DELETE' }),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: ['secprivacy', 'dsr'] })
    },
  })
}
