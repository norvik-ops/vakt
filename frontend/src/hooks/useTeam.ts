import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { apiFetch } from '../api/client'

// ---------------------------------------------------------------------------
// Types
// ---------------------------------------------------------------------------

export interface TeamMember {
  id: string
  email: string
  name: string
  role: 'admin' | 'editor' | 'viewer'
  created_at: string
}

export interface TeamInvitation {
  id: string
  org_id: string
  email: string
  role: 'admin' | 'editor' | 'viewer'
  invited_by: string
  accepted_at?: string | null
  expires_at: string
  created_at: string
}

export interface InviteInput {
  email: string
  role: 'admin' | 'editor' | 'viewer'
}

export interface UpdateRoleInput {
  role: 'admin' | 'editor' | 'viewer'
}

export interface InviteInfo {
  id: string
  email: string
  role: 'admin' | 'editor' | 'viewer'
  invited_by: string
  expires_at: string
}

// ---------------------------------------------------------------------------
// Hooks — members
// ---------------------------------------------------------------------------

export function useTeamMembers() {
  return useQuery<TeamMember[]>({
    queryKey: ['team', 'members'],
    queryFn: () => apiFetch<TeamMember[]>('/admin/users'),
  })
}

export function useUpdateRole() {
  const qc = useQueryClient()
  return useMutation<void, Error, { id: string; role: 'admin' | 'editor' | 'viewer' }>({
    mutationFn: ({ id, role }) =>
      apiFetch<void>(`/admin/users/${id}/role`, {
        method: 'PATCH',
        body: JSON.stringify({ role }),
      }),
    onSuccess: () => {
      void qc.invalidateQueries({ queryKey: ['team', 'members'] })
    },
  })
}

export function useRemoveUser() {
  const qc = useQueryClient()
  return useMutation<void, Error, string>({
    mutationFn: (id) => apiFetch<void>(`/admin/users/${id}`, { method: 'DELETE' }),
    onSuccess: () => {
      void qc.invalidateQueries({ queryKey: ['team', 'members'] })
    },
  })
}

// ---------------------------------------------------------------------------
// Hooks — invitations
// ---------------------------------------------------------------------------

export function useInvitations() {
  return useQuery<TeamInvitation[]>({
    queryKey: ['team', 'invitations'],
    queryFn: () => apiFetch<TeamInvitation[]>('/admin/invitations'),
  })
}

export function useCreateInvitation() {
  const qc = useQueryClient()
  return useMutation<TeamInvitation, Error, InviteInput>({
    mutationFn: (data) =>
      apiFetch<TeamInvitation>('/admin/invitations', {
        method: 'POST',
        body: JSON.stringify(data),
      }),
    onSuccess: () => {
      void qc.invalidateQueries({ queryKey: ['team', 'invitations'] })
    },
  })
}

export function useRevokeInvitation() {
  const qc = useQueryClient()
  return useMutation<void, Error, string>({
    mutationFn: (id) => apiFetch<void>(`/admin/invitations/${id}`, { method: 'DELETE' }),
    onSuccess: () => {
      void qc.invalidateQueries({ queryKey: ['team', 'invitations'] })
    },
  })
}

// ---------------------------------------------------------------------------
// Public hook — invite accept page
// ---------------------------------------------------------------------------

export function useInviteInfo(token: string | null) {
  return useQuery<InviteInfo>({
    queryKey: ['invite', 'info', token],
    queryFn: () => apiFetch<InviteInfo>(`/invite/info?token=${token ?? ''}`),
    enabled: !!token,
    retry: false,
  })
}
