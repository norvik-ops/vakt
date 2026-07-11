import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { apiFetch } from '../../../api/client'
import type { Template } from '../types'

const BASE = '/vaktaware'

export function useTemplates() {
  return useQuery<Template[]>({
    queryKey: ['vaktaware', 'templates'],
    queryFn: () => apiFetch<Template[]>(`${BASE}/templates`),
    staleTime: 60_000,
  })
}

// S121-F2 (C8): removed dead hook useTemplate — 0 UI references and it GET
// /vaktaware/templates/:id, a route the backend never registered.

export interface CreateTemplateInput {
  name: string
  subject: string
  from_name: string
  from_email: string
  html_body: string
}

export function useCreateTemplate() {
  const queryClient = useQueryClient()
  return useMutation<Template, Error, CreateTemplateInput>({
    mutationFn: (data) =>
      apiFetch<Template>(`${BASE}/templates`, {
        method: 'POST',
        body: JSON.stringify(data),
      }),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: ['vaktaware', 'templates'] })
    },
  })
}

export function useDeleteTemplate() {
  const queryClient = useQueryClient()
  return useMutation<undefined, Error, string>({
    mutationFn: (id) =>
      apiFetch<undefined>(`${BASE}/templates/${id}`, { method: 'DELETE' }),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: ['vaktaware', 'templates'] })
    },
  })
}
