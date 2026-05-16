import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { apiFetch } from '../api/client'

export interface GitHubIntegration {
  id: string
  org_id: string
  repo_owner: string
  repo_name: string
  last_synced_at: string | null
  sync_status: string // 'pending' | 'ok' | 'error'
  sync_error?: string
  created_at: string
  updated_at: string
}

export interface GitHubCheckResult {
  id: string
  integration_id: string
  check_type: string
  status: string // 'pass' | 'fail' | 'unknown'
  details?: Record<string, unknown>
  checked_at: string
}

export interface AddGitHubIntegrationInput {
  repo_owner: string
  repo_name: string
  access_token: string
}

const BASE = '/integrations/github'

export function useGitHubIntegrations() {
  return useQuery<GitHubIntegration[]>({
    queryKey: ['integrations', 'github'],
    queryFn: () => apiFetch<GitHubIntegration[]>(BASE),
    staleTime: 60_000,
  })
}

export function useAddGitHubIntegration() {
  const qc = useQueryClient()
  return useMutation<GitHubIntegration, Error, AddGitHubIntegrationInput>({
    mutationFn: (data) =>
      apiFetch<GitHubIntegration>(BASE, {
        method: 'POST',
        body: JSON.stringify(data),
      }),
    onSuccess: () => {
      void qc.invalidateQueries({ queryKey: ['integrations', 'github'] })
    },
  })
}

export function useDeleteGitHubIntegration() {
  const qc = useQueryClient()
  return useMutation<void, Error, string>({
    mutationFn: (id) => apiFetch<void>(`${BASE}/${id}`, { method: 'DELETE' }),
    onSuccess: () => {
      void qc.invalidateQueries({ queryKey: ['integrations', 'github'] })
    },
  })
}

export function useSyncGitHubIntegration() {
  const qc = useQueryClient()
  return useMutation<{ status: string }, Error, string>({
    mutationFn: (id) =>
      apiFetch<{ status: string }>(`${BASE}/${id}/sync`, { method: 'POST' }),
    onSuccess: (_data, id) => {
      void qc.invalidateQueries({ queryKey: ['integrations', 'github'] })
      void qc.invalidateQueries({ queryKey: ['integrations', 'github', 'checks', id] })
    },
  })
}

export function useGitHubCheckResults(integrationId: string) {
  return useQuery<GitHubCheckResult[]>({
    queryKey: ['integrations', 'github', 'checks', integrationId],
    queryFn: () => apiFetch<GitHubCheckResult[]>(`${BASE}/${integrationId}/checks`),
    enabled: !!integrationId,
    staleTime: 30_000,
  })
}
