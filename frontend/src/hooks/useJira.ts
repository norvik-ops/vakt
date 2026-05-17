import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { apiFetch } from '../api/client'

export interface JiraConfig {
  jira_url: string
  project_key: string
  user_email: string
  api_token: string       // always "****" from server after save
  is_configured: boolean
}

export interface SaveJiraConfigInput {
  jira_url: string
  project_key: string
  user_email: string
  api_token: string
}

export interface JiraTestResult {
  success: boolean
  display_name?: string
  error?: string
}

export interface JiraCreateIssueResult {
  issue_key: string
  issue_url: string
}

export interface JiraIssue {
  id: string
  org_id: string
  finding_id: string
  issue_key: string
  issue_url: string
  created_at: string
}

const BASE = '/integrations/jira'

export function useJiraConfig() {
  return useQuery<JiraConfig>({
    queryKey: ['integrations', 'jira', 'config'],
    queryFn: () => apiFetch<JiraConfig>(`${BASE}/config`),
    staleTime: 60_000,
  })
}

export function useSaveJiraConfig() {
  const qc = useQueryClient()
  return useMutation<{ status: string }, Error, SaveJiraConfigInput>({
    mutationFn: (data) =>
      apiFetch<{ status: string }>(`${BASE}/config`, {
        method: 'PUT',
        body: JSON.stringify(data),
      }),
    onSuccess: () => {
      void qc.invalidateQueries({ queryKey: ['integrations', 'jira', 'config'] })
    },
  })
}

export function useTestJiraConnection() {
  return useMutation<JiraTestResult, Error>({
    mutationFn: () =>
      apiFetch<JiraTestResult>(`${BASE}/test`, { method: 'POST' }),
  })
}

export function useCreateJiraIssue() {
  const qc = useQueryClient()
  return useMutation<JiraCreateIssueResult, Error, string>({
    mutationFn: (findingId) =>
      apiFetch<JiraCreateIssueResult>(`${BASE}/findings/${findingId}/create-issue`, {
        method: 'POST',
      }),
    onSuccess: () => {
      void qc.invalidateQueries({ queryKey: ['secpulse', 'findings'] })
      void qc.invalidateQueries({ queryKey: ['integrations', 'jira', 'issues'] })
    },
  })
}
