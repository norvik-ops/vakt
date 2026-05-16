import { useQuery } from '@tanstack/react-query'
import { apiFetch } from '../../../api/client'
import type { MappingResult } from '../types'

/**
 * Fetches TISAX ↔ ISO 27001 mapping with coverage status.
 * Query param framework_id is optional; the backend resolves the TISAX framework by name if omitted.
 */
export function useTISAXISOMapping(frameworkId?: string) {
  const params = frameworkId ? `?framework_id=${frameworkId}` : ''
  return useQuery<MappingResult[]>({
    queryKey: ['secvitals', 'tisax-iso-mapping', frameworkId ?? 'default'],
    queryFn: () => apiFetch<MappingResult[]>(`/secvitals/frameworks/tisax/iso-mapping${params}`),
    staleTime: 5 * 60 * 1000,
  })
}

/**
 * Fetches TISAX controls that are NOT covered by a mapped ISO 27001 control.
 */
export function useTISAXGapsAfterISO(frameworkId?: string) {
  const params = frameworkId ? `?framework_id=${frameworkId}` : ''
  return useQuery<MappingResult[]>({
    queryKey: ['secvitals', 'tisax-coverage-after-iso', frameworkId ?? 'default'],
    queryFn: () => apiFetch<MappingResult[]>(`/secvitals/frameworks/tisax/coverage-after-iso${params}`),
    staleTime: 5 * 60 * 1000,
  })
}

