import { useQuery } from '@tanstack/react-query'
import { apiFetch } from '../../../api/client'
import type { KPIDashboard } from '../types'

export function useKPIDashboard() {
  return useQuery<KPIDashboard>({
    queryKey: ['vaktcomply', 'kpi-dashboard'],
    queryFn: () => apiFetch<KPIDashboard>('/vaktcomply/kpi-dashboard'),
    staleTime: 5 * 60 * 1000, // 5 minutes
  })
}
