import { useState } from 'react'
import { Plug, GitBranch, RefreshCw, Trash2, ChevronDown, ChevronUp, CheckCircle2, XCircle, AlertCircle, Plus, Cloud, ShieldAlert } from 'lucide-react'
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
  type CloudEvidenceItem,
} from '../hooks/useCloud'
import { toast } from '../shared/hooks/useToast'
import { useFormatDate } from '../shared/hooks/useFormatDate'

// --- Status badge ---

function SyncStatusBadge({ status }: { status: string }) {
  if (status === 'ok') {
    return (
      <span className="inline-flex items-center gap-1 text-xs font-medium text-emerald-700 bg-emerald-50 border border-emerald-200 rounded-full px-2 py-0.5">
        <CheckCircle2 className="w-3 h-3" /> Synchronisiert
      </span>
    )
  }
  if (status === 'error') {
    return (
      <span className="inline-flex items-center gap-1 text-xs font-medium text-red-700 bg-red-50 border border-red-200 rounded-full px-2 py-0.5">
        <XCircle className="w-3 h-3" /> Fehler
      </span>
    )
  }
  return (
    <span className="inline-flex items-center gap-1 text-xs font-medium text-amber-700 bg-amber-50 border border-amber-200 rounded-full px-2 py-0.5">
      <AlertCircle className="w-3 h-3" /> Ausstehend
    </span>
  )
}

function CheckStatusBadge({ status }: { status: string }) {
  if (status === 'pass') {
    return (
      <span className="inline-flex items-center gap-1 text-xs font-medium text-emerald-700 bg-emerald-50 border border-emerald-200 rounded-full px-2 py-0.5">
        <CheckCircle2 className="w-3 h-3" /> Pass
      </span>
    )
  }
  if (status === 'fail') {
    return (
      <span className="inline-flex items-center gap-1 text-xs font-medium text-red-700 bg-red-50 border border-red-200 rounded-full px-2 py-0.5">
        <XCircle className="w-3 h-3" /> Fail
      </span>
    )
  }
  return (
    <span className="inline-flex items-center gap-1 text-xs font-medium text-secondary bg-surface border border-border rounded-full px-2 py-0.5">
      <AlertCircle className="w-3 h-3" /> Unbekannt
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
  const { data: checks, isLoading } = useGitHubCheckResults(integrationId)

  if (isLoading) {
    return <p className="text-xs text-secondary py-2">Lade Check-Ergebnisse…</p>
  }

  if (!checks || checks.length === 0) {
    return <p className="text-xs text-secondary py-2">Noch keine Check-Ergebnisse. Synchronisierung starten.</p>
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
  const [expanded, setExpanded] = useState(false)
  const deleteIntegration = useDeleteGitHubIntegration()
  const syncIntegration = useSyncGitHubIntegration()
  const { formatDateTime } = useFormatDate()

  const lastSync = integration.last_synced_at
    ? formatDateTime(integration.last_synced_at, { dateStyle: 'short', timeStyle: 'short' })
    : 'Noch nicht synchronisiert'

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
          <p className="text-xs text-secondary">Letzter Sync: {lastSync}</p>
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
  const addIntegration = useAddGitHubIntegration()
  const [owner, setOwner] = useState('')
  const [repo, setRepo] = useState('')
  const [token, setToken] = useState('')
  const [error, setError] = useState('')

  function handleSubmit(e: React.FormEvent) {
    e.preventDefault()
    setError('')
    if (!owner.trim() || !repo.trim() || !token.trim()) {
      setError('Alle Felder sind erforderlich.')
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
        <h2 className="text-base font-semibold text-primary mb-4">Repository verbinden</h2>
        <form onSubmit={handleSubmit} className="space-y-4">
          <div>
            <label className="block text-xs font-medium text-secondary mb-1">Repository-Owner</label>
            <input
              type="text"
              value={owner}
              onChange={(e) => { setOwner(e.target.value); }}
              placeholder="z.B. my-org"
              className="w-full border border-border rounded-md px-3 py-2 text-sm bg-bg text-primary placeholder:text-secondary focus:outline-none focus:ring-2 focus:ring-brand/30"
            />
          </div>
          <div>
            <label className="block text-xs font-medium text-secondary mb-1">Repository-Name</label>
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
              Abbrechen
            </button>
            <button
              type="submit"
              disabled={addIntegration.isPending}
              className="px-4 py-2 text-sm rounded-md bg-brand text-white hover:bg-brand/90 transition-colors disabled:opacity-50"
            >
              {addIntegration.isPending ? 'Wird gespeichert…' : 'Verbinden'}
            </button>
          </div>
        </form>
      </div>
    </div>
  )
}

// --- GitHub tab ---

function GitHubTab() {
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
    return <p className="text-sm text-red-500">Fehler beim Laden der Integrationen: {error.message}</p>
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
          Repository verbinden
        </button>
      </div>

      {integrations && integrations.length === 0 ? (
        <div className="border border-dashed border-border rounded-lg p-8 text-center">
          <GitBranch className="w-8 h-8 text-secondary mx-auto mb-2" />
          <p className="text-sm font-medium text-primary">Noch keine Repositories verbunden</p>
          <p className="text-xs text-secondary mt-1">
            Verbinde ein GitHub-Repository, um automatisch Compliance-Evidence zu sammeln.
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
  return (
    <div className="flex items-start gap-4 p-5 rounded-xl border border-border bg-surface max-w-lg">
      <ShieldAlert className="w-6 h-6 text-amber-500 shrink-0 mt-0.5" />
      <div>
        <p className="text-sm font-semibold text-primary mb-1">Keine Drittanbieter-Integrationen</p>
        <p className="text-xs text-secondary leading-relaxed">
          Aus Datenschutzgründen (DSGVO Art. 28) verzichtet Vakt auf Integrationen,
          die Sicherheitsdaten an externe SaaS-Dienste übertragen. Nutze Webhooks für
          eigene Automatisierungen.
        </p>
      </div>
    </div>
  )
}

// --- Cloud sync status badge ---

function SyncLastBadge({ status, lastSyncAt }: { status: string | null; lastSyncAt: string | null }) {
  if (!lastSyncAt) {
    return (
      <span className="inline-flex items-center gap-1 text-xs font-medium text-amber-700 bg-amber-50 border border-amber-200 rounded-full px-2 py-0.5">
        <AlertCircle className="w-3 h-3" /> Nie synchronisiert
      </span>
    )
  }
  if (status === 'success') {
    return (
      <span className="inline-flex items-center gap-1 text-xs font-medium text-emerald-700 bg-emerald-50 border border-emerald-200 rounded-full px-2 py-0.5">
        <CheckCircle2 className="w-3 h-3" /> Erfolgreich
      </span>
    )
  }
  return (
    <span className="inline-flex items-center gap-1 text-xs font-medium text-red-700 bg-red-50 border border-red-200 rounded-full px-2 py-0.5">
      <XCircle className="w-3 h-3" /> Fehler
    </span>
  )
}

// --- Recent evidence list ---

function RecentEvidenceList({ items }: { items: CloudEvidenceItem[] }) {
  const { formatDateTime } = useFormatDate()
  if (items.length === 0) {
    return <p className="text-xs text-secondary py-2">Noch keine Evidence gesammelt. Synchronisierung starten.</p>
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
      toast('AWS-Konfiguration gespeichert', 'success')
    } catch (err) {
      toast(err instanceof Error ? err.message : 'Speichern fehlgeschlagen', 'error')
    }
  }

  async function handleTest() {
    setTestResult(null)
    try {
      const result = await testConnection.mutateAsync()
      setTestResult({ ok: result.ok, message: result.ok ? 'Verbindung erfolgreich' : (result.error ?? 'Verbindung fehlgeschlagen') })
    } catch (err) {
      setTestResult({ ok: false, message: err instanceof Error ? err.message : 'Verbindung fehlgeschlagen' })
    }
  }

  async function handleSync() {
    try {
      const result = await syncAWS.mutateAsync()
      if (result.ok) {
        toast(`Synchronisierung abgeschlossen — ${result.evidence_created} Evidence-Einträge erstellt`, 'success')
      } else {
        toast(result.error ?? 'Synchronisierung fehlgeschlagen', 'error')
      }
    } catch (err) {
      toast(err instanceof Error ? err.message : 'Synchronisierung fehlgeschlagen', 'error')
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
              {lastSyncFormatted ? `Letzter Sync: ${lastSyncFormatted}` : 'Noch nie synchronisiert'}
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
            {testConnection.isPending ? 'Teste…' : 'Verbindung testen'}
          </button>
          <button
            type="submit"
            disabled={saveConfig.isPending}
            className="px-4 py-1.5 text-xs font-medium bg-brand text-white rounded-md hover:bg-brand/90 transition-colors disabled:opacity-50"
          >
            {saveConfig.isPending ? 'Wird gespeichert…' : 'Speichern'}
          </button>
        </div>
      </form>

      {/* Recent evidence */}
      {evidence && evidence.length > 0 && (
        <div className="mt-6">
          <p className="text-xs font-medium text-secondary mb-2">Zuletzt gesammelte Evidence</p>
          <RecentEvidenceList items={evidence} />
        </div>
      )}
    </div>
  )
}

// --- Azure tab ---

function AzureTab() {
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
      toast('Azure-Konfiguration gespeichert', 'success')
    } catch (err) {
      toast(err instanceof Error ? err.message : 'Speichern fehlgeschlagen', 'error')
    }
  }

  async function handleTest() {
    setTestResult(null)
    try {
      const result = await testConnection.mutateAsync()
      setTestResult({ ok: result.ok, message: result.ok ? 'Verbindung erfolgreich' : (result.error ?? 'Verbindung fehlgeschlagen') })
    } catch (err) {
      setTestResult({ ok: false, message: err instanceof Error ? err.message : 'Verbindung fehlgeschlagen' })
    }
  }

  async function handleSync() {
    try {
      const result = await syncAzure.mutateAsync()
      if (result.ok) {
        toast(`Synchronisierung abgeschlossen — ${result.evidence_created} Evidence-Einträge erstellt`, 'success')
      } else {
        toast(result.error ?? 'Synchronisierung fehlgeschlagen', 'error')
      }
    } catch (err) {
      toast(err instanceof Error ? err.message : 'Synchronisierung fehlgeschlagen', 'error')
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
              {lastSyncFormatted ? `Letzter Sync: ${lastSyncFormatted}` : 'Noch nie synchronisiert'}
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
            {testConnection.isPending ? 'Teste…' : 'Verbindung testen'}
          </button>
          <button
            type="submit"
            disabled={saveConfig.isPending}
            className="px-4 py-1.5 text-xs font-medium bg-brand text-white rounded-md hover:bg-brand/90 transition-colors disabled:opacity-50"
          >
            {saveConfig.isPending ? 'Wird gespeichert…' : 'Speichern'}
          </button>
        </div>
      </form>

      {/* Recent evidence */}
      {evidence && evidence.length > 0 && (
        <div className="mt-6">
          <p className="text-xs font-medium text-secondary mb-2">Zuletzt gesammelte Evidence</p>
          <RecentEvidenceList items={evidence} />
        </div>
      )}
    </div>
  )
}

// --- Main page ---

type Tab = 'github' | 'aws' | 'azure'

export default function IntegrationsPage() {
  const [activeTab, setActiveTab] = useState<Tab>('github')

  const tabs: { id: Tab; label: string; icon: React.ReactNode }[] = [
    { id: 'github', label: 'GitHub', icon: <GitBranch className="w-4 h-4" /> },
    { id: 'aws', label: 'AWS', icon: <Cloud className="w-4 h-4" /> },
    { id: 'azure', label: 'Azure', icon: <Cloud className="w-4 h-4" /> },
  ]

  return (
    <div className="p-6 max-w-4xl mx-auto">
      {/* Page header */}
      <div className="flex items-center gap-2.5 mb-6">
        <Plug className="w-5 h-5 text-brand" />
        <div>
          <h1 className="text-lg font-semibold text-primary">Integrationen</h1>
          <p className="text-xs text-secondary">Externe Dienste verbinden und Compliance-Evidence automatisch sammeln.</p>
        </div>
      </div>

      {/* Tabs */}
      <div className="flex gap-1 border-b border-border mb-6">
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

      {/* No third-party integrations notice */}
      <div className="mt-6">
        <NoThirdPartyInfoBox />
      </div>
    </div>
  )
}
