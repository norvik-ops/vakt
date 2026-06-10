import { useQuery } from '@tanstack/react-query'
import { apiFetch } from '../../../api/client'
import type { BSIOrp3Compliance } from '../types'

const BASE = '/vaktaware'

export function useORP3Status() {
  return useQuery<BSIOrp3Compliance>({
    queryKey: ['vaktaware', 'bsi-orp3-status'],
    queryFn: () => apiFetch<BSIOrp3Compliance>(`${BASE}/bsi-orp3-status`),
    staleTime: 10 * 60_000,
  })
}
