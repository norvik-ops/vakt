import { useQuery } from '@tanstack/react-query'
import { apiFetch } from '../../../api/client'
import type { MappingCoverageResponse, ImplementationStep } from '../types'

export function useMappingCoverage() {
  return useQuery<MappingCoverageResponse>({
    queryKey: ['vaktcomply', 'mapping-coverage'],
    queryFn: () => apiFetch<MappingCoverageResponse>('/vaktcomply/frameworks/mapping-coverage'),
    staleTime: 5 * 60 * 1000,
  })
}

export function useImplementationPath(frameworkId: string) {
  return useQuery<ImplementationStep[]>({
    queryKey: ['vaktcomply', 'implementation-path', frameworkId],
    queryFn: () => apiFetch<ImplementationStep[]>(`/vaktcomply/frameworks/${frameworkId}/implementation-path`),
    staleTime: 2 * 60 * 1000,
    enabled: !!frameworkId,
  })
}
