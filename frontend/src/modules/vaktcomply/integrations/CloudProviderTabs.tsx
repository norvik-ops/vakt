import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { RefreshCw, CheckCircle2, XCircle } from 'lucide-react'
import { Spinner } from '../../../components/Spinner'
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
} from '../../../hooks/useCloud'
import { toast } from '../../../shared/hooks/useToast'
import { useFormatDate } from '../../../shared/hooks/useFormatDate'
import { SyncLastBadge, RecentEvidenceList } from './shared'

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

export function AWSTab() {
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
          <label className="block text-xs font-medium text-secondary mb-1">{t('vaktcomply.cloudProviderTabs.region')}</label>
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

export function AzureTab() {
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

export function HetznerTab() {
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

export function IONOSTab() {
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
              <label className="block text-xs font-medium text-secondary mb-1">{t('vaktcomply.cloudProviderTabs.username')}</label>
              <input type="text" value={username} onChange={(e) => { setUsername(e.target.value); }}
                placeholder="IONOS-Benutzername"
                className="w-full border border-border rounded-md px-3 py-2 text-sm bg-bg text-primary placeholder:text-secondary focus:outline-none focus:ring-2 focus:ring-brand/30"
                required />
            </div>
            <div>
              <label className="block text-xs font-medium text-secondary mb-1">{t('vaktcomply.cloudProviderTabs.password')}</label>
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

export function WazuhTab() {
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
          <label className="block text-xs font-medium text-secondary mb-1">{t('vaktcomply.cloudProviderTabs.username')}</label>
          <input type="text" value={username} onChange={(e) => { setUsername(e.target.value); }}
            placeholder="wazuh-readonly"
            className="w-full border border-border rounded-md px-3 py-2 text-sm bg-bg text-primary placeholder:text-secondary focus:outline-none focus:ring-2 focus:ring-brand/30"
            required />
        </div>
        <div>
          <label className="block text-xs font-medium text-secondary mb-1">{t('vaktcomply.cloudProviderTabs.password')}</label>
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

export function PrometheusTab() {
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
