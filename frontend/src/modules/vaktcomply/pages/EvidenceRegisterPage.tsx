import { useMemo, useState } from 'react'
import { Link } from 'react-router-dom'
import { useTranslation } from 'react-i18next'
import { FileText, Paperclip } from 'lucide-react'
import { PageHeader } from '../../../shared/components/PageHeader'
import { EmptyState } from '../../../shared/components/EmptyState'
import { ErrorState } from '../../../shared/components/ErrorState'
import { Card, CardContent } from '../../../components/ui/card'
import { Badge } from '../../../components/ui/badge'
import { Input } from '../../../components/ui/input'
import { Skeleton } from '../../../components/ui/skeleton'
import { Table, TableHeader, TableBody, TableRow, TableHead, TableCell } from '../../../components/ui/table'
import { EvidenceExpiryBadge } from '../components/EvidenceExpiryBadge'
import { EvidenceTabs } from '../components/EvidenceTabs'
import { useEvidenceRegister, type EvidenceListItem } from '../hooks/useEvidenceRegister'
import { frameworkDisplayName } from '../utils/frameworkCatalogue'

const EXPIRY_WINDOW_DAYS = 30

type TypeKey = 'manual' | 'document' | 'automated'

/** Same cut as the „Typ" column on the control page: a manual entry with a file is a document. */
function typeKey(ev: Pick<EvidenceListItem, 'source' | 'has_file'>): TypeKey {
  if (ev.source === 'manual') return ev.has_file ? 'document' : 'manual'
  return 'automated'
}

/** Expired or expiring within the window — the audit-prep question „what runs out?". */
function isExpiring(ev: Pick<EvidenceListItem, 'expires_at'>, now: number): boolean {
  if (!ev.expires_at) return false
  return new Date(ev.expires_at).getTime() - now <= EXPIRY_WINDOW_DAYS * 86_400_000
}

const STATUS_VARIANT: Partial<Record<string, 'secondary' | 'success' | 'destructive'>> = {
  pending: 'secondary',
  approved: 'success',
  rejected: 'destructive',
  expired: 'secondary',
}

const selectClass =
  'h-9 rounded-md border border-border bg-surface px-3 text-sm text-primary focus:outline-none focus:ring-1 focus:ring-brand'

export default function EvidenceRegisterPage() {
  const { t } = useTranslation()
  const { data: items = [], isLoading, isError, refetch } = useEvidenceRegister()
  const [search, setSearch] = useState('')
  const [framework, setFramework] = useState('')
  const [status, setStatus] = useState('')
  const [withoutFile, setWithoutFile] = useState(false)
  const [expiring, setExpiring] = useState(false)

  const statusLabel: Record<string, string> = {
    pending: t('vaktcomply.controlDetailPage.evidenceStatusPending'),
    approved: t('vaktcomply.controlDetailPage.evidenceStatusApproved'),
    rejected: t('vaktcomply.controlDetailPage.evidenceStatusRejected'),
    expired: t('vaktcomply.controlDetailPage.evidenceStatusExpired'),
  }
  const typeLabel: Record<TypeKey, string> = {
    manual: t('vaktcomply.controlDetailPage.evidenceTypeManualLabel'),
    document: t('vaktcomply.controlDetailPage.evidenceTypeDocumentLabel'),
    automated: t('vaktcomply.controlDetailPage.evidenceTypeAutomatedLabel'),
  }

  const frameworks = useMemo(() => {
    const byId = new Map<string, string>()
    for (const ev of items) byId.set(ev.framework_id, frameworkDisplayName(ev.framework_name))
    return [...byId.entries()].sort((a, b) => a[1].localeCompare(b[1]))
  }, [items])

  const visible = useMemo(() => {
    const now = Date.now()
    const q = search.trim().toLowerCase()
    return items.filter((ev) => {
      if (framework && ev.framework_id !== framework) return false
      if (status && ev.status !== status) return false
      if (withoutFile && ev.has_file) return false
      if (expiring && !isExpiring(ev, now)) return false
      if (q && !`${ev.title} ${ev.control_ref} ${ev.control_title}`.toLowerCase().includes(q)) return false
      return true
    })
  }, [items, search, framework, status, withoutFile, expiring])

  const withoutFileCount = items.filter((ev) => !ev.has_file).length

  return (
    <div className="flex flex-col h-full">
      <PageHeader
        title={t('vaktcomply.evidenceRegister.title')}
        description={t('vaktcomply.evidenceRegister.description')}
      />
      <div className="flex-1 p-6 space-y-4">
        <EvidenceTabs />

        {isLoading && (
          <div className="space-y-2" aria-label={t('vaktcomply.evidenceRegister.loading')}>
            {Array.from({ length: 4 }).map((_, i) => <Skeleton key={i} className="h-10 w-full rounded-lg" />)}
          </div>
        )}
        {isError && (
          <ErrorState message={t('vaktcomply.evidenceRegister.loadError')} onRetry={() => void refetch()} />
        )}

        {!isLoading && !isError && items.length === 0 && (
          <EmptyState
            icon={FileText}
            title={t('vaktcomply.evidenceRegister.emptyTitle')}
            description={t('vaktcomply.evidenceRegister.emptyDescription')}
          />
        )}

        {!isLoading && !isError && items.length > 0 && (
          <>
            <div className="flex flex-wrap items-center gap-3">
              <Input
                type="search"
                value={search}
                onChange={(e) => { setSearch(e.target.value) }}
                placeholder={t('vaktcomply.evidenceRegister.filterSearch')}
                aria-label={t('vaktcomply.evidenceRegister.filterSearch')}
                className="h-9 w-full sm:w-64"
              />
              <select
                className={selectClass}
                value={framework}
                onChange={(e) => { setFramework(e.target.value) }}
                aria-label={t('vaktcomply.evidenceRegister.filterFramework')}
              >
                <option value="">{t('vaktcomply.evidenceRegister.filterAllFrameworks')}</option>
                {frameworks.map(([id, name]) => <option key={id} value={id}>{name}</option>)}
              </select>
              <select
                className={selectClass}
                value={status}
                onChange={(e) => { setStatus(e.target.value) }}
                aria-label={t('vaktcomply.evidenceRegister.filterStatus')}
              >
                <option value="">{t('vaktcomply.evidenceRegister.filterAllStatuses')}</option>
                {Object.entries(statusLabel).map(([key, label]) => <option key={key} value={key}>{label}</option>)}
              </select>
              <label className="inline-flex items-center gap-2 text-sm text-secondary">
                <input type="checkbox" className="rounded" checked={withoutFile} onChange={(e) => { setWithoutFile(e.target.checked) }} />
                {t('vaktcomply.evidenceRegister.filterNoFile', { count: withoutFileCount })}
              </label>
              <label className="inline-flex items-center gap-2 text-sm text-secondary">
                <input type="checkbox" className="rounded" checked={expiring} onChange={(e) => { setExpiring(e.target.checked) }} />
                {t('vaktcomply.evidenceRegister.filterExpiring', { days: EXPIRY_WINDOW_DAYS })}
              </label>
            </div>

            <p className="text-xs text-secondary" aria-live="polite">
              {t('vaktcomply.evidenceRegister.count', { shown: visible.length, total: items.length })}
            </p>

            <Card>
              <CardContent className="p-0">
                {visible.length === 0 ? (
                  <p className="py-10 text-center text-sm text-secondary">{t('vaktcomply.evidenceRegister.noMatch')}</p>
                ) : (
                  <div className="overflow-x-auto">
                    <Table>
                      <TableHeader>
                        <TableRow>
                          <TableHead>{t('vaktcomply.evidenceRegister.colTitle')}</TableHead>
                          <TableHead>{t('vaktcomply.evidenceRegister.colControl')}</TableHead>
                          <TableHead>{t('vaktcomply.evidenceRegister.colFramework')}</TableHead>
                          <TableHead>{t('vaktcomply.evidenceRegister.colType')}</TableHead>
                          <TableHead>{t('vaktcomply.evidenceRegister.colStatus')}</TableHead>
                          <TableHead>{t('vaktcomply.evidenceRegister.colExpiry')}</TableHead>
                          <TableHead>{t('vaktcomply.evidenceRegister.colFile')}</TableHead>
                        </TableRow>
                      </TableHeader>
                      <TableBody>
                        {visible.map((ev) => (
                          <TableRow key={ev.id}>
                            <TableCell className="font-medium">{ev.title}</TableCell>
                            <TableCell>
                              <Link
                                to={`/vaktcomply/controls/${ev.control_id}?frameworkId=${ev.framework_id}`}
                                className="text-brand hover:underline"
                                title={ev.control_title}
                              >
                                <span className="font-mono text-xs mr-1.5">{ev.control_ref}</span>
                                <span className="text-sm">{ev.control_title}</span>
                              </Link>
                            </TableCell>
                            <TableCell className="text-sm text-secondary">{frameworkDisplayName(ev.framework_name)}</TableCell>
                            <TableCell className="text-sm text-secondary" title={ev.source}>{typeLabel[typeKey(ev)]}</TableCell>
                            <TableCell>
                              <Badge variant={STATUS_VARIANT[ev.status] ?? 'secondary'}>
                                {statusLabel[ev.status] ?? ev.status}
                              </Badge>
                            </TableCell>
                            <TableCell>
                              {ev.expires_at ? <EvidenceExpiryBadge expiresAt={ev.expires_at} /> : <span className="text-secondary">—</span>}
                            </TableCell>
                            <TableCell className="text-sm">
                              {ev.has_file ? (
                                <span className="inline-flex items-center gap-1 text-primary">
                                  <Paperclip className="w-3.5 h-3.5" aria-hidden="true" />
                                  {t('vaktcomply.evidenceRegister.fileYes')}
                                  {ev.file_count > 1 && <span className="text-secondary">({ev.file_count})</span>}
                                </span>
                              ) : (
                                <span className="text-amber-600 dark:text-amber-400">{t('vaktcomply.evidenceRegister.fileNo')}</span>
                              )}
                            </TableCell>
                          </TableRow>
                        ))}
                      </TableBody>
                    </Table>
                  </div>
                )}
              </CardContent>
            </Card>
          </>
        )}
      </div>
    </div>
  )
}
