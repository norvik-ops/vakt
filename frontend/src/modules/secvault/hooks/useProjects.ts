import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { apiFetch } from '../../../api/client'
import type { Project, ProjectHealth } from '../types'

const BASE = '/secvault'

export function useProjects() {
  return useQuery<Project[]>({
    queryKey: ['secvault', 'projects'],
    queryFn: () => apiFetch<Project[]>(`${BASE}/projects`),
    staleTime: 30_000,
  })
}

export function useProject(id: string) {
  return useQuery<Project>({
    queryKey: ['secvault', 'projects', id],
    queryFn: () => apiFetch<Project>(`${BASE}/projects/${id}`),
    staleTime: 30_000,
    enabled: Boolean(id),
  })
}

export function useProjectHealth(projectId: string) {
  return useQuery<ProjectHealth>({
    queryKey: ['secvault', 'projects', projectId, 'health'],
    queryFn: () => apiFetch<ProjectHealth>(`${BASE}/projects/${projectId}/health`),
    staleTime: 60_000,
    enabled: Boolean(projectId),
  })
}

export interface CreateProjectInput {
  name: string
  description?: string
}

export function useCreateProject() {
  const queryClient = useQueryClient()
  return useMutation<Project, Error, CreateProjectInput>({
    mutationFn: (data) =>
      apiFetch<Project>(`${BASE}/projects`, {
        method: 'POST',
        body: JSON.stringify(data),
      }),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: ['secvault', 'projects'] })
    },
  })
}

export function useDeleteProject() {
  const queryClient = useQueryClient()
  return useMutation<void, Error, string>({
    mutationFn: (id) =>
      apiFetch<void>(`${BASE}/projects/${id}`, { method: 'DELETE' }),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: ['secvault', 'projects'] })
    },
  })
}
