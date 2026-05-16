import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { apiFetch } from '../../../api/client'
import type { AnswerWithReview, SupplierStatus, ReviewAnswerInput } from '../types'

export function useAssessmentAnswers(assessmentId: string) {
  return useQuery<AnswerWithReview[]>({
    queryKey: ['secvitals', 'assessments', assessmentId, 'answers'],
    queryFn: () => apiFetch<AnswerWithReview[]>(`/secvitals/assessments/${assessmentId}/answers`),
    enabled: !!assessmentId,
    staleTime: 5 * 60 * 1000,
  })
}

export function useReviewAnswer(assessmentId: string) {
  const queryClient = useQueryClient()
  return useMutation<{ ok: boolean; evidence_id?: string }, Error, { answerId: string; input: ReviewAnswerInput }>({
    mutationFn: ({ answerId, input }) =>
      apiFetch<{ ok: boolean; evidence_id?: string }>(
        `/secvitals/assessments/${assessmentId}/answers/${answerId}`,
        { method: 'PATCH', body: JSON.stringify(input) },
      ),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: ['secvitals', 'assessments', assessmentId, 'answers'] })
    },
  })
}

export function useFinalizeAssessment() {
  const queryClient = useQueryClient()
  return useMutation<{ ok: boolean }, Error, string>({
    mutationFn: (assessmentId) =>
      apiFetch<{ ok: boolean }>(`/secvitals/assessments/${assessmentId}`, {
        method: 'PATCH',
        body: JSON.stringify({ status: 'reviewed' }),
      }),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: ['secvitals', 'assessments'] })
      void queryClient.invalidateQueries({ queryKey: ['secvitals', 'suppliers'] })
    },
  })
}

export function useSupplierStatus(supplierId: string) {
  return useQuery<SupplierStatus>({
    queryKey: ['secvitals', 'suppliers', supplierId, 'status'],
    queryFn: () => apiFetch<SupplierStatus>(`/secvitals/suppliers/${supplierId}/status`),
    enabled: !!supplierId,
    staleTime: 5 * 60 * 1000,
  })
}

export type BadgeVariant = 'destructive' | 'warning' | 'success' | 'default'

export function statusToVariant(status: 'green' | 'yellow' | 'red'): BadgeVariant {
  const map: Record<string, BadgeVariant> = {
    green: 'success',
    yellow: 'warning',
    red: 'destructive',
  }
  return map[status] ?? 'default'
}
