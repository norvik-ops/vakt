import { useState, useRef, useCallback } from 'react'
import { useSavedFilters } from '../../../shared/hooks/useSavedFilters'
import { Spinner } from '../../../components/Spinner'
import { useNavigate } from 'react-router-dom'
import { useTranslation } from 'react-i18next'
import { Download, AlertTriangle, Upload, Trash2, RefreshCw, FileDown, X } from 'lucide-react'
import { useDeferredDelete } from '../../../shared/hooks/useDeferredDelete'
import { PageHeader } from '../../../shared/components/PageHeader'
import { EmptyState } from '../../../shared/components/EmptyState'
import { Pagination } from '../../../shared/components/Pagination'
import { BulkActionBar } from '../../../shared/components/BulkActionBar'
import { SortableHeader } from '../../../shared/components/SortableHeader'
import { useSortableTable } from '../../../shared/hooks/useSortableTable'
import { Button } from '../../../components/ui/button'
import { Badge } from '../../../components/ui/badge'
import { Select, SelectTrigger, SelectContent, SelectItem, SelectValue } from '../../../components/ui/select'
import { Table, TableHeader, TableBody, TableRow, TableHead, TableCell } from '../../../components/ui/table'
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
  DialogFooter,
} from '../../../components/ui/dialog'
import { useFindings, exportFindingsCsv, useBulkUpdateFindings, useDeleteFinding, usePatchFinding } from '../hooks/useFindings'
import { apiFetch } from '../../../api/client'
import { useQueryClient } from '@tanstack/react-query'
import type { Finding } from '../types'
import { cn } from '../../../lib/utils'
import { findingSeverityClass, findingSeverityOrder } from '../../../lib/statusMapping'
import { ImportFindingsDialog } from '../components/ImportFindingsDialog'
import { CSVImportDialog } from '../../../shared/components/CSVImportDialog'
import { MobileCard } from '../../../shared/components/MobileCard'
import { useKeyboardNav } from '../../../shared/hooks/useKeyboardNav'
import { useTableKeyboard } from '../../../shared/hooks/useTableKeyboard'
import { toast } from '../../../shared/hooks/useToast'
import { Skeleton } from '../../../components/ui/skeleton'
import { exportToCSV } from '../../../lib/csv'
import { ErrorState } from '../../../shared/components/ErrorState'
import { useFormatDate } from '../../../shared/hooks/useFormatDate'
import { useNoScannersAvailable } from '../hooks/useScannerStatus'

// ── Inline editable status cell ──────────────────────────────────────────────

function InlineStatusCell({ finding }: { finding: Finding }) {
  const { t } = useTranslation()
  const [editing, setEditing] = useState(false)
  const [displayStatus, setDisplayStatus] = useState<Finding['status']>(finding.status)
  const patch = usePatchFinding(finding.id)

  function handleChange(value: string) {
    const newStatus = value as Finding['status']
    const prev = displayStatus
    setDisplayStatus(newStatus)
    setEditing(false)
    patch.mutate(
      { status: newStatus },
      {
        onError: () => {
          setDisplayStatus(prev)
          toast(t('vaktscan.findingsPage.saveFailed'), 'error')
        },
      },
    )
  }

  if (editing) {
    return (
      <Select
        defaultOpen
        value={displayStatus}
        onValueChange={handleChange}
        onOpenChange={(open) => { if (!open) setEditing(false) }}
      >
        <SelectTrigger
          className="h-7 text-xs w-36"
          onClick={(e) => { e.stopPropagation(); }}
        >
          <SelectValue />
        </SelectTrigger>
        <SelectContent onClick={(e) => { e.stopPropagation(); }}>
          <SelectItem value="open">{t('vaktscan.status.open')}</SelectItem>
          <SelectItem value="in_progress">{t('vaktscan.status.in_progress')}</SelectItem>
          <SelectItem value="accepted_risk">{t('vaktscan.status.accepted_risk')}</SelectItem>
          <SelectItem value="false_positive">{t('vaktscan.status.false_positive')}</SelectItem>
          <SelectItem value="resolved">{t('vaktscan.status.resolved')}</SelectItem>
        </SelectContent>
      </Select>
    )
  }

  return (
    <span
      className="text-sm text-secondary capitalize cursor-pointer hover:text-primary hover:underline underline-offset-2 transition-colors"
      onClick={(e) => { e.stopPropagation(); setEditing(true) }}
      title="Klicken zum Bearbeiten"
    >
      {displayStatus.replace(/_/g, ' ')}
    </span>
  )
}

// ── Inline editable severity cell ────────────────────────────────────────────

function InlineSeverityCell({ finding }: { finding: Finding }) {
  const { t } = useTranslation()
  const [editing, setEditing] = useState(false)
  const [saving, setSaving] = useState(false)
  const [displaySeverity, setDisplaySeverity] = useState<Finding['severity']>(finding.severity)
  const queryClient = useQueryClient()

  async function handleChange(value: string) {
    const newSeverity = value as Finding['severity']
    setEditing(false)
    setSaving(true)
    const prev = displaySeverity
    setDisplaySeverity(newSeverity)
    try {
      await apiFetch<Finding>(`/vaktscan/findings/${finding.id}`, {
        method: 'PATCH',
        body: JSON.stringify({ severity: newSeverity }),
      })
      void queryClient.invalidateQueries({ queryKey: ['vaktscan', 'findings'] })
    } catch {
      setDisplaySeverity(prev)
      toast(t('vaktscan.findingsPage.saveFailed'), 'error')
    } finally {
      setSaving(false)
    }
  }

  const badgeClass = severityClass[displaySeverity]

  if (saving) {
    return (
      <span className="flex items-center gap-1.5">
        <Spinner size="sm" className="w-3.5 h-3.5" />
        <Badge className={cn('capitalize', badgeClass)}>{displaySeverity}</Badge>
      </span>
    )
  }

  if (editing) {
    return (
      <Select
        defaultOpen
        value={displaySeverity}
        onValueChange={(v) => { void handleChange(v) }}
        onOpenChange={(open) => { if (!open) setEditing(false) }}
      >
        <SelectTrigger
          className="h-7 text-xs w-28"
          onClick={(e) => { e.stopPropagation(); }}
        >
          <SelectValue />
        </SelectTrigger>
        <SelectContent onClick={(e) => { e.stopPropagation(); }}>
          <SelectItem value="critical">{t('vaktscan.severity.critical')}</SelectItem>
          <SelectItem value="high">{t('vaktscan.severity.high')}</SelectItem>
          <SelectItem value="medium">{t('vaktscan.severity.medium')}</SelectItem>
          <SelectItem value="low">{t('vaktscan.severity.low')}</SelectItem>
          <SelectItem value="info">{t('vaktscan.severity.info')}</SelectItem>
        </SelectContent>
      </Select>
    )
  }

  return (
    <Badge
      className={cn('capitalize cursor-pointer', badgeClass)}
      onClick={(e) => { e.stopPropagation(); setEditing(true) }}
      title="Klicken zum Bearbeiten"
    >
      {displaySeverity}
    </Badge>
  )
}

const severityClass = findingSeverityClass
const SEVERITY_ORDER = findingSeverityOrder

// Augment Finding with a numeric severity_order field for sorting
type SortableFinding = Finding & { severity_order: number }

export default function FindingsPage() {
  const { t } = useTranslation()
  const { formatDate } = useFormatDate()
  const navigate = useNavigate()
  const noScannersAvailable = useNoScannersAvailable()
  const [filters, setFilters] = useSavedFilters('findings', {
    severityFilter: 'all',
    statusFilter: 'all',
    searchQuery: '',
  })
  const { severityFilter, statusFilter, searchQuery } = filters
  const [selected, setSelected] = useState<Set<string>>(new Set())
  const [hiddenIds, setHiddenIds] = useState<Set<string>>(new Set())
  const [bulkStatus, setBulkStatus] = useState<Finding['status']>('resolved')
  const [importOpen, setImportOpen] = useState(false)
  const [csvImportOpen, setCsvImportOpen] = useState(false)
  const [statusDialogOpen, setStatusDialogOpen] = useState(false)
  const [deleteDialogOpen, setDeleteDialogOpen] = useState(false)
  const [focusedIndex, setFocusedIndex] = useState(-1)
  const [page, setPage] = useState(1)
  const searchRef = useRef<HTMLInputElement>(null)

  const { data, isLoading, isError, error, refetch } = useFindings({
    severity: severityFilter === 'all' ? undefined : severityFilter,
    status: statusFilter === 'all' ? undefined : statusFilter,
    search: searchQuery || undefined,
  }, page)
  const bulkUpdate = useBulkUpdateFindings()
  const deleteFinding = useDeleteFinding()
  const queryClient = useQueryClient()

  const { scheduleDelete } = useDeferredDelete<Finding>({
    getLabel: (f) => f.title,
    onDelete: useCallback(async (f: Finding) => {
      await deleteFinding.mutateAsync(f.id)
      setHiddenIds((prev) => { const n = new Set(prev); n.delete(f.id); return n })
    }, [deleteFinding]),
    onUndo: useCallback((f: Finding) => {
      void queryClient.invalidateQueries({ queryKey: ['vaktscan', 'findings'] })
      setHiddenIds((prev) => { const n = new Set(prev); n.delete(f.id); return n })
    }, [queryClient]),
  })

  function handleDeleteFinding(f: Finding) {
    scheduleDelete(f, () => { setHiddenIds((prev) => new Set(prev).add(f.id)) })
  }

  const rawFindings = (data?.data ?? []).filter((f) => !hiddenIds.has(f.id))
  // Augment with numeric severity order for sorting
  const findingsWithOrder: SortableFinding[] = rawFindings.map((f) => ({
    ...f,
    severity_order: SEVERITY_ORDER[f.severity] || 0,
  }))
  const {
    sorted: sortedFindings,
    sortKey,
    sortDir,
    toggleSort,
  } = useSortableTable<SortableFinding>(findingsWithOrder, { key: 'created_at', dir: 'desc' })
  const findings = sortedFindings

  useKeyboardNav(focusedIndex, setFocusedIndex, {
    itemCount: findings.length,
    enabled: !importOpen && !statusDialogOpen && !deleteDialogOpen,
    onSearch: () => searchRef.current?.focus(),
    onSelect: (i) => {
      if (findings[i]) navigate(`/vaktscan/findings/${findings[i].id}`)
    },
    onEdit: (i) => {
      if (findings[i]) navigate(`/vaktscan/findings/${findings[i].id}`)
    },
  })

  const { focusIdx: tableKeyIdx, setFocusIdx: setTableKeyIdx, onKeyDown: tableRowKeyDown } = useTableKeyboard(
    findings.length,
    (idx) => {
      if (findings[idx]) navigate(`/vaktscan/findings/${findings[idx].id}`)
    },
  )

  function toggleSelect(id: string) {
    setSelected((prev) => {
      const next = new Set(prev)
      if (next.has(id)) next.delete(id)
      else next.add(id)
      return next
    })
  }

  function toggleAll() {
    if (selected.size === findings.length) setSelected(new Set())
    else setSelected(new Set(findings.map((f) => f.id)))
  }

  async function handleBulkUpdate() {
    if (selected.size === 0) return
    try {
      await bulkUpdate.mutateAsync({ ids: Array.from(selected), status: bulkStatus })
      setSelected(new Set())
      setStatusDialogOpen(false)
      toast(t('vaktscan.findingsPage.saved'), 'success')
    } catch {
      toast(t('vaktscan.findingsPage.saveFailed'), 'error')
    }
  }

  async function handleBulkDelete() {
    if (selected.size === 0) return
    const ids = Array.from(selected)
    try {
      await Promise.allSettled(ids.map((id) => deleteFinding.mutateAsync(id)))
      setSelected(new Set())
      setDeleteDialogOpen(false)
      toast(t('vaktscan.findingsPage.bulkDeleted', { count: ids.length }), 'success')
    } catch {
      toast(t('vaktscan.findingsPage.deleteFailed'), 'error')
    }
  }

  function handleExportSelected() {
    const selectedFindings = findings.filter((f) => selected.has(f.id))
    const rows = selectedFindings.map((f) => ({
      id: f.id,
      title: f.title,
      severity: f.severity,
      status: f.status,
      asset: f.asset_name ?? '',
      cve_id: f.cve_id ?? '',
      cvss_score: f.cvss_score ?? '',
      created_at: f.created_at,
      updated_at: f.updated_at,
    }))
    exportToCSV('findings-export', rows)
  }

  return (
    <div className="flex flex-col h-full">
      <ImportFindingsDialog
        open={importOpen}
        onOpenChange={setImportOpen}
      />
      <CSVImportDialog
        open={csvImportOpen}
        onClose={() => { setCsvImportOpen(false); }}
        endpoint="/api/v1/vaktscan/findings/import/csv"
        entityLabel="Findings"
        columns={['title', 'severity', 'description', 'asset', 'status']}
        onSuccess={() => void refetch()}
      />

      {/* Bulk status change dialog */}
      <Dialog open={statusDialogOpen} onOpenChange={setStatusDialogOpen}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>{t('vaktscan.findingsPage.statusChangeTitle')}</DialogTitle>
          </DialogHeader>
          <div className="py-3 space-y-3">
            <p className="text-sm text-secondary">
              {t('vaktscan.findingsPage.statusChangeDesc', {
                count: selected.size,
                suffix: selected.size === 1 ? 'n' : '',
              })}
            </p>
            <Select value={bulkStatus} onValueChange={(v) => { setBulkStatus(v as Finding['status']); }}>
              <SelectTrigger>
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value="open">{t('vaktscan.status.open')}</SelectItem>
                <SelectItem value="in_progress">{t('vaktscan.status.in_progress')}</SelectItem>
                <SelectItem value="accepted_risk">{t('vaktscan.status.accepted_risk')}</SelectItem>
                <SelectItem value="false_positive">{t('vaktscan.status.false_positive')}</SelectItem>
                <SelectItem value="resolved">{t('vaktscan.status.resolved')}</SelectItem>
              </SelectContent>
            </Select>
          </div>
          <DialogFooter>
            <Button variant="outline" onClick={() => { setStatusDialogOpen(false); }}>{t('common.cancel')}</Button>
            <Button onClick={() => { void handleBulkUpdate() }} disabled={bulkUpdate.isPending}>
              {bulkUpdate.isPending ? t('vaktscan.findingsPage.saving') : t('vaktscan.findingsPage.apply')}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      {/* Bulk delete confirm dialog */}
      <Dialog open={deleteDialogOpen} onOpenChange={setDeleteDialogOpen}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>{t('vaktscan.findingsPage.deleteTitle')}</DialogTitle>
          </DialogHeader>
          <div className="py-3">
            <p className="text-sm text-secondary">
              {t('vaktscan.findingsPage.deleteDesc', {
                count: selected.size,
                suffix: selected.size === 1 ? 'n' : '',
              })}
            </p>
          </div>
          <DialogFooter>
            <Button variant="outline" onClick={() => { setDeleteDialogOpen(false); }}>{t('common.cancel')}</Button>
            <Button
              variant="destructive"
              onClick={() => { void handleBulkDelete() }}
              disabled={deleteFinding.isPending}
            >
              {deleteFinding.isPending ? t('vaktscan.findingsPage.deleting') : `${selected.size} ${t('common.delete')}`}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      <PageHeader
        title={t('vaktscan.findingsPage.title')}
        description={data ? t('vaktscan.findingsPage.total', { count: data.pagination.total }) : undefined}
        tier="pro"
        actions={
          <>
            <Button variant="outline" size="sm" onClick={() => { setCsvImportOpen(true); }}>
              <Upload className="w-4 h-4 mr-1" />
              CSV importieren
            </Button>
            <Button variant="outline" size="sm" onClick={() => { setImportOpen(true); }}>
              <Upload className="w-4 h-4 mr-1" />
              {t('common.import')}
            </Button>
            <Button variant="outline" size="sm" onClick={() => { void exportFindingsCsv(); }}>
              <Download className="w-4 h-4 mr-1" />
              Export CSV
            </Button>
          </>
        }
      />

      <div className="p-6 space-y-4">
        {/* Search */}
        <div>
          <input
            ref={searchRef}
            type="text"
            value={searchQuery}
            onChange={(e) => { setFilters((f) => ({ ...f, searchQuery: e.target.value })); setFocusedIndex(-1); setPage(1) }}
            placeholder={t('vaktscan.findingsPage.searchPlaceholder')}
            aria-label={t('vaktscan.findingsPage.searchLabel')}
            className="w-full max-w-sm rounded-md border border-border bg-surface px-3 py-1.5 text-sm placeholder:text-muted focus:outline-none focus:ring-2 focus:ring-brand"
          />
          <p className="text-xs text-muted mt-1">{t('vaktscan.findingsPage.keyboardHint')}</p>
        </div>

        {/* Filters */}
        <div className="flex flex-wrap items-center gap-3">
          <Select value={severityFilter} onValueChange={(v) => { setFilters((f) => ({ ...f, severityFilter: v })); setPage(1) }}>
            <SelectTrigger className="w-40">
              <SelectValue placeholder={t('vaktscan.findingsPage.allSeverities')} />
            </SelectTrigger>
            <SelectContent>
              <SelectItem value="all">{t('vaktscan.findingsPage.allSeverities')}</SelectItem>
              <SelectItem value="critical">{t('vaktscan.severity.critical')}</SelectItem>
              <SelectItem value="high">{t('vaktscan.severity.high')}</SelectItem>
              <SelectItem value="medium">{t('vaktscan.severity.medium')}</SelectItem>
              <SelectItem value="low">{t('vaktscan.severity.low')}</SelectItem>
              <SelectItem value="info">{t('vaktscan.severity.info')}</SelectItem>
            </SelectContent>
          </Select>

          <Select value={statusFilter} onValueChange={(v) => { setFilters((f) => ({ ...f, statusFilter: v })); setPage(1) }}>
            <SelectTrigger className="w-40">
              <SelectValue placeholder={t('vaktscan.findingsPage.allStatuses')} />
            </SelectTrigger>
            <SelectContent>
              <SelectItem value="all">{t('vaktscan.findingsPage.allStatuses')}</SelectItem>
              <SelectItem value="open">{t('vaktscan.status.open')}</SelectItem>
              <SelectItem value="in_progress">{t('vaktscan.status.in_progress')}</SelectItem>
              <SelectItem value="accepted_risk">{t('vaktscan.status.accepted_risk')}</SelectItem>
              <SelectItem value="false_positive">{t('vaktscan.status.false_positive')}</SelectItem>
              <SelectItem value="resolved">{t('vaktscan.status.resolved')}</SelectItem>
            </SelectContent>
          </Select>

        </div>

        {noScannersAvailable && (
          <div className="flex items-start gap-3 rounded-lg border border-amber-500/30 bg-amber-500/10 px-4 py-3">
            <AlertTriangle className="w-5 h-5 text-amber-400 shrink-0 mt-0.5" aria-hidden="true" />
            <div className="flex-1 min-w-0">
              <p className="text-[13px] font-semibold text-amber-400">Scanner nicht eingerichtet</p>
              <p className="text-[11px] text-secondary mt-0.5">
                Trivy und Nuclei sind im Docker-Image gebündelt — prüfe ob die Instanz korrekt gestartet wurde. OpenVAS ist optional via <code className="text-[10px]">VAKT_OPENVAS_URL</code>.
              </p>
            </div>
            <a
              href="/docs/guides/scanner-setup"
              target="_blank"
              rel="noopener noreferrer"
              className="shrink-0 text-[11px] font-medium text-amber-400 hover:text-amber-300 transition-colors whitespace-nowrap"
            >
              Setup-Guide →
            </a>
          </div>
        )}

        {isLoading && (
          <div className="space-y-2">
            {Array.from({ length: 5 }).map((_, i) => (
              <Skeleton key={i} className="h-12 w-full rounded-lg" />
            ))}
          </div>
        )}
        {isError && (
          <ErrorState
            message={error instanceof Error ? error.message : undefined}
            onRetry={() => void refetch()}
          />
        )}

        {!isLoading && !isError && findings.length === 0 && (
          <EmptyState
            icon={AlertTriangle}
            title={t('vaktscan.findingsPage.noFindings')}
            description={t('vaktscan.findingsPage.noFindingsDesc')}
          />
        )}

        {!isLoading && !isError && findings.length > 0 && (
          <>
          {/* Mobile card list — shown below md breakpoint */}
          <div className="md:hidden space-y-3">
            {findings.map((f) => (
              <MobileCard
                key={f.id}
                title={f.title}
                subtitle={f.asset_name ?? undefined}
                badge={{
                  label: f.severity,
                  color: severityClass[f.severity],
                }}
                meta={[
                  { label: 'Status', value: f.status.replace(/_/g, ' ') },
                  { label: 'Erstellt', value: formatDate(f.created_at) },
                  ...(f.cve_id ? [{ label: 'CVE', value: f.cve_id }] : []),
                  ...(f.cvss_score != null ? [{ label: 'CVSS', value: f.cvss_score.toFixed(1) }] : []),
                ]}
                onClick={() => { navigate(`/vaktscan/findings/${f.id}`); }}
              />
            ))}
          </div>
          {/* Desktop table — hidden on mobile */}
          <div className="hidden md:block rounded-md border border-border bg-surface overflow-x-auto">
            {/* WCAG 1.3.1: aria-label names the table for screen readers */}
            <Table aria-label="Sicherheitsbefunde"
              role="grid"
            >
              <TableHeader>
                <TableRow>
                  <TableHead className="w-10">
                    <input
                      type="checkbox"
                      checked={selected.size === findings.length && findings.length > 0}
                      ref={(el) => {
                        if (el) el.indeterminate = selected.size > 0 && selected.size < findings.length
                      }}
                      onChange={toggleAll}
                      aria-label={t('vaktscan.findingsPage.selectAll')}
                      className="rounded"
                    />
                  </TableHead>
                  <SortableHeader
                    label={t('vaktscan.findingsPage.colTitle')}
                    sortKey="title"
                    currentSortKey={sortKey}
                    currentDir={sortDir}
                    onSort={toggleSort}
                    className="px-4 py-3 text-left text-sm font-medium text-secondary"
                  />
                  <SortableHeader
                    label={t('vaktscan.findingsPage.colSeverity')}
                    sortKey="severity_order"
                    currentSortKey={sortKey}
                    currentDir={sortDir}
                    onSort={toggleSort}
                    className="px-4 py-3 text-left text-sm font-medium text-secondary"
                  />
                  <SortableHeader
                    label={t('vaktscan.findingsPage.colStatus')}
                    sortKey="status"
                    currentSortKey={sortKey}
                    currentDir={sortDir}
                    onSort={toggleSort}
                    className="px-4 py-3 text-left text-sm font-medium text-secondary"
                  />
                  <TableHead>{t('vaktscan.findingsPage.colAsset')}</TableHead>
                  <TableHead>CVE / CVSS</TableHead>
                  <SortableHeader
                    label="Erstellt"
                    sortKey="created_at"
                    currentSortKey={sortKey}
                    currentDir={sortDir}
                    onSort={toggleSort}
                    className="px-4 py-3 text-left text-sm font-medium text-secondary"
                  />
                  <TableHead className="w-10" />
                </TableRow>
              </TableHeader>
              <TableBody>
                {findings.map((f, index) => (
                  <TableRow
                    key={f.id}
                    tabIndex={0}
                    ref={(el) => {
                      if (el && tableKeyIdx === index) el.focus()
                    }}
                    /* WCAG 2.1.1: onKeyDown allows keyboard activation of the row */
                    onKeyDown={(e) => {
                      tableRowKeyDown(e, index)
                      if (e.key === 'Enter' || e.key === ' ') {
                        e.preventDefault()
                        navigate(`/vaktscan/findings/${f.id}`)
                      }
                    }}
                    aria-selected={selected.has(f.id)}
                    className={cn(
                      'cursor-pointer hover:bg-surface2 [&>td]:py-3',
                      (index === focusedIndex || index === tableKeyIdx) && 'ring-1 ring-brand bg-brand/10 dark:bg-muted/50',
                      selected.has(f.id) && 'bg-brand/5',
                    )}
                    onClick={() => { setFocusedIndex(index); setTableKeyIdx(index) }}
                  >
                    <TableCell onClick={(e) => { e.stopPropagation(); }}>
                      <input
                        type="checkbox"
                        checked={selected.has(f.id)}
                        onChange={() => { toggleSelect(f.id); }}
                        aria-label={`Befund "${f.title}" auswählen`}
                        className="rounded"
                      />
                    </TableCell>
                    <TableCell
                      className="font-medium max-w-xs"
                      onClick={() => { navigate(`/vaktscan/findings/${f.id}`); }}
                    >
                      <span className="truncate block">{f.title}</span>
                      {f.sla_due_at && new Date(f.sla_due_at) < new Date() && (
                        <Badge className="mt-0.5 text-[10px] py-0 bg-red-500/20 text-red-400 border-red-500/30">
                          SLA überfällig
                        </Badge>
                      )}
                    </TableCell>
                    <TableCell onClick={(e) => { e.stopPropagation(); }}>
                      <InlineSeverityCell finding={f} />
                    </TableCell>
                    <TableCell onClick={(e) => { e.stopPropagation(); }}>
                      <InlineStatusCell finding={f} />
                    </TableCell>
                    <TableCell className="text-sm text-secondary" onClick={() => { navigate(`/vaktscan/findings/${f.id}`); }}>
                      {f.asset_name ?? '—'}
                    </TableCell>
                    <TableCell className="font-mono text-xs" onClick={() => { navigate(`/vaktscan/findings/${f.id}`); }}>
                      <span className="block">{f.cve_id ?? '—'}</span>
                      {f.cvss_score != null && (
                        <span className="text-[10px] text-secondary">{f.cvss_score.toFixed(1)}</span>
                      )}
                    </TableCell>
                    <TableCell className="text-sm text-secondary" onClick={() => { navigate(`/vaktscan/findings/${f.id}`); }}>
                      {formatDate(f.created_at)}
                    </TableCell>
                    <TableCell onClick={(e) => { e.stopPropagation(); }}>
                      <button
                        onClick={() => { handleDeleteFinding(f); }}
                        aria-label={`Befund "${f.title}" löschen`}
                        className="p-1 rounded text-secondary hover:text-red-400 hover:bg-red-500/10 transition-colors"
                      >
                        <X className="w-3.5 h-3.5" />
                      </button>
                    </TableCell>
                  </TableRow>
                ))}
              </TableBody>
            </Table>
          </div>
          </>
        )}
        <Pagination
          page={page}
          totalPages={data?.pagination.total_pages ?? 1}
          onPageChange={setPage}
        />
      </div>

      <BulkActionBar
        selectedCount={selected.size}
        onClearSelection={() => { setSelected(new Set()); }}
        actions={[
          {
            label: t('vaktscan.bulk.changeStatus'),
            icon: RefreshCw,
            onClick: () => { setStatusDialogOpen(true); },
          },
          {
            label: t('vaktscan.bulk.exportSelected'),
            icon: FileDown,
            onClick: handleExportSelected,
          },
          {
            label: t('vaktscan.bulk.delete'),
            icon: Trash2,
            variant: 'destructive' as const,
            onClick: () => { setDeleteDialogOpen(true); },
            disabled: deleteFinding.isPending,
          },
        ]}
      />
    </div>
  )
}
