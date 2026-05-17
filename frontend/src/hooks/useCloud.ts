import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { apiFetch } from '../api/client'

// --- AWS ---

export interface AWSConfig {
  access_key_id: string
  secret_access_key: string // "****" if set
  region: string
  account_id: string
  is_configured: boolean
}

export interface SaveAWSConfigInput {
  access_key_id: string
  secret_access_key: string
  region: string
  account_id: string
}

// --- Azure ---

export interface AzureConfig {
  tenant_id: string
  client_id: string
  client_secret: string // "****" if set
  subscription_id: string
  is_configured: boolean
}

export interface SaveAzureConfigInput {
  tenant_id: string
  client_id: string
  client_secret: string
  subscription_id: string
}

// --- Shared ---

export interface CloudSyncStatus {
  provider: string
  enabled: boolean
  last_sync_at: string | null
  last_sync_status: string | null
  last_sync_error: string | null
  evidence_count: number
}

export interface CloudTestResult {
  ok: boolean
  error?: string
}

export interface CloudSyncResult {
  ok: boolean
  evidence_created: number
  error?: string
}

export interface CloudEvidenceItem {
  id: string
  title: string
  description: string
  source: string
  created_at: string
}

const BASE = '/integrations/cloud'

// --- AWS hooks ---

export function useAWSConfig() {
  return useQuery<AWSConfig>({
    queryKey: ['integrations', 'cloud', 'aws', 'config'],
    queryFn: () => apiFetch<AWSConfig>(`${BASE}/aws/config`),
    staleTime: 60_000,
  })
}

export function useSaveAWSConfig() {
  const qc = useQueryClient()
  return useMutation<{ status: string }, Error, SaveAWSConfigInput>({
    mutationFn: (data) =>
      apiFetch<{ status: string }>(`${BASE}/aws/config`, {
        method: 'PUT',
        body: JSON.stringify(data),
      }),
    onSuccess: () => {
      void qc.invalidateQueries({ queryKey: ['integrations', 'cloud', 'aws'] })
    },
  })
}

export function useTestAWSConnection() {
  return useMutation<CloudTestResult, Error>({
    mutationFn: () =>
      apiFetch<CloudTestResult>(`${BASE}/aws/test`, { method: 'POST' }),
  })
}

export function useSyncAWS() {
  const qc = useQueryClient()
  return useMutation<CloudSyncResult, Error>({
    mutationFn: () =>
      apiFetch<CloudSyncResult>(`${BASE}/aws/sync`, { method: 'POST' }),
    onSuccess: () => {
      void qc.invalidateQueries({ queryKey: ['integrations', 'cloud', 'aws'] })
    },
  })
}

export function useAWSStatus() {
  return useQuery<CloudSyncStatus>({
    queryKey: ['integrations', 'cloud', 'aws', 'status'],
    queryFn: () => apiFetch<CloudSyncStatus>(`${BASE}/aws/status`),
    staleTime: 30_000,
  })
}

export function useAWSEvidence() {
  return useQuery<CloudEvidenceItem[]>({
    queryKey: ['integrations', 'cloud', 'aws', 'evidence'],
    queryFn: () => apiFetch<CloudEvidenceItem[]>(`${BASE}/aws/evidence`),
    staleTime: 30_000,
  })
}

// --- Azure hooks ---

export function useAzureConfig() {
  return useQuery<AzureConfig>({
    queryKey: ['integrations', 'cloud', 'azure', 'config'],
    queryFn: () => apiFetch<AzureConfig>(`${BASE}/azure/config`),
    staleTime: 60_000,
  })
}

export function useSaveAzureConfig() {
  const qc = useQueryClient()
  return useMutation<{ status: string }, Error, SaveAzureConfigInput>({
    mutationFn: (data) =>
      apiFetch<{ status: string }>(`${BASE}/azure/config`, {
        method: 'PUT',
        body: JSON.stringify(data),
      }),
    onSuccess: () => {
      void qc.invalidateQueries({ queryKey: ['integrations', 'cloud', 'azure'] })
    },
  })
}

export function useTestAzureConnection() {
  return useMutation<CloudTestResult, Error>({
    mutationFn: () =>
      apiFetch<CloudTestResult>(`${BASE}/azure/test`, { method: 'POST' }),
  })
}

export function useSyncAzure() {
  const qc = useQueryClient()
  return useMutation<CloudSyncResult, Error>({
    mutationFn: () =>
      apiFetch<CloudSyncResult>(`${BASE}/azure/sync`, { method: 'POST' }),
    onSuccess: () => {
      void qc.invalidateQueries({ queryKey: ['integrations', 'cloud', 'azure'] })
    },
  })
}

export function useAzureStatus() {
  return useQuery<CloudSyncStatus>({
    queryKey: ['integrations', 'cloud', 'azure', 'status'],
    queryFn: () => apiFetch<CloudSyncStatus>(`${BASE}/azure/status`),
    staleTime: 30_000,
  })
}

export function useAzureEvidence() {
  return useQuery<CloudEvidenceItem[]>({
    queryKey: ['integrations', 'cloud', 'azure', 'evidence'],
    queryFn: () => apiFetch<CloudEvidenceItem[]>(`${BASE}/azure/evidence`),
    staleTime: 30_000,
  })
}
