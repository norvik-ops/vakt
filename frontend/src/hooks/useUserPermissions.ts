import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { apiFetch } from '../api/client'

export interface ModulePermission {
  module: 'vaktscan' | 'vaktcomply' | 'vaktvault' | 'vaktaware' | 'vaktprivacy'
  can_read: boolean
  can_write: boolean
}

export function useUserPermissions(userId: string) {
  return useQuery<ModulePermission[]>({
    queryKey: ['users', userId, 'permissions'],
    queryFn: () => apiFetch<{ data: ModulePermission[] }>(`/admin/users/${userId}/permissions`).then((r) => r.data),
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
