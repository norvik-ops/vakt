import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { apiFetch } from '../../../api/client'
import type { AIInsight } from '../types'

// S131-D3 (R-C09/D18-01): the endpoint returns a bare AIInsight[] array (see
// openapi.yaml: /vaktcomply/ai/insights → type: array), NOT { items: [] }. The
// old {items} typing meant every consumer read data.items === undefined:
// FindingDetailPage crashed on `.filter` (no optional chain), and AIInsightsFeed
// silently rendered empty (`data?.items ?? []` is always []). Type it as the
// array it actually is.
export function useAIInsights() {
  return useQuery<AIInsight[]>({
    queryKey: ['vaktcomply', 'ai-insights'],
    queryFn: () => apiFetch<AIInsight[]>('/vaktcomply/ai/insights'),
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
