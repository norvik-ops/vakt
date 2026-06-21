import { useTranslation } from 'react-i18next'
import { CheckCircle2, XCircle, AlertCircle } from 'lucide-react'
import { useGitHubCheckResults, type GitHubCheckResult } from '../../../hooks/useGitHub'
import { useFormatDate } from '../../../shared/hooks/useFormatDate'
import { type CloudEvidenceItem } from '../../../hooks/useCloud'

// --- Status badge ---

export function SyncStatusBadge({ status }: { status: string }) {
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

export function CheckStatusBadge({ status }: { status: string }) {
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

export function checkTypeLabel(type: string): string {
  const labels: Record<string, string> = {
    branch_protection: 'Branch Protection',
    pr_review_required: 'PR Review erforderlich',
    dependency_alerts: 'Dependency Alerts',
    secret_scanning: 'Secret Scanning',
  }
  return labels[type] ?? type
}

// --- Check results panel ---

export function CheckResultsPanel({ integrationId }: { integrationId: string }) {
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

// --- Cloud sync status badge ---

export function SyncLastBadge({ status, lastSyncAt }: { status: string | null; lastSyncAt: string | null }) {
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

export function RecentEvidenceList({ items }: { items: CloudEvidenceItem[] }) {
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
