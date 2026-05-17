import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { apiFetch } from '../api/client'

export interface ModulePermission {
  module: 'secpulse' | 'secvitals' | 'secvault' | 'secreflex' | 'secprivacy'
  can_read: boolean
  can_write: boolean
}

export function useUserPermissions(userId: string) {
  return useQuery<ModulePermission[]>({
    queryKey: ['users', userId, 'permissions'],
    queryFn: () => apiFetch<ModulePermission[]>(`/admin/users/${userId}/permissions`),
    enabled: !!userId,
  })
}

export function useUpdateUserPermissions(userId: string) {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (permissions: ModulePermission[]) =>
      apiFetch(`/admin/users/${userId}/permissions`, { method: 'PUT', body: JSON.stringify({ permissions }) }),
    onSuccess: () => void qc.invalidateQueries({ queryKey: ['users', userId, 'permissions'] }),
  })
}
