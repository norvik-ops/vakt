import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { apiFetch } from '../../../api/client'
import type { PaginatedResponse } from '../../../shared/types/pagination'

export interface CAPA {
  id: string
  org_id: string
  source_type: 'audit' | 'incident' | 'risk' | 'manual'
  source_id: string
  title: string
  description: string
  root_cause: string
  action_plan: string
  assignee_email: string
  due_date: string | null
  priority: 'low' | 'medium' | 'high' | 'critical'
  status: 'open' | 'in_progress' | 'implemented' | 'verified' | 'closed'
  verification_note: string
  closed_at: string | null
  created_at: string
  updated_at: string
}

export interface CreateCAPAInput {
  source_type: 'audit' | 'incident' | 'risk' | 'manual'
  source_id?: string
  title: string
  description?: string
  assignee_email?: string
  due_date?: string | null
  priority?: 'low' | 'medium' | 'high' | 'critical'
}

export interface UpdateCAPAInput {
  title?: string
  description?: string
  root_cause?: string
  action_plan?: string
  assignee_email?: string
  due_date?: string | null
  priority?: 'low' | 'medium' | 'high' | 'critical'
  status?: 'open' | 'in_progress' | 'implemented' | 'verified' | 'closed'
  verification_note?: string
}

export function useCAPAs(statusFilter?: string, page = 1, limit = 25) {
  const params = new URLSearchParams()
  if (statusFilter) params.set('status', statusFilter)
  params.set('page', String(page))
  params.set('limit', String(limit))
  const query = useQuery<PaginatedResponse<CAPA>>({
    queryKey: ['secvitals', 'capas', statusFilter ?? 'all', page, limit],
    queryFn: () => apiFetch<PaginatedResponse<CAPA>>(`/secvitals/capas?${params.toString()}`),
    staleTime: 2 * 60 * 1000,
  })
  return {
    ...query,
    data: query.data?.data,
    pagination: query.data?.pagination,
  }
}

export function useCAPAsForSource(sourceType: string, sourceId: string) {
  const path = sourceType === 'audit'
    ? `/secvitals/audits/${sourceId}/capas`
    : `/secvitals/incidents/${sourceId}/capas`
  return useQuery<CAPA[]>({
    queryKey: ['secvitals', 'capas', sourceType, sourceId],
    queryFn: () => apiFetch<CAPA[]>(path),
    enabled: !!sourceId,
    staleTime: 2 * 60 * 1000,
  })
}

export function useCreateCAPA() {
  const queryClient = useQueryClient()
  return useMutation<CAPA, Error, CreateCAPAInput>({
    mutationFn: (input) =>
      apiFetch<CAPA>('/secvitals/capas', { method: 'POST', body: JSON.stringify(input) }),
    onSuccess: (data) => {
      void queryClient.invalidateQueries({ queryKey: ['secvitals', 'capas'] })
      if (data.source_id) {
        void queryClient.invalidateQueries({ queryKey: ['secvitals', 'capas', data.source_type, data.source_id] })
      }
    },
  })
}

export function useUpdateCAPA() {
  const queryClient = useQueryClient()
  return useMutation<CAPA, Error, { id: string; input: UpdateCAPAInput }>({
    mutationFn: ({ id, input }) =>
      apiFetch<CAPA>(`/secvitals/capas/${id}`, { method: 'PATCH', body: JSON.stringify(input) }),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: ['secvitals', 'capas'] })
    },
  })
}

export function useDeleteCAPA() {
  const queryClient = useQueryClient()
  return useMutation<void, Error, string>({
    mutationFn: (id) =>
      apiFetch<void>(`/secvitals/capas/${id}`, { method: 'DELETE' }),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: ['secvitals', 'capas'] })
    },
  })
}
