import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { apiFetch } from '../../../api/client'

export interface AutoEvidence {
  id: string
  org_id: string
  title: string
  description?: string
  auto_source_type: 'github' | 'vaktaware' | 'vaktscan' | 'ci_pipeline' | 'ci_webhook' | 'hr'
  auto_source_ref?: string
  auto_collected_at?: string
  created_at: string
  suggested_control_hint?: string
}

// useAutoEvidence — lists all unassigned auto-collected evidence
export function useAutoEvidence() {
  return useQuery<AutoEvidence[]>({
    queryKey: ['vaktcomply', 'evidence', 'auto'],
    queryFn: () => apiFetch<AutoEvidence[]>('/vaktcomply/evidence/auto'),
    staleTime: 30 * 1000,
  })
}

interface AssignPayload {
  control_id: string
}

// useAssignEvidence — mutation to assign auto-evidence to a control
export function useAssignEvidence() {
  const queryClient = useQueryClient()
  return useMutation<undefined, Error, { evidenceId: string; controlId: string }>({
    mutationFn: ({ evidenceId, controlId }) =>
      apiFetch<undefined>(`/vaktcomply/evidence/auto/${evidenceId}/assign`, {
        method: 'POST',
        body: JSON.stringify({ control_id: controlId } satisfies AssignPayload),
      }),
    onSuccess: () => {
      void queryClient.invalidateQueries({
        queryKey: ['vaktcomply', 'evidence', 'auto'],
      })
    },
  })
}
