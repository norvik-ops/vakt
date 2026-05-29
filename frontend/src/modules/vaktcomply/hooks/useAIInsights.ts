import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { apiFetch } from '../../../api/client'
import type { AIInsight } from '../types'

export function useAIInsights() {
  return useQuery<{ items: AIInsight[] }>({
    queryKey: ['vaktcomply', 'ai-insights'],
    queryFn: () => apiFetch<{ items: AIInsight[] }>('/vaktcomply/ai/insights'),
    staleTime: 5 * 60 * 1000,
  })
}

export function useDismissInsight() {
  const queryClient = useQueryClient()
  return useMutation<void, Error, string>({
    mutationFn: (id) => apiFetch<void>(`/vaktcomply/ai/insights/${id}`, { method: 'DELETE' }),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: ['vaktcomply', 'ai-insights'] })
    },
  })
}

export function useRiskNarrative(riskId: string) {
  const queryClient = useQueryClient()
  return useMutation<{ narrative: string }>({
    mutationFn: () => apiFetch<{ narrative: string }>(`/vaktcomply/ai/risks/${riskId}/narrative`, { method: 'POST' }),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: ['vaktcomply', 'risks', riskId] })
    },
  })
}
