import { useQuery } from '@tanstack/react-query'
import { apiFetch } from '../api/client'

export interface AuditLogEntry {
  id: string
  org_id: string
  user_id?: string
  user_email?: string
  action: string
  resource_type: string
  resource_id?: string
  resource_name?: string
  details?: Record<string, string>
  ip_address?: string
  created_at: string
}

export interface AuditLogFilters {
  limit?: number
  from?: string  // ISO date string
  to?: string    // ISO date string
  user_email?: string
  action?: string
}

export function useAuditLog(filters: AuditLogFilters = {}) {
  const params = new URLSearchParams()
  if (filters.limit) params.set('limit', String(filters.limit))

  return useQuery<AuditLogEntry[]>({
    queryKey: ['audit-log', filters],
    queryFn: () => {
      const qs = params.toString()
      return apiFetch<AuditLogEntry[]>(`/audit-log${qs ? `?${qs}` : ''}`)
    },
    staleTime: 30_000,
  })
}
