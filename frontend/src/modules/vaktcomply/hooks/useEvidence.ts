import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { apiFetch } from '../../../api/client'
import type { Evidence } from '../types'

export function useEvidence(controlId: string) {
  return useQuery<Evidence[]>({
    queryKey: ['vaktcomply', 'controls', controlId, 'evidence'],
    queryFn: () => apiFetch<Evidence[]>(`/vaktcomply/controls/${controlId}/evidence`),
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
  return useMutation<undefined, Error, ReviewPayload>({
    mutationFn: (payload) =>
      apiFetch<undefined>(`/vaktcomply/evidence/${evidenceId}/review`, {
        method: 'POST',
        body: JSON.stringify(payload),
      }),
    onSuccess: () => {
      void queryClient.invalidateQueries({
        queryKey: ['vaktcomply', 'controls', controlId, 'evidence'],
      })
    },
  })
}
