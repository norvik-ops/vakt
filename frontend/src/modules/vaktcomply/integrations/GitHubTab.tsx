import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { GitBranch, RefreshCw, Trash2, ChevronDown, ChevronUp, Plus } from 'lucide-react'
import { Spinner } from '../../../components/Spinner'
import {
  useGitHubIntegrations,
  useAddGitHubIntegration,
  useDeleteGitHubIntegration,
  useSyncGitHubIntegration,
  type GitHubIntegration,
} from '../../../hooks/useGitHub'
import { useFormatDate } from '../../../shared/hooks/useFormatDate'
import { SyncStatusBadge, CheckResultsPanel } from './shared'

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

export function GitHubTab() {
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
