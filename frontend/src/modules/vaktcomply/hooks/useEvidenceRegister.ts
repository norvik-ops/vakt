import { useQuery } from '@tanstack/react-query'
import { apiFetch } from '../../../api/client'
import type { components } from '../../../api/generated'

/** One row of GET /vaktcomply/evidence — typed from openapi.yaml, not by hand. */
export type EvidenceListItem = components['schemas']['EvidenceListItem']

export const EVIDENCE_REGISTER_KEY = ['vaktcomply', 'evidence', 'register'] as const

/** All evidence of the organisation across controls and frameworks (page „Nachweise"). */
export function useEvidenceRegister() {
  return useQuery<EvidenceListItem[]>({
    queryKey: EVIDENCE_REGISTER_KEY,
    queryFn: () => apiFetch<EvidenceListItem[]>('/vaktcomply/evidence'),
    staleTime: 30 * 1000,
  })
}
