import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { RefreshCw } from 'lucide-react'
import { Spinner } from '../../../components/Spinner'
import {
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
} from '../../../hooks/useCloud'
import { toast } from '../../../shared/hooks/useToast'
import { useFormatDate } from '../../../shared/hooks/useFormatDate'
import { SyncLastBadge, RecentEvidenceList } from './shared'

// --- GitLab tab ---

export function GitLabTab() {
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

export function SonarQubeTab() {
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
