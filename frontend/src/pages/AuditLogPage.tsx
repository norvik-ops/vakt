import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { useSavedFilters } from '../shared/hooks/useSavedFilters'
import { ShieldAlert, Download, RefreshCw } from 'lucide-react'
import { PageHeader } from '../shared/components/PageHeader'
import { Button } from '../components/ui/button'
import { Input } from '../components/ui/input'
import { Badge } from '../components/ui/badge'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '../components/ui/select'
import {
  Table,
  TableHeader,
  TableBody,
  TableRow,
  TableHead,
  TableCell,
} from '../components/ui/table'
import { Skeleton } from '../components/ui/skeleton'
import { useAuthStore } from '../shared/stores/auth'
import { ErrorState } from '../shared/components/ErrorState'
import { useAuditLog, type AuditLogEntry } from '../hooks/useAuditLog'
import { useFormatDate } from '../shared/hooks/useFormatDate'

// ─── Constants ────────────────────────────────────────────────────────────────

const ACTIONS = ['create', 'update', 'delete', 'approve', 'export'] as const
const PAGE_SIZE = 25

// ─── Helpers ──────────────────────────────────────────────────────────────────

/** Convert a local date string (YYYY-MM-DD) to RFC3339 at start-of-day UTC. */
function dateToRFC3339Start(date: string): string {
  return `${date}T00:00:00Z`
}

/** Convert a local date string (YYYY-MM-DD) to RFC3339 at end-of-day UTC. */
function dateToRFC3339End(date: string): string {
  return `${date}T23:59:59Z`
}

function ActionBadge({ action }: { action: string }) {
  const { t } = useTranslation()
  switch (action) {
    case 'create':
      return <Badge className="bg-green-100 text-green-800 dark:bg-green-900/40 dark:text-green-300 border-0 text-[11px]">{t('auditLog.actionCreated')}</Badge>
    case 'update':
      return <Badge className="bg-blue-100 text-blue-800 dark:bg-blue-900/40 dark:text-blue-300 border-0 text-[11px]">{t('auditLog.actionUpdated')}</Badge>
    case 'delete':
      return <Badge className="bg-red-100 text-red-800 dark:bg-red-900/40 dark:text-red-300 border-0 text-[11px]">{t('auditLog.actionDeleted')}</Badge>
    case 'approve':
      return <Badge className="bg-purple-100 text-purple-800 dark:bg-purple-900/40 dark:text-purple-300 border-0 text-[11px]">{t('auditLog.actionApproved')}</Badge>
    case 'export':
      return <Badge className="bg-amber-100 text-amber-800 dark:bg-amber-900/40 dark:text-amber-300 border-0 text-[11px]">{t('auditLog.actionExported')}</Badge>
    default:
      return <Badge variant="secondary" className="text-[11px]">{action}</Badge>
  }
}

function detailsText(entry: AuditLogEntry): string {
  const parts: string[] = []
  if (entry.resource_name) parts.push(entry.resource_name)
  if (entry.resource_type) parts.push(`(${entry.resource_type})`)
  if (entry.details && Object.keys(entry.details).length > 0) {
    const extras = Object.entries(entry.details)
      .map(([k, v]) => `${k}: ${v}`)
      .join(', ')
    parts.push(`— ${extras}`)
  }
  return parts.join(' ') || entry.resource_id || '–'
}

function escapeCsvCell(value: string): string {
  if (value.includes(',') || value.includes('"') || value.includes('\n')) {
    return `"${value.replace(/"/g, '""')}"`
  }
  return value
}

function exportCsv(entries: AuditLogEntry[], formatDateTime: (v: string) => string, t: (key: string) => string) {
  const headers = [t('auditLog.csvHeaderTimestamp'), t('auditLog.csvHeaderUser'), t('auditLog.csvHeaderAction'), t('auditLog.csvHeaderResource'), t('auditLog.csvHeaderDetails')]
  const rows = entries.map((e) => [
    formatDateTime(e.created_at),
    e.user_email ?? e.user_id ?? 'System',
    e.action,
    e.resource_type,
    detailsText(e),
    e.ip_address ?? '',
  ].map(escapeCsvCell).join(','))

  const csv = [headers.join(','), ...rows].join('\n')
  const bom = '\ufeff'
  const blob = new Blob([bom + csv], { type: 'text/csv;charset=utf-8;' })
  const url = URL.createObjectURL(blob)
  const a = document.createElement('a')
  a.href = url
  a.download = `audit-log-${new Date().toISOString().slice(0, 10)}.csv`
  a.click()
  URL.revokeObjectURL(url)
}

// ─── Skeleton rows ────────────────────────────────────────────────────────────

function SkeletonRows() {
  return (
    <>
      {Array.from({ length: 8 }).map((_, i) => (
        <TableRow key={i}>
          <TableCell><Skeleton className="h-4 w-36" /></TableCell>
          <TableCell><Skeleton className="h-4 w-40" /></TableCell>
          <TableCell><Skeleton className="h-5 w-20" /></TableCell>
          <TableCell><Skeleton className="h-4 w-48" /></TableCell>
        </TableRow>
      ))}
    </>
  )
}

// ─── Page ─────────────────────────────────────────────────────────────────────

export default function AuditLogPage() {
  const { t } = useTranslation()
  const { user } = useAuthStore()
  const isAdminOrOwner = user?.roles.includes('Admin') ?? false
  const { formatDateTime } = useFormatDate()

  // Filter state — persisted across page reloads via localStorage
  const [filters, setFilters] = useSavedFilters('audit-log', {
    fromDate: '',
    toDate: '',
    userFilter: '',
    actionFilter: 'all',
  })
  const { fromDate, toDate, userFilter, actionFilter } = filters
  const [page, setPage] = useState(0)

  const offset = page * PAGE_SIZE

  const { data, isLoading, isError, refetch, isFetching } = useAuditLog({
    limit:     PAGE_SIZE,
    offset,
    from:      fromDate ? dateToRFC3339Start(fromDate) : undefined,
    to:        toDate   ? dateToRFC3339End(toDate)     : undefined,
    userEmail: userFilter.trim() || undefined,
    action:    actionFilter !== 'all' ? actionFilter : undefined,
  })

  const entries = data?.entries ?? []
  const total   = data?.total   ?? 0
  const totalPages = Math.max(1, Math.ceil(total / PAGE_SIZE))

  function resetFilters() {
    setFilters({ fromDate: '', toDate: '', userFilter: '', actionFilter: 'all' })
    setPage(0)
  }

  function handleFilterChange() {
    setPage(0)
  }

  if (!isAdminOrOwner) {
    return (
      <div className="flex flex-col h-full">
        <PageHeader
          title={t('auditLog.title')}
          description={t('auditLog.description')}
        />
        <div className="flex-1 flex items-center justify-center">
          <div className="text-center space-y-2">
            <ShieldAlert className="w-10 h-10 text-destructive mx-auto" />
            <p className="text-sm font-medium text-primary">{t('auditLog.noAccess')}</p>
            <p className="text-xs text-secondary">{t('auditLog.noAccessDesc')}</p>
          </div>
        </div>
      </div>
    )
  }

  return (
    <div className="flex flex-col h-full">
      <PageHeader
        title={t('auditLog.title')}
        description={t('auditLog.description')}
        actions={
          <div className="flex items-center gap-2">
            <Button
              variant="outline"
              size="sm"
              onClick={() => void refetch()}
              disabled={isFetching}
              className="h-8 text-xs"
            >
              <RefreshCw className={`w-3.5 h-3.5 mr-1.5 ${isFetching ? 'animate-spin' : ''}`} />
              {t('auditLog.refresh')}
            </Button>
            <Button
              variant="outline"
              size="sm"
              onClick={() => { if (entries.length > 0) exportCsv(entries, formatDateTime, t) }}
              disabled={entries.length === 0}
              className="h-8 text-xs"
            >
              <Download className="w-3.5 h-3.5 mr-1.5" />
              {t('auditLog.exportCsv')}
            </Button>
          </div>
        }
      />

      {/* Filters */}
      <div className="px-6 pt-4 pb-2">
        <div className="flex flex-wrap items-end gap-3">
          <div className="space-y-1">
            <p className="text-[11px] text-secondary font-medium">{t('auditLog.filterFrom')}</p>
            <Input
              type="date"
              value={fromDate}
              onChange={(e) => { setFilters((f) => ({ ...f, fromDate: e.target.value })); handleFilterChange() }}
              className="h-8 text-xs w-36"
            />
          </div>
          <div className="space-y-1">
            <p className="text-[11px] text-secondary font-medium">{t('auditLog.filterTo')}</p>
            <Input
              type="date"
              value={toDate}
              onChange={(e) => { setFilters((f) => ({ ...f, toDate: e.target.value })); handleFilterChange() }}
              className="h-8 text-xs w-36"
            />
          </div>
          <div className="space-y-1">
            <p className="text-[11px] text-secondary font-medium">{t('auditLog.filterUser')}</p>
            <Input
              type="text"
              placeholder={t('auditLog.filterEmailPlaceholder')}
              value={userFilter}
              onChange={(e) => { setFilters((f) => ({ ...f, userFilter: e.target.value })); handleFilterChange() }}
              className="h-8 text-xs w-48"
            />
          </div>
          <div className="space-y-1">
            <p className="text-[11px] text-secondary font-medium">{t('auditLog.filterAction')}</p>
            <Select
              value={actionFilter}
              onValueChange={(v) => { setFilters((f) => ({ ...f, actionFilter: v })); handleFilterChange() }}
            >
              <SelectTrigger className="h-8 text-xs w-36">
                <SelectValue placeholder={t('auditLog.allActions')} />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value="all">{t('auditLog.allActions')}</SelectItem>
                {ACTIONS.map((a) => (
                  <SelectItem key={a} value={a}>{a}</SelectItem>
                ))}
              </SelectContent>
            </Select>
          </div>
          {(fromDate || toDate || userFilter || actionFilter !== 'all') && (
            <Button
              variant="ghost"
              size="sm"
              className="h-8 text-xs self-end"
              onClick={resetFilters}
            >
              {t('auditLog.resetFilters')}
            </Button>
          )}
        </div>
      </div>

      {/* Table */}
      <div className="flex-1 px-6 pb-6 overflow-auto">
        {isError ? (
          <ErrorState
            message={t('auditLog.loadError')}
            onRetry={() => void refetch()}
          />
        ) : (
          <>
            <div className="rounded-lg border border-border overflow-x-auto">
              <Table>
                <TableHeader>
                  <TableRow className="bg-surface">
                    <TableHead className="text-[11px] font-semibold text-secondary w-44">{t('auditLog.colTimestamp')}</TableHead>
                    <TableHead className="text-[11px] font-semibold text-secondary">{t('auditLog.colUser')}</TableHead>
                    <TableHead className="text-[11px] font-semibold text-secondary w-28">{t('auditLog.colAction')}</TableHead>
                    <TableHead className="text-[11px] font-semibold text-secondary">{t('auditLog.colDetails')}</TableHead>
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {isLoading ? (
                    <SkeletonRows />
                  ) : entries.length === 0 ? (
                    <TableRow>
                      <TableCell colSpan={4} className="text-center text-sm text-secondary py-12">
                        {t('auditLog.noEvents')}
                      </TableCell>
                    </TableRow>
                  ) : (
                    entries.map((entry) => (
                      <TableRow key={entry.id} className="hover:bg-surface/50">
                        <TableCell className="text-[12px] text-secondary whitespace-nowrap font-mono">
                          {formatDateTime(entry.created_at)}
                        </TableCell>
                        <TableCell className="text-[12px] text-primary max-w-[200px] truncate">
                          {entry.user_email ?? entry.user_id ?? <span className="text-secondary italic">System</span>}
                        </TableCell>
                        <TableCell>
                          <ActionBadge action={entry.action} />
                        </TableCell>
                        <TableCell className="text-[12px] text-primary max-w-[360px]">
                          <span className="truncate block" title={detailsText(entry)}>
                            {detailsText(entry)}
                          </span>
                          {entry.ip_address && (
                            <span className="text-[10px] text-secondary">{entry.ip_address}</span>
                          )}
                        </TableCell>
                      </TableRow>
                    ))
                  )}
                </TableBody>
              </Table>
            </div>

            {/* Pagination */}
            {!isLoading && total > 0 && (
              <div className="flex items-center justify-between mt-3">
                <p className="text-[11px] text-secondary">
                  {t('auditLog.totalEvents', { count: total })}
                  {totalPages > 1 && ` · ${t('auditLog.pageOf', { page: page + 1, total: totalPages })}`}
                </p>
                {totalPages > 1 && (
                  <div className="flex items-center gap-1">
                    <Button
                      variant="outline"
                      size="sm"
                      className="h-7 text-xs px-2"
                      disabled={page === 0}
                      onClick={() => { setPage((p) => Math.max(0, p - 1)); }}
                    >
                      {t('auditLog.prevPage')}
                    </Button>
                    <Button
                      variant="outline"
                      size="sm"
                      className="h-7 text-xs px-2"
                      disabled={page >= totalPages - 1}
                      onClick={() => { setPage((p) => Math.min(totalPages - 1, p + 1)); }}
                    >
                      {t('auditLog.nextPage')}
                    </Button>
                  </div>
                )}
              </div>
            )}
          </>
        )}
      </div>
    </div>
  )
}
