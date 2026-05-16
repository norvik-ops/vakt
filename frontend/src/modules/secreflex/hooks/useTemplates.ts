import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { apiFetch } from '../../../api/client'
import type { Template } from '../types'

const BASE = '/secreflex'

export function useTemplates() {
  return useQuery<Template[]>({
    queryKey: ['secreflex', 'templates'],
    queryFn: () => apiFetch<Template[]>(`${BASE}/templates`),
    staleTime: 60_000,
  })
}

export function useTemplate(id: string) {
  return useQuery<Template>({
    queryKey: ['secreflex', 'templates', id],
    queryFn: () => apiFetch<Template>(`${BASE}/templates/${id}`),
    staleTime: 60_000,
    enabled: Boolean(id),
  })
}

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
      void queryClient.invalidateQueries({ queryKey: ['secreflex', 'templates'] })
    },
  })
}

export function useDeleteTemplate() {
  const queryClient = useQueryClient()
  return useMutation<void, Error, string>({
    mutationFn: (id) =>
      apiFetch<void>(`${BASE}/templates/${id}`, { method: 'DELETE' }),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: ['secreflex', 'templates'] })
    },
  })
}
