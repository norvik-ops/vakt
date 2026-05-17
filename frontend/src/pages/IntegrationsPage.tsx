import { useState } from 'react'
import { Plug, GitBranch, RefreshCw, Trash2, ChevronDown, ChevronUp, CheckCircle2, XCircle, AlertCircle, Plus, ExternalLink } from 'lucide-react'
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
  useJiraConfig,
  useSaveJiraConfig,
  useTestJiraConnection,
} from '../hooks/useJira'
import { toast } from '../shared/hooks/useToast'

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
              <p className="text-[11px] text-red-500 mt-0.5">{String(cr.details.error)}</p>
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

  const lastSync = integration.last_synced_at
    ? new Date(integration.last_synced_at).toLocaleString('de-DE', { dateStyle: 'short', timeStyle: 'short' })
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
            onClick={() => setExpanded((v) => !v)}
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
        onSuccess: () => onClose(),
        onError: (err) => setError(err.message),
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
              onChange={(e) => setOwner(e.target.value)}
              placeholder="z.B. my-org"
              className="w-full border border-border rounded-md px-3 py-2 text-sm bg-bg text-primary placeholder:text-secondary focus:outline-none focus:ring-2 focus:ring-brand/30"
            />
          </div>
          <div>
            <label className="block text-xs font-medium text-secondary mb-1">Repository-Name</label>
            <input
              type="text"
              value={repo}
              onChange={(e) => setRepo(e.target.value)}
              placeholder="z.B. my-repo"
              className="w-full border border-border rounded-md px-3 py-2 text-sm bg-bg text-primary placeholder:text-secondary focus:outline-none focus:ring-2 focus:ring-brand/30"
            />
          </div>
          <div>
            <label className="block text-xs font-medium text-secondary mb-1">Personal Access Token</label>
            <input
              type="password"
              value={token}
              onChange={(e) => setToken(e.target.value)}
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
        <div className="w-5 h-5 border-2 border-brand border-t-transparent rounded-full animate-spin" />
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
          onClick={() => setShowDialog(true)}
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

      {showDialog && <AddIntegrationDialog onClose={() => setShowDialog(false)} />}
    </div>
  )
}

// --- Jira tab ---

function JiraTab() {
  const { data: cfg, isLoading } = useJiraConfig()
  const saveConfig = useSaveJiraConfig()
  const testConnection = useTestJiraConnection()

  const [jiraUrl, setJiraUrl] = useState('')
  const [projectKey, setProjectKey] = useState('')
  const [userEmail, setUserEmail] = useState('')
  const [apiToken, setApiToken] = useState('')
  const [initialized, setInitialized] = useState(false)
  const [testResult, setTestResult] = useState<{ success: boolean; message: string } | null>(null)

  // Pre-fill fields once config loads
  if (cfg && !initialized) {
    setJiraUrl(cfg.jira_url)
    setProjectKey(cfg.project_key)
    setUserEmail(cfg.user_email)
    setApiToken(cfg.api_token) // "****" mask or empty
    setInitialized(true)
  }

  async function handleSave(e: React.FormEvent) {
    e.preventDefault()
    try {
      await saveConfig.mutateAsync({ jira_url: jiraUrl, project_key: projectKey, user_email: userEmail, api_token: apiToken })
      toast('Jira-Konfiguration gespeichert', 'success')
    } catch (err) {
      toast(err instanceof Error ? err.message : 'Speichern fehlgeschlagen', 'error')
    }
  }

  async function handleTest() {
    setTestResult(null)
    try {
      const result = await testConnection.mutateAsync()
      if (result.success) {
        setTestResult({ success: true, message: result.display_name ? `Verbunden als ${result.display_name}` : 'Verbunden' })
      } else {
        setTestResult({ success: false, message: result.error ?? 'Verbindung fehlgeschlagen' })
      }
    } catch (err) {
      setTestResult({ success: false, message: err instanceof Error ? err.message : 'Verbindung fehlgeschlagen' })
    }
  }

  if (isLoading) {
    return (
      <div className="flex items-center justify-center h-32">
        <div className="w-5 h-5 border-2 border-brand border-t-transparent rounded-full animate-spin" />
      </div>
    )
  }

  return (
    <div>
      <div className="mb-4">
        <h2 className="text-sm font-semibold text-primary">Jira-Integration</h2>
        <p className="text-xs text-secondary mt-0.5">
          Sicherheitsbefunde direkt als Jira-Tickets erstellen. API-Token wird AES-256-GCM verschlüsselt gespeichert.
        </p>
      </div>

      <form onSubmit={(e) => { void handleSave(e) }} className="space-y-4 max-w-lg">
        <div>
          <label className="block text-xs font-medium text-secondary mb-1">Jira-URL</label>
          <input
            type="url"
            value={jiraUrl}
            onChange={(e) => setJiraUrl(e.target.value)}
            placeholder="https://yourorg.atlassian.net"
            className="w-full border border-border rounded-md px-3 py-2 text-sm bg-bg text-primary placeholder:text-secondary focus:outline-none focus:ring-2 focus:ring-brand/30"
            required
          />
        </div>
        <div>
          <label className="block text-xs font-medium text-secondary mb-1">Projekt-Schlüssel</label>
          <input
            type="text"
            value={projectKey}
            onChange={(e) => setProjectKey(e.target.value)}
            placeholder="z.B. SEC"
            className="w-full border border-border rounded-md px-3 py-2 text-sm bg-bg text-primary placeholder:text-secondary focus:outline-none focus:ring-2 focus:ring-brand/30"
            required
          />
        </div>
        <div>
          <label className="block text-xs font-medium text-secondary mb-1">Benutzer-E-Mail</label>
          <input
            type="email"
            value={userEmail}
            onChange={(e) => setUserEmail(e.target.value)}
            placeholder="admin@yourorg.com"
            className="w-full border border-border rounded-md px-3 py-2 text-sm bg-bg text-primary placeholder:text-secondary focus:outline-none focus:ring-2 focus:ring-brand/30"
            required
          />
        </div>
        <div>
          <label className="block text-xs font-medium text-secondary mb-1">API-Token</label>
          <input
            type="password"
            value={apiToken}
            onChange={(e) => setApiToken(e.target.value)}
            placeholder={cfg?.is_configured ? '****' : 'API-Token eingeben'}
            className="w-full border border-border rounded-md px-3 py-2 text-sm bg-bg text-primary placeholder:text-secondary focus:outline-none focus:ring-2 focus:ring-brand/30"
            required
          />
          <p className="text-[11px] text-secondary mt-1">
            Erstelle ein API-Token unter{' '}
            <a href="https://id.atlassian.com/manage-profile/security/api-tokens" target="_blank" rel="noreferrer" className="underline hover:text-primary">
              id.atlassian.com
            </a>.
          </p>
        </div>

        {testResult && (
          <div className={`flex items-center gap-2 text-sm px-3 py-2 rounded-md border ${testResult.success ? 'text-emerald-700 bg-emerald-50 border-emerald-200' : 'text-red-700 bg-red-50 border-red-200'}`}>
            {testResult.success ? <CheckCircle2 className="w-4 h-4 shrink-0" /> : <XCircle className="w-4 h-4 shrink-0" />}
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
    </div>
  )
}

// --- Main page ---

type Tab = 'github' | 'jira'

export default function IntegrationsPage() {
  const [activeTab, setActiveTab] = useState<Tab>('github')

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
        <button
          onClick={() => setActiveTab('github')}
          className={`flex items-center gap-1.5 px-4 py-2 text-sm font-medium border-b-2 transition-colors -mb-px ${
            activeTab === 'github'
              ? 'border-brand text-brand'
              : 'border-transparent text-secondary hover:text-primary'
          }`}
        >
          <GitBranch className="w-4 h-4" />
          GitHub
        </button>
        <button
          onClick={() => setActiveTab('jira')}
          className={`flex items-center gap-1.5 px-4 py-2 text-sm font-medium border-b-2 transition-colors -mb-px ${
            activeTab === 'jira'
              ? 'border-brand text-brand'
              : 'border-transparent text-secondary hover:text-primary'
          }`}
        >
          <ExternalLink className="w-4 h-4" />
          Jira
        </button>
      </div>

      {/* Tab content */}
      {activeTab === 'github' && <GitHubTab />}
      {activeTab === 'jira' && <JiraTab />}
    </div>
  )
}
