import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { apiFetch } from '../api/client'

// ---------------------------------------------------------------------------
// Types
// ---------------------------------------------------------------------------

// PlatformRole is roles.name — the role org_members carries, the role the token
// claim carries, and the role every backend RequireRole guard checks (ADR-0077).
// This is the one to display and the one to send when changing a role.
//
// The legacy `role` field below is the denormalised users.role cache. It only
// knows admin/editor/viewer, so AuditorReadOnly and InternalAuditor both appear
// there as 'viewer' — rendering it would show the org's internal auditor as a
// plain viewer.
export type PlatformRole =
  | 'Admin'
  | 'SecurityAnalyst'
  | 'Viewer'
  | 'AuditorReadOnly'
  | 'InternalAuditor'

export interface TeamMember {
  id: string
  email: string
  name: string
  role: 'admin' | 'editor' | 'viewer'
  platform_role: PlatformRole
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

// POST /admin/users is served by internal/admin, whose validator does NOT accept
// InternalAuditor — a user is created with one of these four and then given the
// auditor role through useUpdateRole().
export interface CreateUserInput {
  email: string
  password: string
  role: 'Admin' | 'SecurityAnalyst' | 'Viewer' | 'AuditorReadOnly'
}

export interface CreateUserResult {
  user_id: string
  email: string
  role: string
}

export interface UpdateRoleInput {
  role: PlatformRole
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
  return useMutation<undefined, Error, { id: string; role: PlatformRole }>({
    mutationFn: ({ id, role }) =>
      apiFetch<undefined>(`/admin/users/${id}/role`, {
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
  return useMutation<undefined, Error, string>({
    mutationFn: (id) => apiFetch<undefined>(`/admin/users/${id}`, { method: 'DELETE' }),
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

export function useCreateUser() {
  const qc = useQueryClient()
  return useMutation<CreateUserResult, Error, CreateUserInput>({
    mutationFn: (data) =>
      apiFetch<CreateUserResult>('/admin/users', {
        method: 'POST',
        body: JSON.stringify(data),
      }),
    onSuccess: () => {
      void qc.invalidateQueries({ queryKey: ['team', 'members'] })
    },
  })
}

export function useRevokeInvitation() {
  const qc = useQueryClient()
  return useMutation<undefined, Error, string>({
    mutationFn: (id) => apiFetch<undefined>(`/admin/invitations/${id}`, { method: 'DELETE' }),
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
