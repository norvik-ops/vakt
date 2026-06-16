import { useQuery } from '@tanstack/react-query'
import { apiFetch } from '../../../api/client'
import type { BCMReadinessScore } from '../types'

export function useBCMReadinessScore() {
  return useQuery<BCMReadinessScore>({
    queryKey: ['vaktcomply', 'bcm', 'score'],
    queryFn: () => apiFetch<BCMReadinessScore>('/vaktcomply/bcm/readiness-score'),
    staleTime: 2 * 60 * 1000,
  })
}
