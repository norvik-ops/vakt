import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { apiFetch } from '../api/client'

// ---------------------------------------------------------------------------
// Types
// ---------------------------------------------------------------------------

export interface AuditorInvite {
  id: string
  org_id: string
  email: string
  expires_at: string
  accepted_at?: string
  created_at: string
}

export interface CreateInviteInput {
  email: string
  expires_in: number // days, 1–90
}

export interface CreateInviteResponse {
  invite: AuditorInvite
  token: string
  invite_url: string
}

// ---------------------------------------------------------------------------
// Hooks
// ---------------------------------------------------------------------------

export function useAuditorInvites() {
  return useQuery<AuditorInvite[]>({
    queryKey: ['auditor', 'invites'],
    queryFn: () => apiFetch<AuditorInvite[]>('/auditor/invites'),
  })
}

export function useCreateAuditorInvite() {
  const qc = useQueryClient()
  return useMutation<CreateInviteResponse, Error, CreateInviteInput>({
    mutationFn: (data) =>
      apiFetch<CreateInviteResponse>('/auditor/invites', {
        method: 'POST',
        body: JSON.stringify(data),
      }),
    onSuccess: () => {
      void qc.invalidateQueries({ queryKey: ['auditor', 'invites'] })
    },
  })
}

export function useRevokeAuditorInvite() {
  const qc = useQueryClient()
  return useMutation<void, Error, string>({
    mutationFn: (id) =>
      apiFetch<void>(`/auditor/invites/${id}`, { method: 'DELETE' }),
    onSuccess: () => {
      void qc.invalidateQueries({ queryKey: ['auditor', 'invites'] })
    },
  })
}
