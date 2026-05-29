import { useQuery } from '@tanstack/react-query'
import { apiFetch } from '../../../api/client'

export interface ScoreHistoryEntry {
  date: string
  score: number
  controls_total: number
  controls_implemented: number
}

export function useScoreHistory(days: 30 | 90) {
  return useQuery<ScoreHistoryEntry[]>({
    queryKey: ['vaktcomply', 'score-history', days],
    queryFn: () => apiFetch<ScoreHistoryEntry[]>(`/vaktcomply/score-history?days=${String(days)}`),
    staleTime: 5 * 60_000,
  })
}
