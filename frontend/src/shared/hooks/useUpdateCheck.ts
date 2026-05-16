import { useQuery } from '@tanstack/react-query'
import { apiFetch } from '../../api/client'

export interface UpdateInfo {
  check_enabled: boolean
  current_version: string
  latest_version?: string
  update_available: boolean
  release_url?: string
}

export function useUpdateCheck() {
  return useQuery<UpdateInfo>({
    queryKey: ['system', 'update'],
    queryFn: () => apiFetch<UpdateInfo>('/system/update'),
    staleTime: 60 * 60 * 1000, // 1 hour
    retry: false,
  })
}
