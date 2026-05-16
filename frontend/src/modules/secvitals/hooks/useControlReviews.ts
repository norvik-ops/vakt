import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { apiFetch } from '../../../api/client'
import type { Control } from '../types'

export interface ControlReview {
  id: string
  control_id: string
  reviewed_by: string
  review_note: string
  status_at_review: string
  reviewed_at: string
}

export interface RecordReviewPayload {
  reviewed_by: string
  review_note?: string
  review_interval_days?: number
}

export function useControlReviews(controlId: string | undefined) {
  return useQuery<ControlReview[]>({
    queryKey: ['secvitals', 'controls', controlId, 'reviews'],
    queryFn: () => apiFetch<ControlReview[]>(`/secvitals/controls/${controlId}/reviews`),
    enabled: !!controlId,
    staleTime: 2 * 60 * 1000,
  })
}

export function useRecordControlReview(controlId: string) {
  const queryClient = useQueryClient()
  return useMutation<Control, Error, RecordReviewPayload>({
    mutationFn: (payload) =>
      apiFetch<Control>(`/secvitals/controls/${controlId}/review`, {
        method: 'POST',
        body: JSON.stringify(payload),
      }),
    onSuccess: () => {
      void queryClient.invalidateQueries({
        queryKey: ['secvitals', 'controls', controlId],
      })
      void queryClient.invalidateQueries({
        queryKey: ['secvitals', 'controls', controlId, 'reviews'],
      })
      void queryClient.invalidateQueries({
        queryKey: ['secvitals', 'controls', 'overdue'],
      })
    },
  })
}

export function useOverdueControls() {
  return useQuery<Control[]>({
    queryKey: ['secvitals', 'controls', 'overdue'],
    queryFn: () => apiFetch<Control[]>('/secvitals/controls/overdue-reviews'),
    staleTime: 5 * 60 * 1000,
  })
}
