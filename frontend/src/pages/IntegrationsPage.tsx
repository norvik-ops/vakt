import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { Plug, GitBranch, RefreshCw, Trash2, ChevronDown, ChevronUp, CheckCircle2, XCircle, AlertCircle, Plus, Cloud, ShieldAlert, Copy } from 'lucide-react'
import { Spinner } from '../components/Spinner'
import {
  useGitHubIntegrations,
  useAddGitHubIntegration,
  useDeleteGitHubIntegration,
  useSyncGitHubIntegration,
  useGitHubCheckResults,
  type GitHubIntegration,
  type GitHubCheckResult,
} from '../hooks/useGitHub'
import {
  useAWSConfig,
  useSaveAWSConfig,
  useTestAWSConnection,
  useSyncAWS,
  useAWSStatus,
  useAWSEvidence,
  useAzureConfig,
  useSaveAzureConfig,
  useTestAzureConnection,
  useSyncAzure,
  useAzureStatus,
  useAzureEvidence,
  useHetznerConfig,
  useSaveHetznerConfig,
  useSyncHetzner,
  useHetznerStatus,
  useHetznerEvidence,
  useIONOSConfig,
  useSaveIONOSConfig,
  useSyncIONOS,
  useIONOSStatus,
  useIONOSEvidence,
  useWazuhConfig,
  useSaveWazuhConfig,
  useSyncWazuh,
  useWazuhStatus,
  useWazuhEvidence,
  usePrometheusConfig,
  useSavePrometheusConfig,
  useSyncPrometheus,
  usePrometheusStatus,
  usePrometheusEvidence,
  useEntraIDConfig,
  useSaveEntraIDConfig,
  useSyncEntraID,
  useEntraIDStatus,
  useEntraIDEvidence,
  useIntuneConfig,
  useSaveIntuneConfig,
  useSyncIntune,
  useIntuneStatus,
  useIntuneEvidence,
  useKeycloakConfig,
  useSaveKeycloakConfig,
  useSyncKeycloak,
  useKeycloakStatus,
  useKeycloakEvidence,
  useLDAPConfig,
  useSaveLDAPConfig,
  useSyncLDAP,
  useLDAPStatus,
  useLDAPEvidence,
  useGitLabConfig,
  useSaveGitLabConfig,
  useSyncGitLab,
  useGitLabStatus,
  useGitLabEvidence,
  useSonarQubeConfig,
  useSaveSonarQubeConfig,
  useSyncSonarQube,
  useSonarQubeStatus,
  useSonarQubeEvidence,
  usePersonioConfig,
  useSavePersonioConfig,
  usePersonioStatus,
  type CloudEvidenceItem,
} from '../hooks/useCloud'
import { toast } from '../shared/hooks/useToast'
import { useFormatDate } from '../shared/hooks/useFormatDate'

// --- Status badge ---

function SyncStatusBadge({ status }: { status: string }) {
  const { t } = useTranslation()
  if (status === 'ok') {
    return (
      <span className="inline-flex items-center gap-1 text-xs font-medium text-emerald-700 bg-emerald-50 border border-emerald-200 rounded-full px-2 py-0.5">
        <CheckCircle2 className="w-3 h-3" /> {t('integrations.page.syncSuccess')}
      </span>
    )
  }
  if (status === 'error') {
    return (
      <span className="inline-flex items-center gap-1 text-xs font-medium text-red-700 bg-red-50 border border-red-200 rounded-full px-2 py-0.5">
        <XCircle className="w-3 h-3" /> {t('integrations.page.syncError')}
      </span>
    )
  }
  return (
    <span className="inline-flex items-center gap-1 text-xs font-medium text-amber-700 bg-amber-50 border border-amber-200 rounded-full px-2 py-0.5">
      <AlertCircle className="w-3 h-3" /> {t('integrations.page.syncPending')}
    </span>
  )
}

function CheckStatusBadge({ status }: { status: string }) {
  const { t } = useTranslation()
  if (status === 'pass') {
    return (
      <span className="inline-flex items-center gap-1 text-xs font-medium text-emerald-700 bg-emerald-50 border border-emerald-200 rounded-full px-2 py-0.5">
        <CheckCircle2 className="w-3 h-3" /> {t('integrations.page.checkPass')}
      </span>
    )
  }
  if (status === 'fail') {
    return (
      <span className="inline-flex items-center gap-1 text-xs font-medium text-red-700 bg-red-50 border border-red-200 rounded-full px-2 py-0.5">
        <XCircle className="w-3 h-3" /> {t('integrations.page.checkFail')}
      </span>
    )
  }
  return (
    <span className="inline-flex items-center gap-1 text-xs font-medium text-secondary bg-surface border border-border rounded-full px-2 py-0.5">
      <AlertCircle className="w-3 h-3" /> {t('integrations.page.checkUnknown')}
    </span>
  )
}

// --- Check label ---

function checkTypeLabel(type: string): string {
  const labels: Record<string, string> = {
    branch_protection: 'Branch Protection',
    pr_review_required: 'PR Review erforderlich',
    dependency_alerts: 'Dependency Alerts',
    secret_scanning: 'Secret Scanning',
  }
  return labels[type] ?? type
}

// --- Check results panel ---

function CheckResultsPanel({ integrationId }: { integrationId: string }) {
  const { t } = useTranslation()
  const { data: checks, isLoading } = useGitHubCheckResults(integrationId)

  if (isLoading) {
    return <p className="text-xs text-secondary py-2">{t('integrations.page.loadingChecks')}</p>
  }

  if (!checks || checks.length === 0) {
    return <p className="text-xs text-secondary py-2">{t('integrations.page.noCheckResults')}</p>
  }

  // Show only the latest result per check_type
  const latestByType = new Map<string, GitHubCheckResult>()
  for (const c of checks) {
    if (!latestByType.has(c.check_type)) {
      latestByType.set(c.check_type, c)
    }
  }

  return (
    <div className="mt-3 space-y-2">
      {Array.from(latestByType.values()).map((cr) => (
        <div key={cr.check_type} className="flex items-start justify-between gap-2 bg-bg rounded-md border border-border px-3 py-2">
          <div>
            <p className="text-xs font-medium text-primary">{checkTypeLabel(cr.check_type)}</p>
            {cr.details && (
              <p className="text-[11px] text-secondary mt-0.5">
                {Object.entries(cr.details)
                  .filter(([k]) => k !== 'error')
                  .map(([k, v]) => `${k}: ${String(v)}`)
                  .join(' · ')}
              </p>
            )}
            {!!cr.details?.error && (
              <p className="text-[11px] text-red-500 mt-0.5">{typeof cr.details.error === 'string' ? cr.details.error : JSON.stringify(cr.details.error)}</p>
            )}
          </div>
          <CheckStatusBadge status={cr.status} />
        </div>
      ))}
    </div>
  )
}

// --- Integration row ---

function IntegrationRow({ integration }: { integration: GitHubIntegration }) {
  const { t } = useTranslation()
  const [expanded, setExpanded] = useState(false)
  const deleteIntegration = useDeleteGitHubIntegration()
  const syncIntegration = useSyncGitHubIntegration()
  const { formatDateTime } = useFormatDate()

  const lastSync = integration.last_synced_at
    ? formatDateTime(integration.last_synced_at, { dateStyle: 'short', timeStyle: 'short' })
    : null

  function handleSync() {
    syncIntegration.mutate(integration.id)
  }

  function handleDelete() {
    if (confirm(`Integration ${integration.repo_owner}/${integration.repo_name} wirklich entfernen?`)) {
      deleteIntegration.mutate(integration.id)
    }
  }

  return (
    <div className="border border-border rounded-lg bg-surface">
      <div className="flex items-center gap-3 px-4 py-3">
        <GitBranch className="w-5 h-5 text-secondary shrink-0" />
        <div className="flex-1 min-w-0">
          <p className="text-sm font-medium text-primary truncate">
            {integration.repo_owner}/{integration.repo_name}
          </p>
          <p className="text-xs text-secondary">
            {lastSync ? t('integrations.page.lastSync', { date: lastSync }) : t('integrations.page.notSyncedYet')}
          </p>
          {integration.sync_error && (
            <p className="text-xs text-red-500 truncate">{integration.sync_error}</p>
          )}
        </div>
        <SyncStatusBadge status={integration.sync_status} />
        <div className="flex items-center gap-1">
          <button
            onClick={handleSync}
            disabled={syncIntegration.isPending}
            title="Jetzt synchronisieren"
            className="p-1.5 rounded-md text-secondary hover:text-primary hover:bg-bg transition-colors disabled:opacity-50"
          >
            <RefreshCw className={`w-4 h-4 ${syncIntegration.isPending ? 'animate-spin' : ''}`} />
          </button>
          <button
            onClick={() => { setExpanded((v) => !v); }}
            title="Details anzeigen"
            className="p-1.5 rounded-md text-secondary hover:text-primary hover:bg-bg transition-colors"
          >
            {expanded ? <ChevronUp className="w-4 h-4" /> : <ChevronDown className="w-4 h-4" />}
          </button>
          <button
            onClick={handleDelete}
            disabled={deleteIntegration.isPending}
            title="Integration entfernen"
            className="p-1.5 rounded-md text-secondary hover:text-red-500 hover:bg-bg transition-colors disabled:opacity-50"
          >
            <Trash2 className="w-4 h-4" />
          </button>
        </div>
      </div>
      {expanded && (
        <div className="border-t border-border px-4 py-3">
          <CheckResultsPanel integrationId={integration.id} />
        </div>
      )}
    </div>
  )
}

// --- Add integration dialog ---

function AddIntegrationDialog({ onClose }: { onClose: () => void }) {
  const { t } = useTranslation()
  const addIntegration = useAddGitHubIntegration()
  const [owner, setOwner] = useState('')
  const [repo, setRepo] = useState('')
  const [token, setToken] = useState('')
  const [error, setError] = useState('')

  function handleSubmit(e: React.FormEvent) {
    e.preventDefault()
    setError('')
    if (!owner.trim() || !repo.trim() || !token.trim()) {
      setError(t('integrations.page.allFieldsRequired'))
      return
    }
    addIntegration.mutate(
      { repo_owner: owner.trim(), repo_name: repo.trim(), access_token: token.trim() },
      {
        onSuccess: () => { onClose(); },
        onError: (err) => { setError(err.message); },
      },
    )
  }

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/40">
      <div className="bg-surface border border-border rounded-xl shadow-xl w-full max-w-md p-6">
        <h2 className="text-base font-semibold text-primary mb-4">{t('integrations.page.connectRepo')}</h2>
        <form onSubmit={handleSubmit} className="space-y-4">
          <div>
            <label className="block text-xs font-medium text-secondary mb-1">{t('integrations.page.repoOwner')}</label>
            <input
              type="text"
              value={owner}
              onChange={(e) => { setOwner(e.target.value); }}
              placeholder="z.B. my-org"
              className="w-full border border-border rounded-md px-3 py-2 text-sm bg-bg text-primary placeholder:text-secondary focus:outline-none focus:ring-2 focus:ring-brand/30"
            />
          </div>
          <div>
            <label className="block text-xs font-medium text-secondary mb-1">{t('integrations.page.repoName')}</label>
            <input
              type="text"
              value={repo}
              onChange={(e) => { setRepo(e.target.value); }}
              placeholder="z.B. my-repo"
              className="w-full border border-border rounded-md px-3 py-2 text-sm bg-bg text-primary placeholder:text-secondary focus:outline-none focus:ring-2 focus:ring-brand/30"
            />
          </div>
          <div>
            <label className="block text-xs font-medium text-secondary mb-1">Personal Access Token</label>
            <input
              type="password"
              value={token}
              onChange={(e) => { setToken(e.target.value); }}
              placeholder="ghp_..."
              className="w-full border border-border rounded-md px-3 py-2 text-sm bg-bg text-primary placeholder:text-secondary focus:outline-none focus:ring-2 focus:ring-brand/30"
            />
            <p className="text-[11px] text-secondary mt-1">
              Token wird AES-256-GCM verschlüsselt gespeichert. Benötigte Scopes: <code>repo</code>, <code>read:org</code>.
            </p>
          </div>
          {error && <p className="text-xs text-red-500">{error}</p>}
          <div className="flex justify-end gap-2 pt-2">
            <button
              type="button"
              onClick={onClose}
              className="px-4 py-2 text-sm rounded-md border border-border text-secondary hover:text-primary hover:bg-bg transition-colors"
            >
              {t('common.cancel')}
            </button>
            <button
              type="submit"
              disabled={addIntegration.isPending}
              className="px-4 py-2 text-sm rounded-md bg-brand text-white hover:bg-brand/90 transition-colors disabled:opacity-50"
            >
              {addIntegration.isPending ? t('integrations.page.connecting') : t('integrations.page.connect')}
            </button>
          </div>
        </form>
      </div>
    </div>
  )
}

// --- GitHub tab ---

function GitHubTab() {
  const { t } = useTranslation()
  const { data: integrations, isLoading, error } = useGitHubIntegrations()
  const [showDialog, setShowDialog] = useState(false)

  if (isLoading) {
    return (
      <div className="flex items-center justify-center h-32">
        <Spinner size="md" />
      </div>
    )
  }

  if (error) {
    return <p className="text-sm text-red-500">{t('common.error')}: {error.message}</p>
  }

  return (
    <div>
      <div className="flex items-center justify-between mb-4">
        <div>
          <h2 className="text-sm font-semibold text-primary">GitHub Repositories</h2>
          <p className="text-xs text-secondary mt-0.5">
            Automatische Compliance-Checks: Branch Protection, PR-Reviews, Dependency Alerts, Secret Scanning.
          </p>
        </div>
        <button
          onClick={() => { setShowDialog(true); }}
          className="inline-flex items-center gap-1.5 px-3 py-1.5 text-xs font-medium bg-brand text-white rounded-md hover:bg-brand/90 transition-colors"
        >
          <Plus className="w-3.5 h-3.5" />
          {t('integrations.page.connectRepo')}
        </button>
      </div>

      {integrations && integrations.length === 0 ? (
        <div className="border border-dashed border-border rounded-lg p-8 text-center">
          <GitBranch className="w-8 h-8 text-secondary mx-auto mb-2" />
          <p className="text-sm font-medium text-primary">{t('integrations.page.noReposConnected')}</p>
          <p className="text-xs text-secondary mt-1">
            {t('integrations.page.connectFirstRepo')}
          </p>
        </div>
      ) : (
        <div className="space-y-3">
          {(integrations ?? []).map((ig) => (
            <IntegrationRow key={ig.id} integration={ig} />
          ))}
        </div>
      )}

      {showDialog && <AddIntegrationDialog onClose={() => { setShowDialog(false); }} />}
    </div>
  )
}

// --- No third-party integrations info box ---

function NoThirdPartyInfoBox() {
  const { t } = useTranslation()
  return (
    <div className="flex items-start gap-4 p-5 rounded-xl border border-border bg-surface max-w-lg">
      <ShieldAlert className="w-6 h-6 text-amber-500 shrink-0 mt-0.5" />
      <div>
        <p className="text-sm font-semibold text-primary mb-1">{t('integrations.page.noThirdPartyTitle')}</p>
        <p className="text-xs text-secondary leading-relaxed">
          {t('integrations.page.noThirdPartyDesc')}
        </p>
      </div>
    </div>
  )
}

// --- Cloud sync status badge ---

function SyncLastBadge({ status, lastSyncAt }: { status: string | null; lastSyncAt: string | null }) {
  const { t } = useTranslation()
  if (!lastSyncAt) {
    return (
      <span className="inline-flex items-center gap-1 text-xs font-medium text-amber-700 bg-amber-50 border border-amber-200 rounded-full px-2 py-0.5">
        <AlertCircle className="w-3 h-3" /> {t('integrations.page.neverSynced')}
      </span>
    )
  }
  if (status === 'success') {
    return (
      <span className="inline-flex items-center gap-1 text-xs font-medium text-emerald-700 bg-emerald-50 border border-emerald-200 rounded-full px-2 py-0.5">
        <CheckCircle2 className="w-3 h-3" /> {t('integrations.page.syncSuccess')}
      </span>
    )
  }
  return (
    <span className="inline-flex items-center gap-1 text-xs font-medium text-red-700 bg-red-50 border border-red-200 rounded-full px-2 py-0.5">
      <XCircle className="w-3 h-3" /> {t('integrations.page.syncError')}
    </span>
  )
}

// --- Recent evidence list ---

function RecentEvidenceList({ items }: { items: CloudEvidenceItem[] }) {
  const { t } = useTranslation()
  const { formatDateTime } = useFormatDate()
  if (items.length === 0) {
    return <p className="text-xs text-secondary py-2">{t('integrations.page.noEvidence')}</p>
  }
  return (
    <div className="mt-3 space-y-2">
      {items.map((item) => (
        <div key={item.id} className="flex items-start gap-2 bg-bg rounded-md border border-border px-3 py-2">
          <CheckCircle2 className="w-3.5 h-3.5 text-emerald-500 mt-0.5 shrink-0" />
          <div className="min-w-0">
            <p className="text-xs font-medium text-primary truncate">{item.title}</p>
            <p className="text-[11px] text-secondary mt-0.5">
              {formatDateTime(item.created_at, { dateStyle: 'short', timeStyle: 'short' })}
            </p>
          </div>
        </div>
      ))}
    </div>
  )
}

// --- AWS tab ---

const AWS_REGIONS = [
  'eu-central-1',
  'eu-west-1',
  'eu-west-2',
  'eu-west-3',
  'eu-north-1',
  'us-east-1',
  'us-east-2',
  'us-west-1',
  'us-west-2',
  'ap-southeast-1',
  'ap-northeast-1',
]

function AWSTab() {
  const { t } = useTranslation()
  const { data: cfg, isLoading } = useAWSConfig()
  const { data: status } = useAWSStatus()
  const { data: evidence } = useAWSEvidence()
  const saveConfig = useSaveAWSConfig()
  const testConnection = useTestAWSConnection()
  const syncAWS = useSyncAWS()
  const { formatDateTime } = useFormatDate()

  const [accessKeyID, setAccessKeyID] = useState('')
  const [secretAccessKey, setSecretAccessKey] = useState('')
  const [region, setRegion] = useState('eu-central-1')
  const [accountID, setAccountID] = useState('')
  const [initialized, setInitialized] = useState(false)
  const [testResult, setTestResult] = useState<{ ok: boolean; message: string } | null>(null)

  if (cfg && !initialized) {
    setAccessKeyID(cfg.access_key_id)
    setSecretAccessKey(cfg.secret_access_key)
    setRegion(cfg.region || 'eu-central-1')
    setAccountID(cfg.account_id)
    setInitialized(true)
  }

  async function handleSave(e: React.FormEvent) {
    e.preventDefault()
    try {
      await saveConfig.mutateAsync({ access_key_id: accessKeyID, secret_access_key: secretAccessKey, region, account_id: accountID })
      toast(t('integrations.page.saved'), 'success')
    } catch (err) {
      toast(err instanceof Error ? err.message : t('integrations.page.saveFailed'), 'error')
    }
  }

  async function handleTest() {
    setTestResult(null)
    try {
      const result = await testConnection.mutateAsync()
      setTestResult({ ok: result.ok, message: result.ok ? t('common.success') : (result.error ?? t('common.error')) })
    } catch (err) {
      setTestResult({ ok: false, message: err instanceof Error ? err.message : t('common.error') })
    }
  }

  async function handleSync() {
    try {
      const result = await syncAWS.mutateAsync()
      if (result.ok) {
        toast(`${t('integrations.page.saved')} — ${result.evidence_created} Evidence-Einträge erstellt`, 'success')
      } else {
        toast(result.error ?? t('common.error'), 'error')
      }
    } catch (err) {
      toast(err instanceof Error ? err.message : t('common.error'), 'error')
    }
  }

  if (isLoading) {
    return (
      <div className="flex items-center justify-center h-32">
        <Spinner size="md" />
      </div>
    )
  }

  const lastSyncFormatted = status?.last_sync_at
    ? formatDateTime(status.last_sync_at, { dateStyle: 'short', timeStyle: 'short' })
    : null

  return (
    <div>
      <div className="mb-4">
        <h2 className="text-sm font-semibold text-primary">AWS-Integration</h2>
        <p className="text-xs text-secondary mt-0.5">
          IAM-Richtlinien, CloudTrail-Konfiguration, S3-Verschlüsselung und MFA-Status automatisch als Compliance-Evidence erfassen.
          Credentials werden AES-256-GCM verschlüsselt gespeichert.
        </p>
      </div>

      {/* Status row */}
      {status && (
        <div className="flex items-center gap-3 mb-5 p-3 rounded-lg border border-border bg-surface">
          <div className="flex-1 min-w-0">
            <p className="text-xs text-secondary">
              {lastSyncFormatted ? t('integrations.page.lastSync', { date: lastSyncFormatted }) : t('integrations.page.neverSynced')}
              {status.evidence_count > 0 && ` · ${status.evidence_count} Evidence-Einträge`}
            </p>
            {status.last_sync_error && (
              <p className="text-xs text-red-500 truncate mt-0.5">{status.last_sync_error}</p>
            )}
          </div>
          <SyncLastBadge status={status.last_sync_status} lastSyncAt={status.last_sync_at} />
          <button
            onClick={() => { void handleSync() }}
            disabled={syncAWS.isPending || !cfg?.is_configured}
            title="Jetzt synchronisieren"
            className="p-1.5 rounded-md text-secondary hover:text-primary hover:bg-bg transition-colors disabled:opacity-50"
          >
            <RefreshCw className={`w-4 h-4 ${syncAWS.isPending ? 'animate-spin' : ''}`} />
          </button>
        </div>
      )}

      <form onSubmit={(e) => { void handleSave(e) }} className="space-y-4 max-w-lg">
        <div>
          <label className="block text-xs font-medium text-secondary mb-1">Access Key ID</label>
          <input
            type="text"
            value={accessKeyID}
            onChange={(e) => { setAccessKeyID(e.target.value); }}
            placeholder="AKIA..."
            className="w-full border border-border rounded-md px-3 py-2 text-sm bg-bg text-primary placeholder:text-secondary focus:outline-none focus:ring-2 focus:ring-brand/30 font-mono"
            required
          />
        </div>
        <div>
          <label className="block text-xs font-medium text-secondary mb-1">Secret Access Key</label>
          <input
            type="password"
            value={secretAccessKey}
            onChange={(e) => { setSecretAccessKey(e.target.value); }}
            placeholder={cfg?.is_configured ? '****' : 'Secret Access Key eingeben'}
            className="w-full border border-border rounded-md px-3 py-2 text-sm bg-bg text-primary placeholder:text-secondary focus:outline-none focus:ring-2 focus:ring-brand/30 font-mono"
            required
          />
          <p className="text-[11px] text-secondary mt-1">
            Empfohlen: IAM-Benutzer mit schreibgeschützter Policy (<code>SecurityAudit</code> + <code>ReadOnlyAccess</code>).
          </p>
        </div>
        <div>
          <label className="block text-xs font-medium text-secondary mb-1">Region</label>
          <select
            value={region}
            onChange={(e) => { setRegion(e.target.value); }}
            className="w-full border border-border rounded-md px-3 py-2 text-sm bg-bg text-primary focus:outline-none focus:ring-2 focus:ring-brand/30"
          >
            {AWS_REGIONS.map((r) => (
              <option key={r} value={r}>{r}</option>
            ))}
          </select>
        </div>
        <div>
          <label className="block text-xs font-medium text-secondary mb-1">Account ID (optional)</label>
          <input
            type="text"
            value={accountID}
            onChange={(e) => { setAccountID(e.target.value); }}
            placeholder="123456789012"
            className="w-full border border-border rounded-md px-3 py-2 text-sm bg-bg text-primary placeholder:text-secondary focus:outline-none focus:ring-2 focus:ring-brand/30 font-mono"
          />
        </div>

        {testResult && (
          <div className={`flex items-center gap-2 text-sm px-3 py-2 rounded-md border ${testResult.ok ? 'text-emerald-700 bg-emerald-50 border-emerald-200' : 'text-red-700 bg-red-50 border-red-200'}`}>
            {testResult.ok ? <CheckCircle2 className="w-4 h-4 shrink-0" /> : <XCircle className="w-4 h-4 shrink-0" />}
            {testResult.message}
          </div>
        )}

        <div className="flex items-center gap-2 pt-1">
          <button
            type="button"
            onClick={() => { void handleTest() }}
            disabled={testConnection.isPending || !cfg?.is_configured}
            className="px-3 py-1.5 text-xs rounded-md border border-border text-secondary hover:text-primary hover:bg-bg transition-colors disabled:opacity-50"
          >
            {testConnection.isPending ? t('integrations.page.testing') : t('integrations.page.testConnection')}
          </button>
          <button
            type="submit"
            disabled={saveConfig.isPending}
            className="px-4 py-1.5 text-xs font-medium bg-brand text-white rounded-md hover:bg-brand/90 transition-colors disabled:opacity-50"
          >
            {saveConfig.isPending ? t('integrations.page.saving') : t('integrations.page.save')}
          </button>
        </div>
      </form>

      {/* Recent evidence */}
      {evidence && evidence.length > 0 && (
        <div className="mt-6">
          <p className="text-xs font-medium text-secondary mb-2">{t('integrations.page.recentEvidence')}</p>
          <RecentEvidenceList items={evidence} />
        </div>
      )}
    </div>
  )
}

// --- Azure tab ---

function AzureTab() {
  const { t } = useTranslation()
  const { data: cfg, isLoading } = useAzureConfig()
  const { data: status } = useAzureStatus()
  const { data: evidence } = useAzureEvidence()
  const saveConfig = useSaveAzureConfig()
  const testConnection = useTestAzureConnection()
  const syncAzure = useSyncAzure()
  const { formatDateTime } = useFormatDate()

  const [tenantID, setTenantID] = useState('')
  const [clientID, setClientID] = useState('')
  const [clientSecret, setClientSecret] = useState('')
  const [subscriptionID, setSubscriptionID] = useState('')
  const [initialized, setInitialized] = useState(false)
  const [testResult, setTestResult] = useState<{ ok: boolean; message: string } | null>(null)

  if (cfg && !initialized) {
    setTenantID(cfg.tenant_id)
    setClientID(cfg.client_id)
    setClientSecret(cfg.client_secret)
    setSubscriptionID(cfg.subscription_id)
    setInitialized(true)
  }

  async function handleSave(e: React.FormEvent) {
    e.preventDefault()
    try {
      await saveConfig.mutateAsync({ tenant_id: tenantID, client_id: clientID, client_secret: clientSecret, subscription_id: subscriptionID })
      toast(t('integrations.page.saved'), 'success')
    } catch (err) {
      toast(err instanceof Error ? err.message : t('integrations.page.saveFailed'), 'error')
    }
  }

  async function handleTest() {
    setTestResult(null)
    try {
      const result = await testConnection.mutateAsync()
      setTestResult({ ok: result.ok, message: result.ok ? t('common.success') : (result.error ?? t('common.error')) })
    } catch (err) {
      setTestResult({ ok: false, message: err instanceof Error ? err.message : t('common.error') })
    }
  }

  async function handleSync() {
    try {
      const result = await syncAzure.mutateAsync()
      if (result.ok) {
        toast(`${t('integrations.page.saved')} — ${result.evidence_created} Evidence-Einträge erstellt`, 'success')
      } else {
        toast(result.error ?? t('common.error'), 'error')
      }
    } catch (err) {
      toast(err instanceof Error ? err.message : t('common.error'), 'error')
    }
  }

  if (isLoading) {
    return (
      <div className="flex items-center justify-center h-32">
        <Spinner size="md" />
      </div>
    )
  }

  const lastSyncFormatted = status?.last_sync_at
    ? formatDateTime(status.last_sync_at, { dateStyle: 'short', timeStyle: 'short' })
    : null

  return (
    <div>
      <div className="mb-4">
        <h2 className="text-sm font-semibold text-primary">Azure-Integration</h2>
        <p className="text-xs text-secondary mt-0.5">
          Azure Secure Score, Security Center Findings und Policy Compliance automatisch als Evidence erfassen.
          Client Secret wird AES-256-GCM verschlüsselt gespeichert.
        </p>
      </div>

      {/* Status row */}
      {status && (
        <div className="flex items-center gap-3 mb-5 p-3 rounded-lg border border-border bg-surface">
          <div className="flex-1 min-w-0">
            <p className="text-xs text-secondary">
              {lastSyncFormatted ? t('integrations.page.lastSync', { date: lastSyncFormatted }) : t('integrations.page.neverSynced')}
              {status.evidence_count > 0 && ` · ${status.evidence_count} Evidence-Einträge`}
            </p>
            {status.last_sync_error && (
              <p className="text-xs text-red-500 truncate mt-0.5">{status.last_sync_error}</p>
            )}
          </div>
          <SyncLastBadge status={status.last_sync_status} lastSyncAt={status.last_sync_at} />
          <button
            onClick={() => { void handleSync() }}
            disabled={syncAzure.isPending || !cfg?.is_configured}
            title="Jetzt synchronisieren"
            className="p-1.5 rounded-md text-secondary hover:text-primary hover:bg-bg transition-colors disabled:opacity-50"
          >
            <RefreshCw className={`w-4 h-4 ${syncAzure.isPending ? 'animate-spin' : ''}`} />
          </button>
        </div>
      )}

      <form onSubmit={(e) => { void handleSave(e) }} className="space-y-4 max-w-lg">
        <div>
          <label className="block text-xs font-medium text-secondary mb-1">Tenant ID</label>
          <input
            type="text"
            value={tenantID}
            onChange={(e) => { setTenantID(e.target.value); }}
            placeholder="xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx"
            className="w-full border border-border rounded-md px-3 py-2 text-sm bg-bg text-primary placeholder:text-secondary focus:outline-none focus:ring-2 focus:ring-brand/30 font-mono"
            required
          />
        </div>
        <div>
          <label className="block text-xs font-medium text-secondary mb-1">Client ID (App Registration)</label>
          <input
            type="text"
            value={clientID}
            onChange={(e) => { setClientID(e.target.value); }}
            placeholder="xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx"
            className="w-full border border-border rounded-md px-3 py-2 text-sm bg-bg text-primary placeholder:text-secondary focus:outline-none focus:ring-2 focus:ring-brand/30 font-mono"
            required
          />
        </div>
        <div>
          <label className="block text-xs font-medium text-secondary mb-1">Client Secret</label>
          <input
            type="password"
            value={clientSecret}
            onChange={(e) => { setClientSecret(e.target.value); }}
            placeholder={cfg?.is_configured ? '****' : 'Client Secret eingeben'}
            className="w-full border border-border rounded-md px-3 py-2 text-sm bg-bg text-primary placeholder:text-secondary focus:outline-none focus:ring-2 focus:ring-brand/30 font-mono"
            required
          />
          <p className="text-[11px] text-secondary mt-1">
            Service Principal mit <code>Security Reader</code> + <code>Policy Insights Reader</code> Rollen.
          </p>
        </div>
        <div>
          <label className="block text-xs font-medium text-secondary mb-1">Subscription ID</label>
          <input
            type="text"
            value={subscriptionID}
            onChange={(e) => { setSubscriptionID(e.target.value); }}
            placeholder="xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx"
            className="w-full border border-border rounded-md px-3 py-2 text-sm bg-bg text-primary placeholder:text-secondary focus:outline-none focus:ring-2 focus:ring-brand/30 font-mono"
            required
          />
        </div>

        {testResult && (
          <div className={`flex items-center gap-2 text-sm px-3 py-2 rounded-md border ${testResult.ok ? 'text-emerald-700 bg-emerald-50 border-emerald-200' : 'text-red-700 bg-red-50 border-red-200'}`}>
            {testResult.ok ? <CheckCircle2 className="w-4 h-4 shrink-0" /> : <XCircle className="w-4 h-4 shrink-0" />}
            {testResult.message}
          </div>
        )}

        <div className="flex items-center gap-2 pt-1">
          <button
            type="button"
            onClick={() => { void handleTest() }}
            disabled={testConnection.isPending || !cfg?.is_configured}
            className="px-3 py-1.5 text-xs rounded-md border border-border text-secondary hover:text-primary hover:bg-bg transition-colors disabled:opacity-50"
          >
            {testConnection.isPending ? t('integrations.page.testing') : t('integrations.page.testConnection')}
          </button>
          <button
            type="submit"
            disabled={saveConfig.isPending}
            className="px-4 py-1.5 text-xs font-medium bg-brand text-white rounded-md hover:bg-brand/90 transition-colors disabled:opacity-50"
          >
            {saveConfig.isPending ? t('integrations.page.saving') : t('integrations.page.save')}
          </button>
        </div>
      </form>

      {/* Recent evidence */}
      {evidence && evidence.length > 0 && (
        <div className="mt-6">
          <p className="text-xs font-medium text-secondary mb-2">{t('integrations.page.recentEvidence')}</p>
          <RecentEvidenceList items={evidence} />
        </div>
      )}
    </div>
  )
}

// --- Hetzner tab ---

const HETZNER_LOCATIONS = ['', 'nbg1', 'fsn1', 'hel1', 'ash', 'hil']

function HetznerTab() {
  const { t } = useTranslation()
  const { data: cfg, isLoading } = useHetznerConfig()
  const { data: status } = useHetznerStatus()
  const { data: evidence } = useHetznerEvidence()
  const saveConfig = useSaveHetznerConfig()
  const syncHetzner = useSyncHetzner()
  const { formatDateTime } = useFormatDate()

  const [apiToken, setApiToken] = useState('')
  const [location, setLocation] = useState('')
  const [initialized, setInitialized] = useState(false)

  if (cfg && !initialized) {
    setApiToken(cfg.api_token)
    setLocation(cfg.location)
    setInitialized(true)
  }

  async function handleSave(e: React.FormEvent) {
    e.preventDefault()
    try {
      await saveConfig.mutateAsync({ api_token: apiToken, location })
      toast(t('integrations.page.saved'), 'success')
    } catch (err) {
      toast(err instanceof Error ? err.message : t('integrations.page.saveFailed'), 'error')
    }
  }

  async function handleSync() {
    try {
      const result = await syncHetzner.mutateAsync()
      if (result.ok) {
        toast(`${t('integrations.page.saved')} — ${result.evidence_created} Evidence-Einträge erstellt`, 'success')
      } else {
        toast(result.error ?? t('common.error'), 'error')
      }
    } catch (err) {
      toast(err instanceof Error ? err.message : t('common.error'), 'error')
    }
  }

  if (isLoading) return <div className="flex items-center justify-center h-32"><Spinner size="md" /></div>

  const lastSyncFormatted = status?.last_sync_at
    ? formatDateTime(status.last_sync_at, { dateStyle: 'short', timeStyle: 'short' })
    : null

  return (
    <div>
      <div className="mb-4">
        <h2 className="text-sm font-semibold text-primary">Hetzner Cloud-Integration</h2>
        <p className="text-xs text-secondary mt-0.5">
          Server-Inventar, Firewall-Regeln, SSH-Keys und Snapshot-Nachweis täglich als Compliance-Evidence erfassen.
          API-Token wird AES-256-GCM verschlüsselt gespeichert.
        </p>
      </div>

      {status && (
        <div className="flex items-center gap-3 mb-5 p-3 rounded-lg border border-border bg-surface">
          <div className="flex-1 min-w-0">
            <p className="text-xs text-secondary">
              {lastSyncFormatted ? t('integrations.page.lastSync', { date: lastSyncFormatted }) : t('integrations.page.neverSynced')}
              {status.evidence_count > 0 && ` · ${status.evidence_count} Evidence-Einträge`}
            </p>
            {status.last_sync_error && <p className="text-xs text-red-500 truncate mt-0.5">{status.last_sync_error}</p>}
          </div>
          <SyncLastBadge status={status.last_sync_status} lastSyncAt={status.last_sync_at} />
          <button onClick={() => { void handleSync() }} disabled={syncHetzner.isPending || !cfg?.is_configured}
            title="Jetzt synchronisieren"
            className="p-1.5 rounded-md text-secondary hover:text-primary hover:bg-bg transition-colors disabled:opacity-50">
            <RefreshCw className={`w-4 h-4 ${syncHetzner.isPending ? 'animate-spin' : ''}`} />
          </button>
        </div>
      )}

      <form onSubmit={(e) => { void handleSave(e) }} className="space-y-4 max-w-lg">
        <div>
          <label className="block text-xs font-medium text-secondary mb-1">API-Token</label>
          <input type="password" value={apiToken} onChange={(e) => { setApiToken(e.target.value); }}
            placeholder={cfg?.is_configured ? '****' : 'Hetzner Cloud API-Token'}
            className="w-full border border-border rounded-md px-3 py-2 text-sm bg-bg text-primary placeholder:text-secondary focus:outline-none focus:ring-2 focus:ring-brand/30 font-mono"
            required />
          <p className="text-[11px] text-secondary mt-1">Empfohlen: Read-only-Token (Berechtigungen: Server, Firewall, SSH Key, Image)</p>
        </div>
        <div>
          <label className="block text-xs font-medium text-secondary mb-1">Standort-Filter (optional)</label>
          <select value={location} onChange={(e) => { setLocation(e.target.value); }}
            className="w-full border border-border rounded-md px-3 py-2 text-sm bg-bg text-primary focus:outline-none focus:ring-2 focus:ring-brand/30">
            {HETZNER_LOCATIONS.map((l) => <option key={l} value={l}>{l || '— Alle Standorte —'}</option>)}
          </select>
        </div>
        <div className="flex gap-2 pt-1">
          <button type="submit" disabled={saveConfig.isPending}
            className="px-4 py-1.5 text-xs font-medium bg-brand text-white rounded-md hover:bg-brand/90 transition-colors disabled:opacity-50">
            {saveConfig.isPending ? t('integrations.page.saving') : t('integrations.page.save')}
          </button>
        </div>
      </form>

      {evidence && evidence.length > 0 && (
        <div className="mt-6">
          <p className="text-xs font-medium text-secondary mb-2">{t('integrations.page.recentEvidence')}</p>
          <RecentEvidenceList items={evidence} />
        </div>
      )}
    </div>
  )
}

// --- IONOS tab ---

function IONOSTab() {
  const { t } = useTranslation()
  const { data: cfg, isLoading } = useIONOSConfig()
  const { data: status } = useIONOSStatus()
  const { data: evidence } = useIONOSEvidence()
  const saveConfig = useSaveIONOSConfig()
  const syncIONOS = useSyncIONOS()
  const { formatDateTime } = useFormatDate()

  const [authMode, setAuthMode] = useState<'credentials' | 'token'>('credentials')
  const [username, setUsername] = useState('')
  const [password, setPassword] = useState('')
  const [token, setToken] = useState('')
  const [initialized, setInitialized] = useState(false)

  if (cfg && !initialized) {
    setUsername(cfg.username)
    setPassword(cfg.password)
    setToken(cfg.token)
    if (cfg.token === '****') setAuthMode('token')
    setInitialized(true)
  }

  async function handleSave(e: React.FormEvent) {
    e.preventDefault()
    const input = authMode === 'token'
      ? { username: '', password: '', token }
      : { username, password, token: '' }
    try {
      await saveConfig.mutateAsync(input)
      toast(t('integrations.page.saved'), 'success')
    } catch (err) {
      toast(err instanceof Error ? err.message : t('integrations.page.saveFailed'), 'error')
    }
  }

  async function handleSync() {
    try {
      const result = await syncIONOS.mutateAsync()
      if (result.ok) {
        toast(`${t('integrations.page.saved')} — ${result.evidence_created} Evidence-Einträge erstellt`, 'success')
      } else {
        toast(result.error ?? t('common.error'), 'error')
      }
    } catch (err) {
      toast(err instanceof Error ? err.message : t('common.error'), 'error')
    }
  }

  if (isLoading) return <div className="flex items-center justify-center h-32"><Spinner size="md" /></div>

  const lastSyncFormatted = status?.last_sync_at
    ? formatDateTime(status.last_sync_at, { dateStyle: 'short', timeStyle: 'short' })
    : null

  return (
    <div>
      <div className="mb-4">
        <h2 className="text-sm font-semibold text-primary">IONOS Cloud-Integration</h2>
        <p className="text-xs text-secondary mt-0.5">
          Server-Inventar, SSH-Keys und Snapshot-Nachweis aus IONOS Cloud automatisch als Evidence erfassen.
          Credentials werden AES-256-GCM verschlüsselt gespeichert.
        </p>
      </div>

      {status && (
        <div className="flex items-center gap-3 mb-5 p-3 rounded-lg border border-border bg-surface">
          <div className="flex-1 min-w-0">
            <p className="text-xs text-secondary">
              {lastSyncFormatted ? t('integrations.page.lastSync', { date: lastSyncFormatted }) : t('integrations.page.neverSynced')}
              {status.evidence_count > 0 && ` · ${status.evidence_count} Evidence-Einträge`}
            </p>
            {status.last_sync_error && <p className="text-xs text-red-500 truncate mt-0.5">{status.last_sync_error}</p>}
          </div>
          <SyncLastBadge status={status.last_sync_status} lastSyncAt={status.last_sync_at} />
          <button onClick={() => { void handleSync() }} disabled={syncIONOS.isPending || !cfg?.is_configured}
            title="Jetzt synchronisieren"
            className="p-1.5 rounded-md text-secondary hover:text-primary hover:bg-bg transition-colors disabled:opacity-50">
            <RefreshCw className={`w-4 h-4 ${syncIONOS.isPending ? 'animate-spin' : ''}`} />
          </button>
        </div>
      )}

      <div className="flex gap-2 mb-4 max-w-lg">
        <button type="button" onClick={() => { setAuthMode('credentials'); }}
          className={`px-3 py-1 text-xs rounded-md border transition-colors ${authMode === 'credentials' ? 'bg-brand text-white border-brand' : 'border-border text-secondary hover:text-primary'}`}>
          Benutzername / Passwort
        </button>
        <button type="button" onClick={() => { setAuthMode('token'); }}
          className={`px-3 py-1 text-xs rounded-md border transition-colors ${authMode === 'token' ? 'bg-brand text-white border-brand' : 'border-border text-secondary hover:text-primary'}`}>
          API Token
        </button>
      </div>

      <form onSubmit={(e) => { void handleSave(e) }} className="space-y-4 max-w-lg">
        {authMode === 'credentials' ? (
          <>
            <div>
              <label className="block text-xs font-medium text-secondary mb-1">Benutzername</label>
              <input type="text" value={username} onChange={(e) => { setUsername(e.target.value); }}
                placeholder="IONOS-Benutzername"
                className="w-full border border-border rounded-md px-3 py-2 text-sm bg-bg text-primary placeholder:text-secondary focus:outline-none focus:ring-2 focus:ring-brand/30"
                required />
            </div>
            <div>
              <label className="block text-xs font-medium text-secondary mb-1">Passwort</label>
              <input type="password" value={password} onChange={(e) => { setPassword(e.target.value); }}
                placeholder={cfg?.is_configured && cfg.password === '****' ? '****' : 'Passwort eingeben'}
                className="w-full border border-border rounded-md px-3 py-2 text-sm bg-bg text-primary placeholder:text-secondary focus:outline-none focus:ring-2 focus:ring-brand/30"
                required />
            </div>
          </>
        ) : (
          <div>
            <label className="block text-xs font-medium text-secondary mb-1">API-Token</label>
            <input type="password" value={token} onChange={(e) => { setToken(e.target.value); }}
              placeholder={cfg?.is_configured && cfg.token === '****' ? '****' : 'IONOS API-Token'}
              className="w-full border border-border rounded-md px-3 py-2 text-sm bg-bg text-primary placeholder:text-secondary focus:outline-none focus:ring-2 focus:ring-brand/30 font-mono"
              required />
          </div>
        )}
        <div className="flex gap-2 pt-1">
          <button type="submit" disabled={saveConfig.isPending}
            className="px-4 py-1.5 text-xs font-medium bg-brand text-white rounded-md hover:bg-brand/90 transition-colors disabled:opacity-50">
            {saveConfig.isPending ? t('integrations.page.saving') : t('integrations.page.save')}
          </button>
        </div>
      </form>

      {evidence && evidence.length > 0 && (
        <div className="mt-6">
          <p className="text-xs font-medium text-secondary mb-2">{t('integrations.page.recentEvidence')}</p>
          <RecentEvidenceList items={evidence} />
        </div>
      )}
    </div>
  )
}

// --- Wazuh tab ---

function WazuhTab() {
  const { t } = useTranslation()
  const { data: cfg, isLoading } = useWazuhConfig()
  const { data: status } = useWazuhStatus()
  const { data: evidence } = useWazuhEvidence()
  const saveConfig = useSaveWazuhConfig()
  const syncWazuh = useSyncWazuh()
  const { formatDateTime } = useFormatDate()

  const [baseURL, setBaseURL] = useState('')
  const [username, setUsername] = useState('')
  const [password, setPassword] = useState('')
  const [verifyTLS, setVerifyTLS] = useState(true)
  const [initialized, setInitialized] = useState(false)

  if (cfg && !initialized) {
    setBaseURL(cfg.base_url)
    setUsername(cfg.username)
    setPassword(cfg.password)
    setVerifyTLS(cfg.verify_tls)
    setInitialized(true)
  }

  async function handleSave(e: React.FormEvent) {
    e.preventDefault()
    try {
      await saveConfig.mutateAsync({ base_url: baseURL, username, password, verify_tls: verifyTLS })
      toast(t('integrations.page.saved'), 'success')
    } catch (err) {
      toast(err instanceof Error ? err.message : t('integrations.page.saveFailed'), 'error')
    }
  }

  async function handleSync() {
    try {
      const result = await syncWazuh.mutateAsync()
      if (result.ok) {
        toast(`${t('integrations.page.saved')} — ${result.evidence_created} Evidence-Einträge erstellt`, 'success')
      } else {
        toast(result.error ?? t('common.error'), 'error')
      }
    } catch (err) {
      toast(err instanceof Error ? err.message : t('common.error'), 'error')
    }
  }

  if (isLoading) return <div className="flex items-center justify-center h-32"><Spinner size="md" /></div>

  const lastSyncFormatted = status?.last_sync_at
    ? formatDateTime(status.last_sync_at, { dateStyle: 'short', timeStyle: 'short' })
    : null

  return (
    <div>
      <div className="mb-4">
        <h2 className="text-sm font-semibold text-primary">Wazuh Pull-Integration</h2>
        <p className="text-xs text-secondary mt-0.5">
          Vulnerability-Scans, SCA-Compliance-Scores und FIM-Events täglich aus dem Wazuh-Manager als Compliance-Evidence erfassen.
          Passwort wird AES-256-GCM verschlüsselt gespeichert.
        </p>
      </div>

      {status && (
        <div className="flex items-center gap-3 mb-5 p-3 rounded-lg border border-border bg-surface">
          <div className="flex-1 min-w-0">
            <p className="text-xs text-secondary">
              {lastSyncFormatted ? t('integrations.page.lastSync', { date: lastSyncFormatted }) : t('integrations.page.neverSynced')}
              {status.evidence_count > 0 && ` · ${status.evidence_count} Evidence-Einträge`}
              {status.agent_count > 0 && ` · ${status.agent_count} Agents`}
              {status.agents_offline > 0 && ` · ${status.agents_offline} offline`}
            </p>
            {status.last_sync_error && <p className="text-xs text-red-500 truncate mt-0.5">{status.last_sync_error}</p>}
          </div>
          <SyncLastBadge status={status.last_sync_status} lastSyncAt={status.last_sync_at} />
          <button onClick={() => { void handleSync() }} disabled={syncWazuh.isPending || !cfg?.is_configured}
            title="Jetzt synchronisieren"
            className="p-1.5 rounded-md text-secondary hover:text-primary hover:bg-bg transition-colors disabled:opacity-50">
            <RefreshCw className={`w-4 h-4 ${syncWazuh.isPending ? 'animate-spin' : ''}`} />
          </button>
        </div>
      )}

      <form onSubmit={(e) => { void handleSave(e) }} className="space-y-4 max-w-lg">
        <div>
          <label className="block text-xs font-medium text-secondary mb-1">Wazuh Manager URL</label>
          <input type="url" value={baseURL} onChange={(e) => { setBaseURL(e.target.value); }}
            placeholder="https://wazuh-manager:55000"
            className="w-full border border-border rounded-md px-3 py-2 text-sm bg-bg text-primary placeholder:text-secondary focus:outline-none focus:ring-2 focus:ring-brand/30 font-mono"
            required />
        </div>
        <div>
          <label className="block text-xs font-medium text-secondary mb-1">Benutzername</label>
          <input type="text" value={username} onChange={(e) => { setUsername(e.target.value); }}
            placeholder="wazuh-readonly"
            className="w-full border border-border rounded-md px-3 py-2 text-sm bg-bg text-primary placeholder:text-secondary focus:outline-none focus:ring-2 focus:ring-brand/30"
            required />
        </div>
        <div>
          <label className="block text-xs font-medium text-secondary mb-1">Passwort</label>
          <input type="password" value={password} onChange={(e) => { setPassword(e.target.value); }}
            placeholder={cfg?.is_configured && cfg.password === '****' ? '****' : 'Passwort eingeben'}
            className="w-full border border-border rounded-md px-3 py-2 text-sm bg-bg text-primary placeholder:text-secondary focus:outline-none focus:ring-2 focus:ring-brand/30"
            required />
        </div>
        <div className="flex items-center gap-2">
          <input type="checkbox" id="wazuh-tls" checked={!verifyTLS} onChange={(e) => { setVerifyTLS(!e.target.checked); }}
            className="rounded border-border" />
          <label htmlFor="wazuh-tls" className="text-xs text-secondary">Self-signed Zertifikat akzeptieren (TLS-Verifizierung deaktivieren)</label>
        </div>
        <div className="flex gap-2 pt-1">
          <button type="submit" disabled={saveConfig.isPending}
            className="px-4 py-1.5 text-xs font-medium bg-brand text-white rounded-md hover:bg-brand/90 transition-colors disabled:opacity-50">
            {saveConfig.isPending ? t('integrations.page.saving') : t('integrations.page.save')}
          </button>
        </div>
      </form>

      {evidence && evidence.length > 0 && (
        <div className="mt-6">
          <p className="text-xs font-medium text-secondary mb-2">{t('integrations.page.recentEvidence')}</p>
          <RecentEvidenceList items={evidence} />
        </div>
      )}
    </div>
  )
}

// --- Prometheus tab ---

function PrometheusTab() {
  const { t } = useTranslation()
  const { data: cfg, isLoading } = usePrometheusConfig()
  const { data: status } = usePrometheusStatus()
  const { data: evidence } = usePrometheusEvidence()
  const saveConfig = useSavePrometheusConfig()
  const syncPrometheus = useSyncPrometheus()
  const { formatDateTime } = useFormatDate()

  const [prometheusURL, setPrometheusURL] = useState('')
  const [alertmanagerURL, setAlertmanagerURL] = useState('')
  const [token, setToken] = useState('')
  const [initialized, setInitialized] = useState(false)

  if (cfg && !initialized) {
    setPrometheusURL(cfg.prometheus_url)
    setAlertmanagerURL(cfg.alertmanager_url)
    setToken(cfg.token)
    setInitialized(true)
  }

  async function handleSave(e: React.FormEvent) {
    e.preventDefault()
    try {
      await saveConfig.mutateAsync({ prometheus_url: prometheusURL, alertmanager_url: alertmanagerURL, token })
      toast(t('integrations.page.saved'), 'success')
    } catch (err) {
      toast(err instanceof Error ? err.message : t('integrations.page.saveFailed'), 'error')
    }
  }

  async function handleSync() {
    try {
      const result = await syncPrometheus.mutateAsync()
      if (result.ok) {
        toast(`${t('integrations.page.saved')} — ${result.evidence_created} Evidence-Einträge erstellt`, 'success')
      } else {
        toast(result.error ?? t('common.error'), 'error')
      }
    } catch (err) {
      toast(err instanceof Error ? err.message : t('common.error'), 'error')
    }
  }

  if (isLoading) return <div className="flex items-center justify-center h-32"><Spinner size="md" /></div>

  const lastSyncFormatted = status?.last_sync_at
    ? formatDateTime(status.last_sync_at, { dateStyle: 'short', timeStyle: 'short' })
    : null

  return (
    <div>
      <div className="mb-4">
        <h2 className="text-sm font-semibold text-primary">Prometheus / Alertmanager-Integration</h2>
        <p className="text-xs text-secondary mt-0.5">
          Uptime-Metriken, aktive Alerts und Scrape-Target-Health täglich als Monitoring-Evidence erfassen.
          Bearer Token wird AES-256-GCM verschlüsselt gespeichert.
        </p>

      </div>

      {status && (
        <div className="flex items-center gap-3 mb-5 p-3 rounded-lg border border-border bg-surface">
          <div className="flex-1 min-w-0">
            <p className="text-xs text-secondary">
              {lastSyncFormatted ? t('integrations.page.lastSync', { date: lastSyncFormatted }) : t('integrations.page.neverSynced')}
              {status.evidence_count > 0 && ` · ${status.evidence_count} Evidence-Einträge`}
              {status.target_count > 0 && ` · ${status.target_count} Targets`}
              {status.active_alert_count > 0 && ` · ${status.active_alert_count} aktive Alerts`}
            </p>
            {status.last_sync_error && <p className="text-xs text-red-500 truncate mt-0.5">{status.last_sync_error}</p>}
          </div>
          <SyncLastBadge status={status.last_sync_status} lastSyncAt={status.last_sync_at} />
          <button onClick={() => { void handleSync() }} disabled={syncPrometheus.isPending || !cfg?.is_configured}
            title="Jetzt synchronisieren"
            className="p-1.5 rounded-md text-secondary hover:text-primary hover:bg-bg transition-colors disabled:opacity-50">
            <RefreshCw className={`w-4 h-4 ${syncPrometheus.isPending ? 'animate-spin' : ''}`} />
          </button>
        </div>
      )}

      <form onSubmit={(e) => { void handleSave(e) }} className="space-y-4 max-w-lg">
        <div>
          <label className="block text-xs font-medium text-secondary mb-1">Prometheus URL</label>
          <input type="url" value={prometheusURL} onChange={(e) => { setPrometheusURL(e.target.value); }}
            placeholder="http://prometheus:9090"
            className="w-full border border-border rounded-md px-3 py-2 text-sm bg-bg text-primary placeholder:text-secondary focus:outline-none focus:ring-2 focus:ring-brand/30 font-mono"
            required />
        </div>
        <div>
          <label className="block text-xs font-medium text-secondary mb-1">Alertmanager URL (optional)</label>
          <input type="url" value={alertmanagerURL} onChange={(e) => { setAlertmanagerURL(e.target.value); }}
            placeholder="http://alertmanager:9093"
            className="w-full border border-border rounded-md px-3 py-2 text-sm bg-bg text-primary placeholder:text-secondary focus:outline-none focus:ring-2 focus:ring-brand/30 font-mono" />
        </div>
        <div>
          <label className="block text-xs font-medium text-secondary mb-1">Bearer Token (optional)</label>
          <input type="password" value={token} onChange={(e) => { setToken(e.target.value); }}
            placeholder={cfg?.is_configured && cfg.token === '****' ? '****' : 'Leer lassen wenn keine Auth'}
            className="w-full border border-border rounded-md px-3 py-2 text-sm bg-bg text-primary placeholder:text-secondary focus:outline-none focus:ring-2 focus:ring-brand/30 font-mono" />
        </div>
        <div className="flex gap-2 pt-1">
          <button type="submit" disabled={saveConfig.isPending}
            className="px-4 py-1.5 text-xs font-medium bg-brand text-white rounded-md hover:bg-brand/90 transition-colors disabled:opacity-50">
            {saveConfig.isPending ? t('integrations.page.saving') : t('integrations.page.save')}
          </button>
        </div>
      </form>

      {evidence && evidence.length > 0 && (
        <div className="mt-6">
          <p className="text-xs font-medium text-secondary mb-2">{t('integrations.page.recentEvidence')}</p>
          <RecentEvidenceList items={evidence} />
        </div>
      )}
    </div>
  )
}

// --- Entra ID tab ---

function EntraIDTab() {
  const { t } = useTranslation()
  const { data: cfg, isLoading } = useEntraIDConfig()
  const { data: status } = useEntraIDStatus()
  const { data: evidence } = useEntraIDEvidence()
  const saveConfig = useSaveEntraIDConfig()
  const syncEntraID = useSyncEntraID()
  const { formatDateTime } = useFormatDate()

  const [tenantID, setTenantID] = useState('')
  const [clientID, setClientID] = useState('')
  const [clientSecret, setClientSecret] = useState('')
  const [initialized, setInitialized] = useState(false)

  if (cfg && !initialized) {
    setTenantID(cfg.tenant_id)
    setClientID(cfg.client_id)
    setClientSecret(cfg.client_secret)
    setInitialized(true)
  }

  async function handleSave(e: React.FormEvent) {
    e.preventDefault()
    try {
      await saveConfig.mutateAsync({ tenant_id: tenantID, client_id: clientID, client_secret: clientSecret })
      toast(t('integrations.page.saved'), 'success')
    } catch (err) {
      toast(err instanceof Error ? err.message : t('integrations.page.saveFailed'), 'error')
    }
  }

  async function handleSync() {
    try {
      const result = await syncEntraID.mutateAsync()
      if (result.ok) {
        toast(`${t('integrations.page.saved')} — ${result.evidence_created} Evidence-Einträge erstellt`, 'success')
      } else {
        toast(result.error ?? t('common.error'), 'error')
      }
    } catch (err) {
      toast(err instanceof Error ? err.message : t('common.error'), 'error')
    }
  }

  if (isLoading) return <div className="flex items-center justify-center h-32"><Spinner size="md" /></div>

  const lastSyncFormatted = status?.last_sync_at
    ? formatDateTime(status.last_sync_at, { dateStyle: 'short', timeStyle: 'short' })
    : null

  return (
    <div>
      <div className="mb-4">
        <h2 className="text-sm font-semibold text-primary">Microsoft Entra ID (Azure AD) Integration</h2>
        <p className="text-xs text-secondary mt-0.5">
          MFA-Enrollment, Conditional Access, Risky Users und Admin-Rollen täglich als Identity-Evidence erfassen.
          Benötigt App Registration mit Application Permissions (User.Read.All, Policy.Read.All, IdentityRiskyUser.Read.All).
        </p>
      </div>

      {status && (
        <div className="flex items-center gap-3 mb-5 p-3 rounded-lg border border-border bg-surface">
          <div className="flex-1 min-w-0">
            <p className="text-xs text-secondary">
              {lastSyncFormatted ? t('integrations.page.lastSync', { date: lastSyncFormatted }) : t('integrations.page.neverSynced')}
              {status.evidence_count > 0 && ` · ${status.evidence_count} Evidence-Einträge`}
              {status.mfa_enrollment_pct > 0 && ` · MFA ${Math.round(status.mfa_enrollment_pct)}%`}
              {status.risky_user_count > 0 && ` · ${status.risky_user_count} Risky Users`}
            </p>
            {status.last_sync_error && <p className="text-xs text-red-500 truncate mt-0.5">{status.last_sync_error}</p>}
          </div>
          <SyncLastBadge status={status.last_sync_status} lastSyncAt={status.last_sync_at} />
          <button onClick={() => { void handleSync() }} disabled={syncEntraID.isPending || !cfg?.is_configured}
            title="Jetzt synchronisieren"
            className="p-1.5 rounded-md text-secondary hover:text-primary hover:bg-bg transition-colors disabled:opacity-50">
            <RefreshCw className={`w-4 h-4 ${syncEntraID.isPending ? 'animate-spin' : ''}`} />
          </button>
        </div>
      )}

      <form onSubmit={(e) => { void handleSave(e) }} className="space-y-4 max-w-lg">
        <div>
          <label className="block text-xs font-medium text-secondary mb-1">Tenant ID</label>
          <input type="text" value={tenantID} onChange={(e) => { setTenantID(e.target.value); }}
            placeholder="xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx"
            className="w-full border border-border rounded-md px-3 py-2 text-sm bg-bg text-primary placeholder:text-secondary focus:outline-none focus:ring-2 focus:ring-brand/30 font-mono"
            required />
        </div>
        <div>
          <label className="block text-xs font-medium text-secondary mb-1">Client ID (Application ID)</label>
          <input type="text" value={clientID} onChange={(e) => { setClientID(e.target.value); }}
            placeholder="xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx"
            className="w-full border border-border rounded-md px-3 py-2 text-sm bg-bg text-primary placeholder:text-secondary focus:outline-none focus:ring-2 focus:ring-brand/30 font-mono"
            required />
        </div>
        <div>
          <label className="block text-xs font-medium text-secondary mb-1">Client Secret</label>
          <input type="password" value={clientSecret} onChange={(e) => { setClientSecret(e.target.value); }}
            placeholder={cfg?.is_configured && cfg.client_secret === '****' ? '****' : 'App-Secret aus Azure Portal'}
            className="w-full border border-border rounded-md px-3 py-2 text-sm bg-bg text-primary placeholder:text-secondary focus:outline-none focus:ring-2 focus:ring-brand/30"
            required />
          <p className="text-[11px] text-secondary mt-1">Secret wird AES-256-GCM verschlüsselt gespeichert.</p>
        </div>
        <div className="flex gap-2 pt-1">
          <button type="submit" disabled={saveConfig.isPending}
            className="px-4 py-1.5 text-xs font-medium bg-brand text-white rounded-md hover:bg-brand/90 transition-colors disabled:opacity-50">
            {saveConfig.isPending ? t('integrations.page.saving') : t('integrations.page.save')}
          </button>
        </div>
      </form>

      {evidence && evidence.length > 0 && (
        <div className="mt-6">
          <p className="text-xs font-medium text-secondary mb-2">{t('integrations.page.recentEvidence')}</p>
          <RecentEvidenceList items={evidence} />
        </div>
      )}
    </div>
  )
}

// --- Intune tab (S88-7) ---

function IntuneTab() {
  const { t } = useTranslation()
  const { data: cfg, isLoading } = useIntuneConfig()
  const { data: status } = useIntuneStatus()
  const { data: evidence } = useIntuneEvidence()
  const saveConfig = useSaveIntuneConfig()
  const syncIntune = useSyncIntune()
  const { formatDateTime } = useFormatDate()

  const [tenantID, setTenantID] = useState('')
  const [clientID, setClientID] = useState('')
  const [clientSecret, setClientSecret] = useState('')
  const [initialized, setInitialized] = useState(false)

  if (cfg && !initialized) {
    setTenantID(cfg.tenant_id)
    setClientID(cfg.client_id)
    setClientSecret(cfg.client_secret)
    setInitialized(true)
  }

  async function handleSave(e: React.FormEvent) {
    e.preventDefault()
    try {
      await saveConfig.mutateAsync({ tenant_id: tenantID, client_id: clientID, client_secret: clientSecret })
      toast(t('integrations.page.saved'), 'success')
    } catch (err) {
      toast(err instanceof Error ? err.message : t('integrations.page.saveFailed'), 'error')
    }
  }

  async function handleSync() {
    try {
      const result = await syncIntune.mutateAsync()
      if (result.ok) {
        toast(`${t('integrations.page.saved')} — ${result.evidence_created} Evidence-Einträge erstellt`, 'success')
      } else {
        toast(result.error ?? t('common.error'), 'error')
      }
    } catch (err) {
      toast(err instanceof Error ? err.message : t('common.error'), 'error')
    }
  }

  if (isLoading) return <div className="flex items-center justify-center h-32"><Spinner size="md" /></div>

  const lastSyncFormatted = status?.last_sync_at
    ? formatDateTime(status.last_sync_at, { dateStyle: 'short', timeStyle: 'short' })
    : null

  return (
    <div>
      <div className="mb-4">
        <h2 className="text-sm font-semibold text-primary">{t('integrations.intune.title')}</h2>
        <p className="text-xs text-secondary mt-0.5">{t('integrations.intune.description')}</p>
      </div>

      {status && (
        <div className="flex items-center gap-3 mb-5 p-3 rounded-lg border border-border bg-surface">
          <div className="flex-1 min-w-0">
            <p className="text-xs text-secondary">
              {lastSyncFormatted ? t('integrations.page.lastSync', { date: lastSyncFormatted }) : t('integrations.page.neverSynced')}
              {status.evidence_count > 0 && ` · ${status.evidence_count} Evidence-Einträge`}
              {status.device_compliance_pct > 0 && ` · ${t('integrations.intune.compliance')} ${Math.round(status.device_compliance_pct)}%`}
            </p>
            {status.last_sync_error && <p className="text-xs text-red-500 truncate mt-0.5">{status.last_sync_error}</p>}
          </div>
          <SyncLastBadge status={status.last_sync_status} lastSyncAt={status.last_sync_at} />
          <button onClick={() => { void handleSync() }} disabled={syncIntune.isPending || !cfg?.is_configured}
            title={t('integrations.page.syncNow')}
            className="p-1.5 rounded-md text-secondary hover:text-primary hover:bg-bg transition-colors disabled:opacity-50">
            <RefreshCw className={`w-4 h-4 ${syncIntune.isPending ? 'animate-spin' : ''}`} />
          </button>
        </div>
      )}

      <form onSubmit={(e) => { void handleSave(e) }} className="space-y-4 max-w-lg">
        <div>
          <label className="block text-xs font-medium text-secondary mb-1">Tenant ID</label>
          <input type="text" value={tenantID} onChange={(e) => { setTenantID(e.target.value); }}
            placeholder="xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx"
            className="w-full border border-border rounded-md px-3 py-2 text-sm bg-bg text-primary placeholder:text-secondary focus:outline-none focus:ring-2 focus:ring-brand/30 font-mono"
            required />
        </div>
        <div>
          <label className="block text-xs font-medium text-secondary mb-1">Client ID (Application ID)</label>
          <input type="text" value={clientID} onChange={(e) => { setClientID(e.target.value); }}
            placeholder="xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx"
            className="w-full border border-border rounded-md px-3 py-2 text-sm bg-bg text-primary placeholder:text-secondary focus:outline-none focus:ring-2 focus:ring-brand/30 font-mono"
            required />
        </div>
        <div>
          <label className="block text-xs font-medium text-secondary mb-1">Client Secret</label>
          <input type="password" value={clientSecret} onChange={(e) => { setClientSecret(e.target.value); }}
            placeholder={cfg?.is_configured && cfg.client_secret === '****' ? '****' : 'App-Secret aus Azure Portal'}
            className="w-full border border-border rounded-md px-3 py-2 text-sm bg-bg text-primary placeholder:text-secondary focus:outline-none focus:ring-2 focus:ring-brand/30"
            required />
          <p className="text-[11px] text-secondary mt-1">{t('integrations.intune.permissionsHint')}</p>
        </div>
        <div className="flex gap-2 pt-1">
          <button type="submit" disabled={saveConfig.isPending}
            className="px-4 py-1.5 text-xs font-medium bg-brand text-white rounded-md hover:bg-brand/90 transition-colors disabled:opacity-50">
            {saveConfig.isPending ? t('integrations.page.saving') : t('integrations.page.save')}
          </button>
        </div>
      </form>

      {evidence && evidence.length > 0 && (
        <div className="mt-6">
          <p className="text-xs font-medium text-secondary mb-2">{t('integrations.page.recentEvidence')}</p>
          <RecentEvidenceList items={evidence} />
        </div>
      )}
    </div>
  )
}

// --- Keycloak tab ---

function KeycloakTab() {
  const { t } = useTranslation()
  const { data: cfg, isLoading } = useKeycloakConfig()
  const { data: status } = useKeycloakStatus()
  const { data: evidence } = useKeycloakEvidence()
  const saveConfig = useSaveKeycloakConfig()
  const syncKeycloak = useSyncKeycloak()
  const { formatDateTime } = useFormatDate()

  const [keycloakURL, setKeycloakURL] = useState('')
  const [realm, setRealm] = useState('')
  const [clientID, setClientID] = useState('')
  const [clientSecret, setClientSecret] = useState('')
  const [initialized, setInitialized] = useState(false)

  if (cfg && !initialized) {
    setKeycloakURL(cfg.keycloak_url)
    setRealm(cfg.realm)
    setClientID(cfg.client_id)
    setClientSecret(cfg.client_secret)
    setInitialized(true)
  }

  async function handleSave(e: React.FormEvent) {
    e.preventDefault()
    try {
      await saveConfig.mutateAsync({
        keycloak_url: keycloakURL,
        realm,
        client_id: clientID,
        client_secret: clientSecret,
      })
      toast(t('integrations.page.saved'), 'success')
    } catch (err) {
      toast(err instanceof Error ? err.message : t('integrations.page.saveFailed'), 'error')
    }
  }

  async function handleSync() {
    try {
      const result = await syncKeycloak.mutateAsync()
      if (result.ok) {
        toast(`${t('integrations.page.saved')} — ${result.evidence_created} Evidence-Einträge erstellt`, 'success')
      } else {
        toast(result.error ?? t('common.error'), 'error')
      }
    } catch (err) {
      toast(err instanceof Error ? err.message : t('common.error'), 'error')
    }
  }

  if (isLoading) return <div className="flex items-center justify-center h-32"><Spinner size="md" /></div>

  const lastSyncFormatted = status?.last_sync_at
    ? formatDateTime(status.last_sync_at, { dateStyle: 'short', timeStyle: 'short' })
    : null

  return (
    <div>
      <div className="mb-4">
        <h2 className="text-sm font-semibold text-primary">Keycloak REST Collector</h2>
        <p className="text-xs text-secondary mt-0.5">
          MFA-Status, Password-Policy, inaktive Accounts und Admin-Rollen aus Keycloak als Compliance-Evidence erfassen.
          Service Account benötigt &apos;view-users&apos; und &apos;view-realm&apos; Rollen.
        </p>
      </div>

      {status && (
        <div className="flex items-center gap-3 mb-5 p-3 rounded-lg border border-border bg-surface">
          <div className="flex-1 min-w-0">
            <p className="text-xs text-secondary">
              {lastSyncFormatted ? t('integrations.page.lastSync', { date: lastSyncFormatted }) : t('integrations.page.neverSynced')}
              {status.evidence_count > 0 && ` · ${status.evidence_count} Evidence-Einträge`}
              {status.user_count > 0 && ` · ${status.user_count} User`}
              {status.mfa_enrollment_pct > 0 && ` · MFA ${Math.round(status.mfa_enrollment_pct)}%`}
            </p>
            {status.last_sync_error && <p className="text-xs text-red-500 truncate mt-0.5">{status.last_sync_error}</p>}
          </div>
          <SyncLastBadge status={status.last_sync_status} lastSyncAt={status.last_sync_at} />
          <button onClick={() => { void handleSync() }} disabled={syncKeycloak.isPending || !cfg?.is_configured}
            title="Jetzt synchronisieren"
            className="p-1.5 rounded-md text-secondary hover:text-primary hover:bg-bg transition-colors disabled:opacity-50">
            <RefreshCw className={`w-4 h-4 ${syncKeycloak.isPending ? 'animate-spin' : ''}`} />
          </button>
        </div>
      )}

      <form onSubmit={(e) => { void handleSave(e) }} className="space-y-4 max-w-lg">
        <div>
          <label className="block text-xs font-medium text-secondary mb-1">Keycloak URL</label>
          <input type="url" value={keycloakURL} onChange={(e) => { setKeycloakURL(e.target.value); }}
            placeholder="https://keycloak.example.com"
            className="w-full border border-border rounded-md px-3 py-2 text-sm bg-bg text-primary placeholder:text-secondary focus:outline-none focus:ring-2 focus:ring-brand/30 font-mono"
            required />
        </div>
        <div>
          <label className="block text-xs font-medium text-secondary mb-1">Realm</label>
          <input type="text" value={realm} onChange={(e) => { setRealm(e.target.value); }}
            placeholder="master"
            className="w-full border border-border rounded-md px-3 py-2 text-sm bg-bg text-primary placeholder:text-secondary focus:outline-none focus:ring-2 focus:ring-brand/30"
            required />
        </div>
        <div>
          <label className="block text-xs font-medium text-secondary mb-1">Client ID</label>
          <input type="text" value={clientID} onChange={(e) => { setClientID(e.target.value); }}
            placeholder="vakt-collector"
            className="w-full border border-border rounded-md px-3 py-2 text-sm bg-bg text-primary placeholder:text-secondary focus:outline-none focus:ring-2 focus:ring-brand/30"
            required />
        </div>
        <div>
          <label className="block text-xs font-medium text-secondary mb-1">Client Secret</label>
          <input type="password" value={clientSecret} onChange={(e) => { setClientSecret(e.target.value); }}
            placeholder={cfg?.is_configured && cfg.client_secret === '****' ? '****' : 'Service-Account-Secret'}
            className="w-full border border-border rounded-md px-3 py-2 text-sm bg-bg text-primary placeholder:text-secondary focus:outline-none focus:ring-2 focus:ring-brand/30"
            required />
        </div>
        <div className="flex gap-2 pt-1">
          <button type="submit" disabled={saveConfig.isPending}
            className="px-4 py-1.5 text-xs font-medium bg-brand text-white rounded-md hover:bg-brand/90 transition-colors disabled:opacity-50">
            {saveConfig.isPending ? t('integrations.page.saving') : t('integrations.page.save')}
          </button>
        </div>
      </form>

      {evidence && evidence.length > 0 && (
        <div className="mt-6">
          <p className="text-xs font-medium text-secondary mb-2">{t('integrations.page.recentEvidence')}</p>
          <RecentEvidenceList items={evidence} />
        </div>
      )}
    </div>
  )
}

// --- LDAP tab ---

function LDAPTab() {
  const { t } = useTranslation()
  const { data: cfg, isLoading } = useLDAPConfig()
  const { data: status } = useLDAPStatus()
  const { data: evidence } = useLDAPEvidence()
  const saveConfig = useSaveLDAPConfig()
  const syncLDAP = useSyncLDAP()
  const { formatDateTime } = useFormatDate()

  const [host, setHost] = useState('')
  const [port, setPort] = useState('389')
  const [bindDN, setBindDN] = useState('')
  const [bindPassword, setBindPassword] = useState('')
  const [baseDN, setBaseDN] = useState('')
  const [useTLS, setUseTLS] = useState(false)
  const [isAD, setIsAD] = useState(true)
  const [privilegedGroups, setPrivilegedGroups] = useState('Domain Admins,Administrators')
  const [initialized, setInitialized] = useState(false)

  if (cfg && !initialized) {
    setHost(cfg.host)
    setPort(String(cfg.port || 389))
    setBindDN(cfg.bind_dn)
    setBindPassword(cfg.bind_password)
    setBaseDN(cfg.base_dn)
    setUseTLS(cfg.use_tls)
    setIsAD(cfg.is_active_directory)
    setPrivilegedGroups((cfg.privileged_groups ?? []).join(',') || 'Domain Admins,Administrators')
    setInitialized(true)
  }

  async function handleSave(e: React.FormEvent) {
    e.preventDefault()
    try {
      await saveConfig.mutateAsync({
        host,
        port: parseInt(port, 10),
        bind_dn: bindDN,
        bind_password: bindPassword,
        base_dn: baseDN,
        use_tls: useTLS,
        is_active_directory: isAD,
        privileged_groups: privilegedGroups.split(',').map((g) => g.trim()).filter(Boolean),
      })
      toast(t('integrations.page.saved'), 'success')
    } catch (err) {
      toast(err instanceof Error ? err.message : t('integrations.page.saveFailed'), 'error')
    }
  }

  async function handleSync() {
    try {
      const result = await syncLDAP.mutateAsync()
      if (result.ok) {
        toast(`${t('integrations.page.saved')} — ${result.evidence_created} Evidence-Einträge erstellt`, 'success')
      } else {
        toast(result.error ?? t('common.error'), 'error')
      }
    } catch (err) {
      toast(err instanceof Error ? err.message : t('common.error'), 'error')
    }
  }

  if (isLoading) return <div className="flex items-center justify-center h-32"><Spinner size="md" /></div>

  const lastSyncFormatted = status?.last_sync_at
    ? formatDateTime(status.last_sync_at, { dateStyle: 'short', timeStyle: 'short' })
    : null

  return (
    <div>
      <div className="mb-4">
        <h2 className="text-sm font-semibold text-primary">LDAP / Active Directory Collector</h2>
        <p className="text-xs text-secondary mt-0.5">
          Inaktive Accounts, Password-Hygiene und privilegierte Gruppen aus Active Directory oder OpenLDAP als Evidence erfassen.
          Service Account benötigt Read-Zugriff auf das Verzeichnis.
        </p>
      </div>

      {status && (
        <div className="flex items-center gap-3 mb-5 p-3 rounded-lg border border-border bg-surface">
          <div className="flex-1 min-w-0">
            <p className="text-xs text-secondary">
              {lastSyncFormatted ? t('integrations.page.lastSync', { date: lastSyncFormatted }) : t('integrations.page.neverSynced')}
              {status.evidence_count > 0 && ` · ${status.evidence_count} Evidence-Einträge`}
              {status.user_count > 0 && ` · ${status.user_count} aktive User`}
              {status.inactive_count > 0 && ` · ${status.inactive_count} inaktiv`}
              {status.privileged_count > 0 && ` · ${status.privileged_count} privilegiert`}
            </p>
            {status.last_sync_error && <p className="text-xs text-red-500 truncate mt-0.5">{status.last_sync_error}</p>}
          </div>
          <SyncLastBadge status={status.last_sync_status} lastSyncAt={status.last_sync_at} />
          <button onClick={() => { void handleSync() }} disabled={syncLDAP.isPending || !cfg?.is_configured}
            title="Jetzt synchronisieren"
            className="p-1.5 rounded-md text-secondary hover:text-primary hover:bg-bg transition-colors disabled:opacity-50">
            <RefreshCw className={`w-4 h-4 ${syncLDAP.isPending ? 'animate-spin' : ''}`} />
          </button>
        </div>
      )}

      <form onSubmit={(e) => { void handleSave(e) }} className="space-y-4 max-w-lg">
        <div className="grid grid-cols-3 gap-3">
          <div className="col-span-2">
            <label className="block text-xs font-medium text-secondary mb-1">LDAP-Host</label>
            <input type="text" value={host} onChange={(e) => { setHost(e.target.value); }}
              placeholder="dc.example.com"
              className="w-full border border-border rounded-md px-3 py-2 text-sm bg-bg text-primary placeholder:text-secondary focus:outline-none focus:ring-2 focus:ring-brand/30 font-mono"
              required />
          </div>
          <div>
            <label className="block text-xs font-medium text-secondary mb-1">Port</label>
            <input type="number" value={port} onChange={(e) => { setPort(e.target.value); }}
              placeholder="389"
              className="w-full border border-border rounded-md px-3 py-2 text-sm bg-bg text-primary focus:outline-none focus:ring-2 focus:ring-brand/30"
              required />
          </div>
        </div>
        <div>
          <label className="block text-xs font-medium text-secondary mb-1">Bind DN (Service Account)</label>
          <input type="text" value={bindDN} onChange={(e) => { setBindDN(e.target.value); }}
            placeholder="CN=vakt-svc,OU=ServiceAccounts,DC=example,DC=com"
            className="w-full border border-border rounded-md px-3 py-2 text-sm bg-bg text-primary placeholder:text-secondary focus:outline-none focus:ring-2 focus:ring-brand/30 font-mono"
            required />
        </div>
        <div>
          <label className="block text-xs font-medium text-secondary mb-1">Bind Password</label>
          <input type="password" value={bindPassword} onChange={(e) => { setBindPassword(e.target.value); }}
            placeholder={cfg?.is_configured && cfg.bind_password === '****' ? '****' : 'Service-Account-Passwort'}
            className="w-full border border-border rounded-md px-3 py-2 text-sm bg-bg text-primary placeholder:text-secondary focus:outline-none focus:ring-2 focus:ring-brand/30"
            required />
        </div>
        <div>
          <label className="block text-xs font-medium text-secondary mb-1">Base DN</label>
          <input type="text" value={baseDN} onChange={(e) => { setBaseDN(e.target.value); }}
            placeholder="DC=example,DC=com"
            className="w-full border border-border rounded-md px-3 py-2 text-sm bg-bg text-primary placeholder:text-secondary focus:outline-none focus:ring-2 focus:ring-brand/30 font-mono"
            required />
        </div>
        <div>
          <label className="block text-xs font-medium text-secondary mb-1">Privilegierte Gruppen (kommagetrennt)</label>
          <input type="text" value={privilegedGroups} onChange={(e) => { setPrivilegedGroups(e.target.value); }}
            placeholder="Domain Admins,Administrators"
            className="w-full border border-border rounded-md px-3 py-2 text-sm bg-bg text-primary placeholder:text-secondary focus:outline-none focus:ring-2 focus:ring-brand/30" />
        </div>
        <div className="flex flex-col gap-2">
          <div className="flex items-center gap-2">
            <input type="checkbox" id="ldap-tls" checked={useTLS} onChange={(e) => { setUseTLS(e.target.checked); }}
              className="rounded border-border" />
            <label htmlFor="ldap-tls" className="text-xs text-secondary">LDAPS / STARTTLS verwenden (Port 636)</label>
          </div>
          <div className="flex items-center gap-2">
            <input type="checkbox" id="ldap-ad" checked={isAD} onChange={(e) => { setIsAD(e.target.checked); }}
              className="rounded border-border" />
            <label htmlFor="ldap-ad" className="text-xs text-secondary">Active Directory Modus (Windows FILETIME für lastLogon)</label>
          </div>
        </div>
        <div className="flex gap-2 pt-1">
          <button type="submit" disabled={saveConfig.isPending}
            className="px-4 py-1.5 text-xs font-medium bg-brand text-white rounded-md hover:bg-brand/90 transition-colors disabled:opacity-50">
            {saveConfig.isPending ? t('integrations.page.saving') : t('integrations.page.save')}
          </button>
        </div>
      </form>

      {evidence && evidence.length > 0 && (
        <div className="mt-6">
          <p className="text-xs font-medium text-secondary mb-2">{t('integrations.page.recentEvidence')}</p>
          <RecentEvidenceList items={evidence} />
        </div>
      )}
    </div>
  )
}

// --- GitLab tab ---

function GitLabTab() {
  const { t } = useTranslation()
  const { data: cfg, isLoading } = useGitLabConfig()
  const { data: status } = useGitLabStatus()
  const { data: evidence } = useGitLabEvidence()
  const saveConfig = useSaveGitLabConfig()
  const syncGitLab = useSyncGitLab()
  const { formatDateTime } = useFormatDate()

  const [gitlabURL, setGitlabURL] = useState('')
  const [accessToken, setAccessToken] = useState('')
  const [groupID, setGroupID] = useState('')
  const [initialized, setInitialized] = useState(false)

  if (cfg && !initialized) {
    setGitlabURL(cfg.gitlab_url)
    setAccessToken(cfg.access_token)
    setGroupID(cfg.group_id)
    setInitialized(true)
  }

  async function handleSave(e: React.FormEvent) {
    e.preventDefault()
    try {
      await saveConfig.mutateAsync({ gitlab_url: gitlabURL, access_token: accessToken, group_id: groupID })
      toast(t('integrations.page.saved'), 'success')
    } catch (err) {
      toast(err instanceof Error ? err.message : t('integrations.page.saveFailed'), 'error')
    }
  }

  async function handleSync() {
    try {
      const result = await syncGitLab.mutateAsync()
      if (result.ok) {
        toast(`${t('integrations.page.saved')} — ${result.evidence_created} Evidence-Einträge erstellt`, 'success')
      } else {
        toast(result.error ?? t('common.error'), 'error')
      }
    } catch (err) {
      toast(err instanceof Error ? err.message : t('common.error'), 'error')
    }
  }

  if (isLoading) return <div className="flex items-center justify-center h-32"><Spinner size="md" /></div>

  const lastSyncFormatted = status?.last_sync_at
    ? formatDateTime(status.last_sync_at, { dateStyle: 'short', timeStyle: 'short' })
    : null

  return (
    <div>
      <div className="mb-4">
        <h2 className="text-sm font-semibold text-primary">GitLab CI Collector</h2>
        <p className="text-xs text-secondary mt-0.5">
          Branch-Protection, MR-Approval-Regeln und SAST-Präsenz aus GitLab als Compliance-Evidence für ISO 27001 A.8.4/A.8.29/A.8.32 erfassen.
          Access Token wird AES-256-GCM verschlüsselt gespeichert.
        </p>
      </div>

      {status && (
        <div className="flex items-center gap-3 mb-5 p-3 rounded-lg border border-border bg-surface">
          <div className="flex-1 min-w-0">
            <p className="text-xs text-secondary">
              {lastSyncFormatted ? t('integrations.page.lastSync', { date: lastSyncFormatted }) : t('integrations.page.neverSynced')}
              {status.evidence_count > 0 && ` · ${status.evidence_count} Evidence-Einträge`}
              {status.project_count > 0 && ` · ${status.project_count} Projekte`}
              {status.unprotected_branches_count > 0 && (
                <span className="text-amber-600"> · {status.unprotected_branches_count} ungeschützte Branches</span>
              )}
            </p>
            {status.last_sync_error && <p className="text-xs text-red-500 truncate mt-0.5">{status.last_sync_error}</p>}
          </div>
          <SyncLastBadge status={status.last_sync_status} lastSyncAt={status.last_sync_at} />
          <button onClick={() => { void handleSync() }} disabled={syncGitLab.isPending || !cfg?.is_configured}
            title="Jetzt synchronisieren"
            className="p-1.5 rounded-md text-secondary hover:text-primary hover:bg-bg transition-colors disabled:opacity-50">
            <RefreshCw className={`w-4 h-4 ${syncGitLab.isPending ? 'animate-spin' : ''}`} />
          </button>
        </div>
      )}

      <form onSubmit={(e) => { void handleSave(e) }} className="space-y-4 max-w-lg">
        <div>
          <label className="block text-xs font-medium text-secondary mb-1">GitLab URL</label>
          <input type="url" value={gitlabURL} onChange={(e) => { setGitlabURL(e.target.value); }}
            placeholder="https://gitlab.example.com"
            className="w-full border border-border rounded-md px-3 py-2 text-sm bg-bg text-primary placeholder:text-secondary focus:outline-none focus:ring-2 focus:ring-brand/30 font-mono"
            required />
          <p className="text-[11px] text-secondary mt-1">Für GitLab.com: <code>https://gitlab.com</code></p>
        </div>
        <div>
          <label className="block text-xs font-medium text-secondary mb-1">Personal / Group Access Token</label>
          <input type="password" value={accessToken} onChange={(e) => { setAccessToken(e.target.value); }}
            placeholder={cfg?.is_configured && cfg.access_token === '****' ? '****' : 'glpat-...'}
            className="w-full border border-border rounded-md px-3 py-2 text-sm bg-bg text-primary placeholder:text-secondary focus:outline-none focus:ring-2 focus:ring-brand/30 font-mono"
            required />
          <p className="text-[11px] text-secondary mt-1">Benötigte Scopes: <code>read_api</code>, <code>read_repository</code></p>
        </div>
        <div>
          <label className="block text-xs font-medium text-secondary mb-1">Gruppen-ID oder Namespace (optional)</label>
          <input type="text" value={groupID} onChange={(e) => { setGroupID(e.target.value); }}
            placeholder="my-group oder 42"
            className="w-full border border-border rounded-md px-3 py-2 text-sm bg-bg text-primary placeholder:text-secondary focus:outline-none focus:ring-2 focus:ring-brand/30"
          />
          <p className="text-[11px] text-secondary mt-1">Leer lassen um alle zugänglichen Projekte (Membership) zu erfassen.</p>
        </div>
        <div className="flex gap-2 pt-1">
          <button type="submit" disabled={saveConfig.isPending}
            className="px-4 py-1.5 text-xs font-medium bg-brand text-white rounded-md hover:bg-brand/90 transition-colors disabled:opacity-50">
            {saveConfig.isPending ? t('integrations.page.saving') : t('integrations.page.save')}
          </button>
        </div>
      </form>

      {evidence && evidence.length > 0 && (
        <div className="mt-6">
          <p className="text-xs font-medium text-secondary mb-2">{t('integrations.page.recentEvidence')}</p>
          <RecentEvidenceList items={evidence} />
        </div>
      )}
    </div>
  )
}

// --- SonarQube tab ---

function SonarQubeTab() {
  const { t } = useTranslation()
  const { data: cfg, isLoading } = useSonarQubeConfig()
  const { data: status } = useSonarQubeStatus()
  const { data: evidence } = useSonarQubeEvidence()
  const saveConfig = useSaveSonarQubeConfig()
  const syncSonarQube = useSyncSonarQube()
  const { formatDateTime } = useFormatDate()

  const [baseURL, setBaseURL] = useState('')
  const [token, setToken] = useState('')
  const [initialized, setInitialized] = useState(false)

  if (cfg && !initialized) {
    setBaseURL(cfg.base_url)
    setToken(cfg.token)
    setInitialized(true)
  }

  async function handleSave(e: React.FormEvent) {
    e.preventDefault()
    try {
      await saveConfig.mutateAsync({ base_url: baseURL, token })
      toast(t('integrations.page.saved'), 'success')
    } catch (err) {
      toast(err instanceof Error ? err.message : t('integrations.page.saveFailed'), 'error')
    }
  }

  async function handleSync() {
    try {
      const result = await syncSonarQube.mutateAsync()
      if (result.ok) {
        toast(`${t('integrations.page.saved')} — ${result.evidence_created} Evidence-Einträge erstellt`, 'success')
      } else {
        toast(result.error ?? t('common.error'), 'error')
      }
    } catch (err) {
      toast(err instanceof Error ? err.message : t('common.error'), 'error')
    }
  }

  if (isLoading) return <div className="flex items-center justify-center h-32"><Spinner size="md" /></div>

  const lastSyncFormatted = status?.last_sync_at
    ? formatDateTime(status.last_sync_at, { dateStyle: 'short', timeStyle: 'short' })
    : null

  return (
    <div>
      <div className="mb-4">
        <h2 className="text-sm font-semibold text-primary">SonarQube Collector</h2>
        <p className="text-xs text-secondary mt-0.5">
          Quality Gate Status, Security Hotspots und kritische Schwachstellen aus SonarQube / SonarCloud als
          Evidence für ISO 27001 A.8.8/A.8.29 erfassen. Token wird AES-256-GCM verschlüsselt gespeichert.
        </p>
      </div>

      {status && (
        <div className="flex items-center gap-3 mb-5 p-3 rounded-lg border border-border bg-surface">
          <div className="flex-1 min-w-0">
            <p className="text-xs text-secondary">
              {lastSyncFormatted ? t('integrations.page.lastSync', { date: lastSyncFormatted }) : t('integrations.page.neverSynced')}
              {status.evidence_count > 0 && ` · ${status.evidence_count} Evidence-Einträge`}
              {status.project_count > 0 && ` · ${status.project_count} Projekte`}
              {status.quality_gate_failed_count > 0 && (
                <span className="text-red-600"> · {status.quality_gate_failed_count} Quality Gates fehlgeschlagen</span>
              )}
              {status.hotspot_count > 0 && ` · ${status.hotspot_count} Hotspots`}
            </p>
            {status.last_sync_error && <p className="text-xs text-red-500 truncate mt-0.5">{status.last_sync_error}</p>}
          </div>
          <SyncLastBadge status={status.last_sync_status} lastSyncAt={status.last_sync_at} />
          <button onClick={() => { void handleSync() }} disabled={syncSonarQube.isPending || !cfg?.is_configured}
            title="Jetzt synchronisieren"
            className="p-1.5 rounded-md text-secondary hover:text-primary hover:bg-bg transition-colors disabled:opacity-50">
            <RefreshCw className={`w-4 h-4 ${syncSonarQube.isPending ? 'animate-spin' : ''}`} />
          </button>
        </div>
      )}

      <form onSubmit={(e) => { void handleSave(e) }} className="space-y-4 max-w-lg">
        <div>
          <label className="block text-xs font-medium text-secondary mb-1">SonarQube Base URL</label>
          <input type="url" value={baseURL} onChange={(e) => { setBaseURL(e.target.value); }}
            placeholder="https://sonarqube.example.com"
            className="w-full border border-border rounded-md px-3 py-2 text-sm bg-bg text-primary placeholder:text-secondary focus:outline-none focus:ring-2 focus:ring-brand/30 font-mono"
            required />
          <p className="text-[11px] text-secondary mt-1">Für SonarCloud: <code>https://sonarcloud.io</code></p>
        </div>
        <div>
          <label className="block text-xs font-medium text-secondary mb-1">User Token</label>
          <input type="password" value={token} onChange={(e) => { setToken(e.target.value); }}
            placeholder={cfg?.is_configured && cfg.token === '****' ? '****' : 'squ_...'}
            className="w-full border border-border rounded-md px-3 py-2 text-sm bg-bg text-primary placeholder:text-secondary focus:outline-none focus:ring-2 focus:ring-brand/30 font-mono"
            required />
          <p className="text-[11px] text-secondary mt-1">
            Token aus SonarQube → My Account → Security → Generate Token. Typ: <code>User Token</code>.
          </p>
        </div>
        <div className="flex gap-2 pt-1">
          <button type="submit" disabled={saveConfig.isPending}
            className="px-4 py-1.5 text-xs font-medium bg-brand text-white rounded-md hover:bg-brand/90 transition-colors disabled:opacity-50">
            {saveConfig.isPending ? t('integrations.page.saving') : t('integrations.page.save')}
          </button>
        </div>
      </form>

      {evidence && evidence.length > 0 && (
        <div className="mt-6">
          <p className="text-xs font-medium text-secondary mb-2">{t('integrations.page.recentEvidence')}</p>
          <RecentEvidenceList items={evidence} />
        </div>
      )}
    </div>
  )
}

// --- Personio tab ---

function PersonioTab() {
  const { t } = useTranslation()
  const { data: cfg, isLoading } = usePersonioConfig()
  const { data: status } = usePersonioStatus()
  const saveConfig = useSavePersonioConfig()
  const { formatDateTime } = useFormatDate()

  const [webhookSecret, setWebhookSecret] = useState('')
  const [initialized, setInitialized] = useState(false)
  const [copied, setCopied] = useState(false)

  if (cfg && !initialized) {
    setWebhookSecret(cfg.webhook_secret)
    setInitialized(true)
  }

  async function handleSave(e: React.FormEvent) {
    e.preventDefault()
    try {
      await saveConfig.mutateAsync({ webhook_secret: webhookSecret })
      toast(t('integrations.page.saved'), 'success')
    } catch (err) {
      toast(err instanceof Error ? err.message : t('integrations.page.saveFailed'), 'error')
    }
  }

  function handleCopyURL() {
    if (!status?.webhook_url) return
    const fullURL = window.location.origin + status.webhook_url
    void navigator.clipboard.writeText(fullURL).then(() => {
      setCopied(true)
      setTimeout(() => { setCopied(false) }, 2000)
    })
  }

  if (isLoading) return <div className="flex items-center justify-center h-32"><Spinner size="md" /></div>

  const lastWebhookFormatted = status?.last_sync_at
    ? formatDateTime(status.last_sync_at, { dateStyle: 'short', timeStyle: 'short' })
    : null

  const fullWebhookURL = status?.webhook_url
    ? window.location.origin + status.webhook_url
    : null

  return (
    <div>
      <div className="mb-4">
        <h2 className="text-sm font-semibold text-primary">Personio HRIS Webhook</h2>
        <p className="text-xs text-secondary mt-0.5">
          Automatisches Offboarding-Checklisten-Trigger bei <code>employee.departed</code>-Events aus Personio.
          Vakt empfängt den Webhook und startet die HR-Offboarding-Checkliste. Kein Pull aus Personio — Push-only.
        </p>
      </div>

      {/* DSGVO notice */}
      <div className="flex items-start gap-3 p-3 mb-5 rounded-lg border border-amber-200 bg-amber-50">
        <ShieldAlert className="w-4 h-4 text-amber-600 shrink-0 mt-0.5" />
        <p className="text-xs text-amber-800">
          <strong>DSGVO-Hinweis:</strong> Vakt speichert aus dem Personio-Webhook ausschließlich die
          numerische <code>employee_id</code> und das <code>departure_date</code>. Keine Namen, E-Mail-Adressen
          oder andere personenbezogenen Daten werden persistiert (Art. 5 Abs. 1 lit. c DSGVO — Datensparsamkeit).
        </p>
      </div>

      {/* Status row */}
      {status && (
        <div className="flex items-center gap-3 mb-5 p-3 rounded-lg border border-border bg-surface">
          <div className="flex-1 min-w-0">
            <p className="text-xs text-secondary">
              {lastWebhookFormatted ? t('integrations.page.lastSync', { date: lastWebhookFormatted }) : t('integrations.page.neverSynced')}
              {status.offboardings_triggered > 0 && ` · ${status.offboardings_triggered} Offboardings ausgelöst`}
              {status.offboardings_completed_on_time > 0 && ` · ${status.offboardings_completed_on_time} fristgerecht`}
            </p>
          </div>
          {status.webhook_configured ? (
            <span className="inline-flex items-center gap-1 text-xs font-medium text-emerald-700 bg-emerald-50 border border-emerald-200 rounded-full px-2 py-0.5">
              <CheckCircle2 className="w-3 h-3" /> {t('integrations.page.saved')}
            </span>
          ) : (
            <span className="inline-flex items-center gap-1 text-xs font-medium text-amber-700 bg-amber-50 border border-amber-200 rounded-full px-2 py-0.5">
              <AlertCircle className="w-3 h-3" /> {t('integrations.page.syncPending')}
            </span>
          )}
        </div>
      )}

      {/* Webhook URL display */}
      {fullWebhookURL && (
        <div className="mb-5 max-w-lg">
          <p className="text-xs font-medium text-secondary mb-1">Webhook URL (in Personio eintragen)</p>
          <div className="flex items-center gap-2 bg-bg border border-border rounded-md px-3 py-2">
            <code className="text-xs text-primary flex-1 break-all">{fullWebhookURL}</code>
            <button onClick={handleCopyURL} title="URL kopieren"
              className="p-1 rounded text-secondary hover:text-primary transition-colors shrink-0">
              {copied ? <CheckCircle2 className="w-4 h-4 text-emerald-500" /> : <Copy className="w-4 h-4" />}
            </button>
          </div>
          <p className="text-[11px] text-secondary mt-1">
            In Personio unter Einstellungen → Integrationen → Webhooks → Add Webhook eintragen.
            Methode: <code>POST</code>, Event: <code>employee.departed</code>.
          </p>
        </div>
      )}

      <form onSubmit={(e) => { void handleSave(e) }} className="space-y-4 max-w-lg">
        <div>
          <label className="block text-xs font-medium text-secondary mb-1">Webhook Secret</label>
          <input type="password" value={webhookSecret} onChange={(e) => { setWebhookSecret(e.target.value); }}
            placeholder={cfg?.is_configured && cfg.webhook_secret === '****' ? '****' : 'Webhook Secret aus Personio'}
            className="w-full border border-border rounded-md px-3 py-2 text-sm bg-bg text-primary placeholder:text-secondary focus:outline-none focus:ring-2 focus:ring-brand/30 font-mono"
            required />
          <p className="text-[11px] text-secondary mt-1">
            Das Secret wird von Personio zum Signieren der Webhook-Payloads verwendet (HMAC-SHA256, Header: <code>X-Personio-Signature</code>).
            Secret wird AES-256-GCM verschlüsselt gespeichert.
          </p>
        </div>
        <div className="flex gap-2 pt-1">
          <button type="submit" disabled={saveConfig.isPending}
            className="px-4 py-1.5 text-xs font-medium bg-brand text-white rounded-md hover:bg-brand/90 transition-colors disabled:opacity-50">
            {saveConfig.isPending ? t('integrations.page.saving') : t('integrations.page.save')}
          </button>
        </div>
      </form>
    </div>
  )
}

// --- Main page ---

type Tab = 'github' | 'aws' | 'azure' | 'hetzner' | 'ionos' | 'wazuh' | 'prometheus' | 'entra-id' | 'intune' | 'keycloak' | 'ldap' | 'gitlab' | 'sonarqube' | 'personio'

export default function IntegrationsPage() {
  const { t } = useTranslation()
  const [activeTab, setActiveTab] = useState<Tab>('github')

  const tabs: { id: Tab; label: string; icon: React.ReactNode }[] = [
    { id: 'github', label: 'GitHub', icon: <GitBranch className="w-4 h-4" /> },
    { id: 'aws', label: 'AWS', icon: <Cloud className="w-4 h-4" /> },
    { id: 'azure', label: 'Azure', icon: <Cloud className="w-4 h-4" /> },
    { id: 'hetzner', label: 'Hetzner', icon: <Cloud className="w-4 h-4" /> },
    { id: 'ionos', label: 'IONOS', icon: <Cloud className="w-4 h-4" /> },
    { id: 'wazuh', label: 'Wazuh', icon: <Cloud className="w-4 h-4" /> },
    { id: 'prometheus', label: 'Prometheus', icon: <Cloud className="w-4 h-4" /> },
    { id: 'entra-id', label: 'Entra ID', icon: <ShieldAlert className="w-4 h-4" /> },
    { id: 'intune', label: 'Intune', icon: <ShieldAlert className="w-4 h-4" /> },
    { id: 'keycloak', label: 'Keycloak', icon: <ShieldAlert className="w-4 h-4" /> },
    { id: 'ldap', label: 'LDAP/AD', icon: <ShieldAlert className="w-4 h-4" /> },
    { id: 'gitlab', label: 'GitLab', icon: <GitBranch className="w-4 h-4" /> },
    { id: 'sonarqube', label: 'SonarQube', icon: <ShieldAlert className="w-4 h-4" /> },
    { id: 'personio', label: 'Personio', icon: <ShieldAlert className="w-4 h-4" /> },
  ]

  return (
    <div className="p-6 max-w-4xl mx-auto">
      {/* Page header */}
      <div className="flex items-center gap-2.5 mb-6">
        <Plug className="w-5 h-5 text-brand" />
        <div>
          <h1 className="text-lg font-semibold text-primary">{t('integrations.page.title')}</h1>
          <p className="text-xs text-secondary">{t('integrations.page.description')}</p>
        </div>
      </div>

      {/* Tabs */}
      <div className="flex flex-wrap gap-1 border-b border-border mb-6">
        {tabs.map((tab) => (
          <button
            key={tab.id}
            onClick={() => { setActiveTab(tab.id); }}
            className={`flex items-center gap-1.5 px-4 py-2 text-sm font-medium border-b-2 transition-colors -mb-px ${
              activeTab === tab.id
                ? 'border-brand text-brand'
                : 'border-transparent text-secondary hover:text-primary'
            }`}
          >
            {tab.icon}
            {tab.label}
          </button>
        ))}
      </div>

      {/* Tab content */}
      {activeTab === 'github' && <GitHubTab />}
      {activeTab === 'aws' && <AWSTab />}
      {activeTab === 'azure' && <AzureTab />}
      {activeTab === 'hetzner' && <HetznerTab />}
      {activeTab === 'ionos' && <IONOSTab />}
      {activeTab === 'wazuh' && <WazuhTab />}
      {activeTab === 'prometheus' && <PrometheusTab />}
      {activeTab === 'entra-id' && <EntraIDTab />}
      {activeTab === 'intune' && <IntuneTab />}
      {activeTab === 'keycloak' && <KeycloakTab />}
      {activeTab === 'ldap' && <LDAPTab />}
      {activeTab === 'gitlab' && <GitLabTab />}
      {activeTab === 'sonarqube' && <SonarQubeTab />}
      {activeTab === 'personio' && <PersonioTab />}

      {/* No third-party integrations notice */}
      <div className="mt-6">
        <NoThirdPartyInfoBox />
      </div>
    </div>
  )
}
