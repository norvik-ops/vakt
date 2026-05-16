import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { apiFetch } from '../../../api/client'

export interface AutoEvidence {
  id: string
  org_id: string
  title: string
  description?: string
  auto_source_type: 'github' | 'secreflex' | 'secpulse'
  auto_source_ref?: string
  auto_collected_at?: string
  created_at: string
}

// useAutoEvidence — lists all unassigned auto-collected evidence
export function useAutoEvidence() {
  return useQuery<AutoEvidence[]>({
    queryKey: ['secvitals', 'evidence', 'auto'],
    queryFn: () => apiFetch<AutoEvidence[]>('/secvitals/evidence/auto'),
    staleTime: 30 * 1000,
  })
}

interface AssignPayload {
  control_id: string
}

// useAssignEvidence — mutation to assign auto-evidence to a control
export function useAssignEvidence() {
  const queryClient = useQueryClient()
  return useMutation<void, Error, { evidenceId: string; controlId: string }>({
    mutationFn: ({ evidenceId, controlId }) =>
      apiFetch<void>(`/secvitals/evidence/auto/${evidenceId}/assign`, {
        method: 'POST',
        body: JSON.stringify({ control_id: controlId } satisfies AssignPayload),
      }),
    onSuccess: () => {
      void queryClient.invalidateQueries({
        queryKey: ['secvitals', 'evidence', 'auto'],
      })
    },
  })
}
