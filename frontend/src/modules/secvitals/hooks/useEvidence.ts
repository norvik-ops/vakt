import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { apiFetch } from '../../../api/client'
import type { Evidence } from '../types'

export function useEvidence(controlId: string) {
  return useQuery<Evidence[]>({
    queryKey: ['secvitals', 'controls', controlId, 'evidence'],
    queryFn: () => apiFetch<Evidence[]>(`/secvitals/controls/${controlId}/evidence`),
    enabled: !!controlId,
    staleTime: 5 * 60 * 1000,
  })
}

interface ReviewPayload {
  status: 'approved' | 'rejected'
  notes?: string
}

export function useReviewEvidence(evidenceId: string, controlId: string) {
  const queryClient = useQueryClient()
  return useMutation<void, Error, ReviewPayload>({
    mutationFn: (payload) =>
      apiFetch<void>(`/secvitals/evidence/${evidenceId}/review`, {
        method: 'POST',
        body: JSON.stringify(payload),
      }),
    onSuccess: () => {
      void queryClient.invalidateQueries({
        queryKey: ['secvitals', 'controls', controlId, 'evidence'],
      })
    },
  })
}
