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
  return useMutation<CloudTestResult>({
    mutationFn: () =>
      apiFetch<CloudTestResult>(`${BASE}/aws/test`, { method: 'POST' }),
  })
}

export function useSyncAWS() {
  const qc = useQueryClient()
  return useMutation<CloudSyncResult>({
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
  return useMutation<CloudTestResult>({
    mutationFn: () =>
      apiFetch<CloudTestResult>(`${BASE}/azure/test`, { method: 'POST' }),
  })
}

export function useSyncAzure() {
  const qc = useQueryClient()
  return useMutation<CloudSyncResult>({
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

// --- Hetzner ---

export interface HetznerConfig {
  api_token: string // "****" if set
  location: string
  is_configured: boolean
}

export interface SaveHetznerConfigInput {
  api_token: string
  location: string
}

export interface HetznerStatus extends CloudSyncStatus {
  server_count: number
}

export function useHetznerConfig() {
  return useQuery<HetznerConfig>({
    queryKey: ['integrations', 'cloud', 'hetzner', 'config'],
    queryFn: () => apiFetch<HetznerConfig>(`${BASE}/hetzner/config`),
    staleTime: 60_000,
  })
}

export function useSaveHetznerConfig() {
  const qc = useQueryClient()
  return useMutation<{ status: string }, Error, SaveHetznerConfigInput>({
    mutationFn: (data) =>
      apiFetch<{ status: string }>(`${BASE}/hetzner/config`, {
        method: 'PUT',
        body: JSON.stringify(data),
      }),
    onSuccess: () => {
      void qc.invalidateQueries({ queryKey: ['integrations', 'cloud', 'hetzner'] })
    },
  })
}

export function useSyncHetzner() {
  const qc = useQueryClient()
  return useMutation<CloudSyncResult>({
    mutationFn: () => apiFetch<CloudSyncResult>(`${BASE}/hetzner/sync`, { method: 'POST' }),
    onSuccess: () => {
      void qc.invalidateQueries({ queryKey: ['integrations', 'cloud', 'hetzner'] })
    },
  })
}

export function useHetznerStatus() {
  return useQuery<HetznerStatus>({
    queryKey: ['integrations', 'cloud', 'hetzner', 'status'],
    queryFn: () => apiFetch<HetznerStatus>(`${BASE}/hetzner/status`),
    staleTime: 30_000,
  })
}

export function useHetznerEvidence() {
  return useQuery<CloudEvidenceItem[]>({
    queryKey: ['integrations', 'cloud', 'hetzner', 'evidence'],
    queryFn: () => apiFetch<CloudEvidenceItem[]>(`${BASE}/hetzner/evidence`),
    staleTime: 30_000,
  })
}

// --- IONOS ---

export interface IONOSConfig {
  username: string
  password: string // "****" if set
  token: string    // "****" if set
  is_configured: boolean
}

export interface SaveIONOSConfigInput {
  username: string
  password: string
  token: string
}

export interface IONOSStatus extends CloudSyncStatus {
  server_count: number
  datacenter_count: number
}

export function useIONOSConfig() {
  return useQuery<IONOSConfig>({
    queryKey: ['integrations', 'cloud', 'ionos', 'config'],
    queryFn: () => apiFetch<IONOSConfig>(`${BASE}/ionos/config`),
    staleTime: 60_000,
  })
}

export function useSaveIONOSConfig() {
  const qc = useQueryClient()
  return useMutation<{ status: string }, Error, SaveIONOSConfigInput>({
    mutationFn: (data) =>
      apiFetch<{ status: string }>(`${BASE}/ionos/config`, {
        method: 'PUT',
        body: JSON.stringify(data),
      }),
    onSuccess: () => {
      void qc.invalidateQueries({ queryKey: ['integrations', 'cloud', 'ionos'] })
    },
  })
}

export function useSyncIONOS() {
  const qc = useQueryClient()
  return useMutation<CloudSyncResult>({
    mutationFn: () => apiFetch<CloudSyncResult>(`${BASE}/ionos/sync`, { method: 'POST' }),
    onSuccess: () => {
      void qc.invalidateQueries({ queryKey: ['integrations', 'cloud', 'ionos'] })
    },
  })
}

export function useIONOSStatus() {
  return useQuery<IONOSStatus>({
    queryKey: ['integrations', 'cloud', 'ionos', 'status'],
    queryFn: () => apiFetch<IONOSStatus>(`${BASE}/ionos/status`),
    staleTime: 30_000,
  })
}

export function useIONOSEvidence() {
  return useQuery<CloudEvidenceItem[]>({
    queryKey: ['integrations', 'cloud', 'ionos', 'evidence'],
    queryFn: () => apiFetch<CloudEvidenceItem[]>(`${BASE}/ionos/evidence`),
    staleTime: 30_000,
  })
}

// --- Wazuh ---

export interface WazuhConfig {
  base_url: string
  username: string
  password: string // "****" if set
  verify_tls: boolean
  is_configured: boolean
}

export interface SaveWazuhConfigInput {
  base_url: string
  username: string
  password: string
  verify_tls: boolean
}

export interface WazuhStatus extends CloudSyncStatus {
  agent_count: number
  agents_offline: number
}

export function useWazuhConfig() {
  return useQuery<WazuhConfig>({
    queryKey: ['integrations', 'cloud', 'wazuh', 'config'],
    queryFn: () => apiFetch<WazuhConfig>(`${BASE}/wazuh/config`),
    staleTime: 60_000,
  })
}

export function useSaveWazuhConfig() {
  const qc = useQueryClient()
  return useMutation<{ status: string }, Error, SaveWazuhConfigInput>({
    mutationFn: (data) =>
      apiFetch<{ status: string }>(`${BASE}/wazuh/config`, {
        method: 'PUT',
        body: JSON.stringify(data),
      }),
    onSuccess: () => {
      void qc.invalidateQueries({ queryKey: ['integrations', 'cloud', 'wazuh'] })
    },
  })
}

export function useSyncWazuh() {
  const qc = useQueryClient()
  return useMutation<CloudSyncResult>({
    mutationFn: () => apiFetch<CloudSyncResult>(`${BASE}/wazuh/sync`, { method: 'POST' }),
    onSuccess: () => {
      void qc.invalidateQueries({ queryKey: ['integrations', 'cloud', 'wazuh'] })
    },
  })
}

export function useWazuhStatus() {
  return useQuery<WazuhStatus>({
    queryKey: ['integrations', 'cloud', 'wazuh', 'status'],
    queryFn: () => apiFetch<WazuhStatus>(`${BASE}/wazuh/status`),
    staleTime: 30_000,
  })
}

export function useWazuhEvidence() {
  return useQuery<CloudEvidenceItem[]>({
    queryKey: ['integrations', 'cloud', 'wazuh', 'evidence'],
    queryFn: () => apiFetch<CloudEvidenceItem[]>(`${BASE}/wazuh/evidence`),
    staleTime: 30_000,
  })
}

// --- Prometheus ---

export interface PrometheusConfig {
  prometheus_url: string
  alertmanager_url: string
  token: string // "****" if set
  is_configured: boolean
}

export interface SavePrometheusConfigInput {
  prometheus_url: string
  alertmanager_url: string
  token: string
}

export interface PrometheusStatus extends CloudSyncStatus {
  target_count: number
  active_alert_count: number
}

export function usePrometheusConfig() {
  return useQuery<PrometheusConfig>({
    queryKey: ['integrations', 'cloud', 'prometheus', 'config'],
    queryFn: () => apiFetch<PrometheusConfig>(`${BASE}/prometheus/config`),
    staleTime: 60_000,
  })
}

export function useSavePrometheusConfig() {
  const qc = useQueryClient()
  return useMutation<{ status: string }, Error, SavePrometheusConfigInput>({
    mutationFn: (data) =>
      apiFetch<{ status: string }>(`${BASE}/prometheus/config`, {
        method: 'PUT',
        body: JSON.stringify(data),
      }),
    onSuccess: () => {
      void qc.invalidateQueries({ queryKey: ['integrations', 'cloud', 'prometheus'] })
    },
  })
}

export function useSyncPrometheus() {
  const qc = useQueryClient()
  return useMutation<CloudSyncResult>({
    mutationFn: () => apiFetch<CloudSyncResult>(`${BASE}/prometheus/sync`, { method: 'POST' }),
    onSuccess: () => {
      void qc.invalidateQueries({ queryKey: ['integrations', 'cloud', 'prometheus'] })
    },
  })
}

export function usePrometheusStatus() {
  return useQuery<PrometheusStatus>({
    queryKey: ['integrations', 'cloud', 'prometheus', 'status'],
    queryFn: () => apiFetch<PrometheusStatus>(`${BASE}/prometheus/status`),
    staleTime: 30_000,
  })
}

export function usePrometheusEvidence() {
  return useQuery<CloudEvidenceItem[]>({
    queryKey: ['integrations', 'cloud', 'prometheus', 'evidence'],
    queryFn: () => apiFetch<CloudEvidenceItem[]>(`${BASE}/prometheus/evidence`),
    staleTime: 30_000,
  })
}

// --- Entra ID ---

export interface EntraIDConfig {
  tenant_id: string
  client_id: string
  client_secret: string // "****" if set
  is_configured: boolean
}

export interface SaveEntraIDConfigInput {
  tenant_id: string
  client_id: string
  client_secret: string
}

export interface EntraIDStatus extends CloudSyncStatus {
  mfa_enrollment_pct: number
  risky_user_count: number
  inactive_user_count: number
}

export function useEntraIDConfig() {
  return useQuery<EntraIDConfig>({
    queryKey: ['integrations', 'cloud', 'entra-id', 'config'],
    queryFn: () => apiFetch<EntraIDConfig>(`${BASE}/entra-id/config`),
    staleTime: 60_000,
  })
}

export function useSaveEntraIDConfig() {
  const qc = useQueryClient()
  return useMutation<{ status: string }, Error, SaveEntraIDConfigInput>({
    mutationFn: (data) =>
      apiFetch<{ status: string }>(`${BASE}/entra-id/config`, {
        method: 'PUT',
        body: JSON.stringify(data),
      }),
    onSuccess: () => {
      void qc.invalidateQueries({ queryKey: ['integrations', 'cloud', 'entra-id'] })
    },
  })
}

export function useSyncEntraID() {
  const qc = useQueryClient()
  return useMutation<CloudSyncResult>({
    mutationFn: () => apiFetch<CloudSyncResult>(`${BASE}/entra-id/sync`, { method: 'POST' }),
    onSuccess: () => {
      void qc.invalidateQueries({ queryKey: ['integrations', 'cloud', 'entra-id'] })
    },
  })
}

export function useEntraIDStatus() {
  return useQuery<EntraIDStatus>({
    queryKey: ['integrations', 'cloud', 'entra-id', 'status'],
    queryFn: () => apiFetch<EntraIDStatus>(`${BASE}/entra-id/status`),
    staleTime: 30_000,
  })
}

export function useEntraIDEvidence() {
  return useQuery<CloudEvidenceItem[]>({
    queryKey: ['integrations', 'cloud', 'entra-id', 'evidence'],
    queryFn: () => apiFetch<CloudEvidenceItem[]>(`${BASE}/entra-id/evidence`),
    staleTime: 30_000,
  })
}

// --- Intune (S88-7) ---

export interface IntuneConfig {
  tenant_id: string
  client_id: string
  client_secret: string // "****" if set
  is_configured: boolean
}

export interface SaveIntuneConfigInput {
  tenant_id: string
  client_id: string
  client_secret: string
}

export interface IntuneStatus extends CloudSyncStatus {
  device_compliance_pct: number
}

export function useIntuneConfig() {
  return useQuery<IntuneConfig>({
    queryKey: ['integrations', 'cloud', 'intune', 'config'],
    queryFn: () => apiFetch<IntuneConfig>(`${BASE}/intune/config`),
    staleTime: 60_000,
  })
}

export function useSaveIntuneConfig() {
  const qc = useQueryClient()
  return useMutation<{ status: string }, Error, SaveIntuneConfigInput>({
    mutationFn: (data) =>
      apiFetch<{ status: string }>(`${BASE}/intune/config`, {
        method: 'PUT',
        body: JSON.stringify(data),
      }),
    onSuccess: () => {
      void qc.invalidateQueries({ queryKey: ['integrations', 'cloud', 'intune'] })
    },
  })
}

export function useSyncIntune() {
  const qc = useQueryClient()
  return useMutation<CloudSyncResult>({
    mutationFn: () => apiFetch<CloudSyncResult>(`${BASE}/intune/sync`, { method: 'POST' }),
    onSuccess: () => {
      void qc.invalidateQueries({ queryKey: ['integrations', 'cloud', 'intune'] })
    },
  })
}

export function useIntuneStatus() {
  return useQuery<IntuneStatus>({
    queryKey: ['integrations', 'cloud', 'intune', 'status'],
    queryFn: () => apiFetch<IntuneStatus>(`${BASE}/intune/status`),
    staleTime: 30_000,
  })
}

export function useIntuneEvidence() {
  return useQuery<CloudEvidenceItem[]>({
    queryKey: ['integrations', 'cloud', 'intune', 'evidence'],
    queryFn: () => apiFetch<CloudEvidenceItem[]>(`${BASE}/intune/evidence`),
    staleTime: 30_000,
  })
}

// --- Keycloak ---

export interface KeycloakConfig {
  keycloak_url: string
  realm: string
  client_id: string
  client_secret: string // "****" if set
  is_configured: boolean
}

export interface SaveKeycloakConfigInput {
  keycloak_url: string
  realm: string
  client_id: string
  client_secret: string
}

export interface KeycloakStatus extends CloudSyncStatus {
  user_count: number
  mfa_enrollment_pct: number
}

export function useKeycloakConfig() {
  return useQuery<KeycloakConfig>({
    queryKey: ['integrations', 'cloud', 'keycloak', 'config'],
    queryFn: () => apiFetch<KeycloakConfig>(`${BASE}/keycloak/config`),
    staleTime: 60_000,
  })
}

export function useSaveKeycloakConfig() {
  const qc = useQueryClient()
  return useMutation<{ status: string }, Error, SaveKeycloakConfigInput>({
    mutationFn: (data) =>
      apiFetch<{ status: string }>(`${BASE}/keycloak/config`, {
        method: 'PUT',
        body: JSON.stringify(data),
      }),
    onSuccess: () => {
      void qc.invalidateQueries({ queryKey: ['integrations', 'cloud', 'keycloak'] })
    },
  })
}

export function useSyncKeycloak() {
  const qc = useQueryClient()
  return useMutation<CloudSyncResult>({
    mutationFn: () => apiFetch<CloudSyncResult>(`${BASE}/keycloak/sync`, { method: 'POST' }),
    onSuccess: () => {
      void qc.invalidateQueries({ queryKey: ['integrations', 'cloud', 'keycloak'] })
    },
  })
}

export function useKeycloakStatus() {
  return useQuery<KeycloakStatus>({
    queryKey: ['integrations', 'cloud', 'keycloak', 'status'],
    queryFn: () => apiFetch<KeycloakStatus>(`${BASE}/keycloak/status`),
    staleTime: 30_000,
  })
}

export function useKeycloakEvidence() {
  return useQuery<CloudEvidenceItem[]>({
    queryKey: ['integrations', 'cloud', 'keycloak', 'evidence'],
    queryFn: () => apiFetch<CloudEvidenceItem[]>(`${BASE}/keycloak/evidence`),
    staleTime: 30_000,
  })
}

// --- LDAP ---

export interface LDAPConfig {
  host: string
  port: number
  bind_dn: string
  bind_password: string // "****" if set
  base_dn: string
  use_tls: boolean
  is_active_directory: boolean
  privileged_groups: string[]
  is_configured: boolean
}

export interface SaveLDAPConfigInput {
  host: string
  port: number
  bind_dn: string
  bind_password: string
  base_dn: string
  use_tls: boolean
  is_active_directory: boolean
  privileged_groups: string[]
}

export interface LDAPStatus extends CloudSyncStatus {
  user_count: number
  inactive_count: number
  privileged_count: number
}

export function useLDAPConfig() {
  return useQuery<LDAPConfig>({
    queryKey: ['integrations', 'cloud', 'ldap', 'config'],
    queryFn: () => apiFetch<LDAPConfig>(`${BASE}/ldap/config`),
    staleTime: 60_000,
  })
}

export function useSaveLDAPConfig() {
  const qc = useQueryClient()
  return useMutation<{ status: string }, Error, SaveLDAPConfigInput>({
    mutationFn: (data) =>
      apiFetch<{ status: string }>(`${BASE}/ldap/config`, {
        method: 'PUT',
        body: JSON.stringify(data),
      }),
    onSuccess: () => {
      void qc.invalidateQueries({ queryKey: ['integrations', 'cloud', 'ldap'] })
    },
  })
}

export function useSyncLDAP() {
  const qc = useQueryClient()
  return useMutation<CloudSyncResult>({
    mutationFn: () => apiFetch<CloudSyncResult>(`${BASE}/ldap/sync`, { method: 'POST' }),
    onSuccess: () => {
      void qc.invalidateQueries({ queryKey: ['integrations', 'cloud', 'ldap'] })
    },
  })
}

export function useLDAPStatus() {
  return useQuery<LDAPStatus>({
    queryKey: ['integrations', 'cloud', 'ldap', 'status'],
    queryFn: () => apiFetch<LDAPStatus>(`${BASE}/ldap/status`),
    staleTime: 30_000,
  })
}

export function useLDAPEvidence() {
  return useQuery<CloudEvidenceItem[]>({
    queryKey: ['integrations', 'cloud', 'ldap', 'evidence'],
    queryFn: () => apiFetch<CloudEvidenceItem[]>(`${BASE}/ldap/evidence`),
    staleTime: 30_000,
  })
}

// --- GitLab ---

export interface GitLabConfig {
  gitlab_url: string
  access_token: string // "****" if set
  group_id: string
  is_configured: boolean
}

export interface SaveGitLabConfigInput {
  gitlab_url: string
  access_token: string
  group_id: string
}

export interface GitLabStatus extends CloudSyncStatus {
  project_count: number
  unprotected_branches_count: number
}

export function useGitLabConfig() {
  return useQuery<GitLabConfig>({
    queryKey: ['integrations', 'cloud', 'gitlab', 'config'],
    queryFn: () => apiFetch<GitLabConfig>(`${BASE}/gitlab/config`),
    staleTime: 60_000,
  })
}

export function useSaveGitLabConfig() {
  const qc = useQueryClient()
  return useMutation<{ status: string }, Error, SaveGitLabConfigInput>({
    mutationFn: (data) =>
      apiFetch<{ status: string }>(`${BASE}/gitlab/config`, {
        method: 'PUT',
        body: JSON.stringify(data),
      }),
    onSuccess: () => {
      void qc.invalidateQueries({ queryKey: ['integrations', 'cloud', 'gitlab'] })
    },
  })
}

export function useSyncGitLab() {
  const qc = useQueryClient()
  return useMutation<CloudSyncResult>({
    mutationFn: () => apiFetch<CloudSyncResult>(`${BASE}/gitlab/sync`, { method: 'POST' }),
    onSuccess: () => {
      void qc.invalidateQueries({ queryKey: ['integrations', 'cloud', 'gitlab'] })
    },
  })
}

export function useGitLabStatus() {
  return useQuery<GitLabStatus>({
    queryKey: ['integrations', 'cloud', 'gitlab', 'status'],
    queryFn: () => apiFetch<GitLabStatus>(`${BASE}/gitlab/status`),
    staleTime: 30_000,
  })
}

export function useGitLabEvidence() {
  return useQuery<CloudEvidenceItem[]>({
    queryKey: ['integrations', 'cloud', 'gitlab', 'evidence'],
    queryFn: () => apiFetch<CloudEvidenceItem[]>(`${BASE}/gitlab/evidence`),
    staleTime: 30_000,
  })
}

// --- SonarQube ---

export interface SonarQubeConfig {
  base_url: string
  token: string // "****" if set
  is_configured: boolean
}

export interface SaveSonarQubeConfigInput {
  base_url: string
  token: string
}

export interface SonarQubeStatus extends CloudSyncStatus {
  project_count: number
  quality_gate_failed_count: number
  hotspot_count: number
}

export function useSonarQubeConfig() {
  return useQuery<SonarQubeConfig>({
    queryKey: ['integrations', 'cloud', 'sonarqube', 'config'],
    queryFn: () => apiFetch<SonarQubeConfig>(`${BASE}/sonarqube/config`),
    staleTime: 60_000,
  })
}

export function useSaveSonarQubeConfig() {
  const qc = useQueryClient()
  return useMutation<{ status: string }, Error, SaveSonarQubeConfigInput>({
    mutationFn: (data) =>
      apiFetch<{ status: string }>(`${BASE}/sonarqube/config`, {
        method: 'PUT',
        body: JSON.stringify(data),
      }),
    onSuccess: () => {
      void qc.invalidateQueries({ queryKey: ['integrations', 'cloud', 'sonarqube'] })
    },
  })
}

export function useSyncSonarQube() {
  const qc = useQueryClient()
  return useMutation<CloudSyncResult>({
    mutationFn: () => apiFetch<CloudSyncResult>(`${BASE}/sonarqube/sync`, { method: 'POST' }),
    onSuccess: () => {
      void qc.invalidateQueries({ queryKey: ['integrations', 'cloud', 'sonarqube'] })
    },
  })
}

export function useSonarQubeStatus() {
  return useQuery<SonarQubeStatus>({
    queryKey: ['integrations', 'cloud', 'sonarqube', 'status'],
    queryFn: () => apiFetch<SonarQubeStatus>(`${BASE}/sonarqube/status`),
    staleTime: 30_000,
  })
}

export function useSonarQubeEvidence() {
  return useQuery<CloudEvidenceItem[]>({
    queryKey: ['integrations', 'cloud', 'sonarqube', 'evidence'],
    queryFn: () => apiFetch<CloudEvidenceItem[]>(`${BASE}/sonarqube/evidence`),
    staleTime: 30_000,
  })
}

// --- Personio ---

export interface PersonioConfig {
  webhook_secret: string // "****" if set
  is_configured: boolean
}

export interface SavePersonioConfigInput {
  webhook_secret: string
}

export interface PersonioStatus extends CloudSyncStatus {
  webhook_url: string
  webhook_configured: boolean
  offboardings_triggered: number
  offboardings_completed_on_time: number
}

export function usePersonioConfig() {
  return useQuery<PersonioConfig>({
    queryKey: ['integrations', 'cloud', 'personio', 'config'],
    queryFn: () => apiFetch<PersonioConfig>(`${BASE}/personio/config`),
    staleTime: 60_000,
  })
}

export function useSavePersonioConfig() {
  const qc = useQueryClient()
  return useMutation<{ status: string }, Error, SavePersonioConfigInput>({
    mutationFn: (data) =>
      apiFetch<{ status: string }>(`${BASE}/personio/config`, {
        method: 'PUT',
        body: JSON.stringify(data),
      }),
    onSuccess: () => {
      void qc.invalidateQueries({ queryKey: ['integrations', 'cloud', 'personio'] })
    },
  })
}

export function usePersonioStatus() {
  return useQuery<PersonioStatus>({
    queryKey: ['integrations', 'cloud', 'personio', 'status'],
    queryFn: () => apiFetch<PersonioStatus>(`${BASE}/personio/status`),
    staleTime: 30_000,
  })
}
