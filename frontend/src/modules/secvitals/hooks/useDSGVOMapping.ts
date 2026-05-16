import { useQuery } from '@tanstack/react-query'
import { apiFetch } from '../../../api/client'
import type { MappingResult } from '../types'

export function useDSGVOTOMCoverage(frameworkId?: string) {
  const params = frameworkId ? `?framework_id=${frameworkId}` : ''
  return useQuery<MappingResult[]>({
    queryKey: ['secvitals', 'dsgvo-tom-coverage', frameworkId ?? 'default'],
    queryFn: () => apiFetch<MappingResult[]>(`/secvitals/dsgvo/tom-coverage${params}`),
    staleTime: 5 * 60 * 1000,
  })
}
