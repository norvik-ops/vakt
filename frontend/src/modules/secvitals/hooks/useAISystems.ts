import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { apiFetch } from '../../../api/client'
import type { AISystem, AIClassification, AIDocumentation, ClassifyAISystemInput, CreateAISystemInput, UpdateAISystemInput, UpsertAIDocumentationInput } from '../types'

export interface AISystemFilters {
  riskClass?: string
  status?: string
}

export function useAISystems(filters?: AISystemFilters) {
  const params = new URLSearchParams()
  if (filters?.riskClass) params.set('risk_class', filters.riskClass)
  if (filters?.status) params.set('status', filters.status)
  const qs = params.toString() ? `?${params.toString()}` : ''
  return useQuery<AISystem[]>({
    queryKey: ['secvitals', 'ai-systems', filters?.riskClass ?? '', filters?.status ?? ''],
    queryFn: () => apiFetch<AISystem[]>(`/secvitals/ai-systems${qs}`),
    staleTime: 5 * 60 * 1000,
  })
}

export function useDeleteAISystem() {
  const queryClient = useQueryClient()
  return useMutation<void, Error, string>({
    mutationFn: (id) =>
      apiFetch<void>(`/secvitals/ai-systems/${id}`, { method: 'DELETE' }),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: ['secvitals', 'ai-systems'] })
    },
  })
}

export function useAISystem(id: string) {
  return useQuery<AISystem>({
    queryKey: ['secvitals', 'ai-systems', id],
    queryFn: () => apiFetch<AISystem>(`/secvitals/ai-systems/${id}`),
    enabled: !!id,
    staleTime: 5 * 60 * 1000,
  })
}

export function useCreateAISystem() {
  const queryClient = useQueryClient()
  return useMutation<AISystem, Error, CreateAISystemInput>({
    mutationFn: (input) =>
      apiFetch<AISystem>('/secvitals/ai-systems', { method: 'POST', body: JSON.stringify(input) }),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: ['secvitals', 'ai-systems'] })
    },
  })
}

export function useUpdateAISystem(id: string) {
  const queryClient = useQueryClient()
  return useMutation<AISystem, Error, UpdateAISystemInput>({
    mutationFn: (input) =>
      apiFetch<AISystem>(`/secvitals/ai-systems/${id}`, { method: 'PATCH', body: JSON.stringify(input) }),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: ['secvitals', 'ai-systems'] })
      void queryClient.invalidateQueries({ queryKey: ['secvitals', 'ai-systems', id] })
    },
  })
}

export function useAIClassifications(systemId: string) {
  return useQuery<AIClassification[]>({
    queryKey: ['secvitals', 'ai-systems', systemId, 'classifications'],
    queryFn: () => apiFetch<AIClassification[]>(`/secvitals/ai-systems/${systemId}/classifications`),
    enabled: !!systemId,
    staleTime: 5 * 60 * 1000,
  })
}

export function useClassifyAISystem(systemId: string) {
  const queryClient = useQueryClient()
  return useMutation<void, Error, ClassifyAISystemInput>({
    mutationFn: (input) =>
      apiFetch<void>(`/secvitals/ai-systems/${systemId}/classify`, { method: 'POST', body: JSON.stringify(input) }),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: ['secvitals', 'ai-systems'] })
      void queryClient.invalidateQueries({ queryKey: ['secvitals', 'ai-systems', systemId, 'classifications'] })
    },
  })
}

export function useAIDocumentation(systemId: string) {
  return useQuery<AIDocumentation>({
    queryKey: ['secvitals', 'ai-systems', systemId, 'documentation'],
    queryFn: () => apiFetch<AIDocumentation>(`/secvitals/ai-systems/${systemId}/documentation`),
    enabled: !!systemId,
    retry: false,
    staleTime: 5 * 60 * 1000,
  })
}

export function useAIDocumentationVersions(systemId: string) {
  return useQuery<AIDocumentation[]>({
    queryKey: ['secvitals', 'ai-systems', systemId, 'documentation', 'versions'],
    queryFn: () => apiFetch<AIDocumentation[]>(`/secvitals/ai-systems/${systemId}/documentation/versions`),
    enabled: !!systemId,
    staleTime: 5 * 60 * 1000,
  })
}

export function useSaveAIDocumentation(systemId: string) {
  const queryClient = useQueryClient()
  return useMutation<AIDocumentation, Error, UpsertAIDocumentationInput>({
    mutationFn: (input) =>
      apiFetch<AIDocumentation>(`/secvitals/ai-systems/${systemId}/documentation`, { method: 'POST', body: JSON.stringify(input) }),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: ['secvitals', 'ai-systems', systemId, 'documentation'] })
    },
  })
}
