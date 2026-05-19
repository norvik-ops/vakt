import { useQuery } from '@tanstack/react-query'
import { apiFetch } from '../../../api/client'

export interface EvidenceHistoryEntry {
  id: string
  evidence_id: string
  changed_by_id?: string | null
  changed_at: string
  title?: string
  status?: string
  change_note?: string
}

export function useEvidenceHistory(evidenceId: string) {
  return useQuery<EvidenceHistoryEntry[]>({
    queryKey: ['secvitals', 'evidence', evidenceId, 'history'],
    queryFn: () => apiFetch<EvidenceHistoryEntry[]>(`/secvitals/evidence/${evidenceId}/history`),
    enabled: !!evidenceId,
    staleTime: 2 * 60 * 1000,
  })
}
