import { useState, useRef } from 'react'
import { useSavedFilters } from '../../../shared/hooks/useSavedFilters'
import { useNavigate } from 'react-router-dom'
import { useTranslation } from 'react-i18next'
import { Download, AlertTriangle, Upload, Trash2, RefreshCw, FileDown, ExternalLink } from 'lucide-react'
import { PageHeader } from '../../../shared/components/PageHeader'
import { EmptyState } from '../../../shared/components/EmptyState'
import { Pagination } from '../../../shared/components/Pagination'
import { BulkActionBar } from '../../../shared/components/BulkActionBar'
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
import { useFindings, exportFindingsCsv, useBulkUpdateFindings, useDeleteFinding } from '../hooks/useFindings'
import type { Finding } from '../types'
import { cn } from '../../../lib/utils'
import { ImportFindingsDialog } from '../components/ImportFindingsDialog'
import { useKeyboardNav } from '../../../shared/hooks/useKeyboardNav'
import { toast } from '../../../shared/hooks/useToast'
import { Skeleton } from '../../../components/ui/skeleton'
import { exportToCSV } from '../../../lib/csv'
import { ErrorState } from '../../../shared/components/ErrorState'
import { useJiraConfig, useCreateJiraIssue } from '../../../hooks/useJira'
import type { JiraIssue } from '../../../hooks/useJira'

const severityClass: Record<Finding['severity'], string> = {
  info:     'bg-[#374151] text-[#94a3b8] border-transparent',
  low:      'bg-[#1e3a5f] text-[#93c5fd] border-transparent',
  medium:   'bg-[#78350f] text-[#f59e0b] border-transparent',
  high:     'bg-[#7c2d12] text-[#f97316] border-transparent',
  critical: 'bg-[#7f1d1d] text-[#ef4444] border-transparent',
}

// --- Jira issue cell ---

function JiraCell({ findingId, issue, isConfigured }: { findingId: string; issue: JiraIssue | undefined; isConfigured: boolean }) {
  const { t } = useTranslation()
  const createIssue = useCreateJiraIssue()

  if (!isConfigured) return null

  if (issue) {
    return (
      <a
        href={issue.issue_url}
        target="_blank"
        rel="noreferrer"
        onClick={(e) => e.stopPropagation()}
        className="inline-flex items-center gap-1 text-xs font-mono text-brand hover:underline"
        aria-label={`Jira-Ticket ${issue.issue_key} öffnen (neues Fenster)`}
      >
        {issue.issue_key}
        {/* WCAG 1.1.1: icon is decorative, link is named by aria-label */}
        <ExternalLink className="w-3 h-3" aria-hidden="true" />
      </a>
    )
  }

  return (
    <button
      onClick={(e) => {
        e.stopPropagation()
        createIssue.mutate(findingId, {
          onSuccess: (data) => {
            toast(t('secpulse.findingsPage.ticketCreated', { key: data.issue_key }), 'success')
          },
          onError: (err) => {
            toast(err.message, 'error')
          },
        })
      }}
      disabled={createIssue.isPending}
      title={t('secpulse.findingsPage.createJiraTicket')}
      className="inline-flex items-center gap-1 text-xs text-secondary hover:text-brand transition-colors disabled:opacity-50"
    >
      <ExternalLink className="w-3.5 h-3.5" aria-hidden="true" />
      Ticket
    </button>
  )
}

export default function FindingsPage() {
  const { t } = useTranslation()
  const navigate = useNavigate()
  const [filters, setFilters] = useSavedFilters('findings', {
    severityFilter: 'all',
    statusFilter: 'all',
    searchQuery: '',
  })
  const { severityFilter, statusFilter, searchQuery } = filters
  const [selected, setSelected] = useState<Set<string>>(new Set())
  const [bulkStatus, setBulkStatus] = useState<Finding['status']>('resolved')
  const [importOpen, setImportOpen] = useState(false)
  const [statusDialogOpen, setStatusDialogOpen] = useState(false)
  const [deleteDialogOpen, setDeleteDialogOpen] = useState(false)
  const [focusedIndex, setFocusedIndex] = useState(-1)
  const [page, setPage] = useState(1)
  const [jiraIssueMap, setJiraIssueMap] = useState<Map<string, JiraIssue>>(new Map())
  const searchRef = useRef<HTMLInputElement>(null)

  const { data, isLoading, isError, error, refetch } = useFindings({
    severity: severityFilter === 'all' ? undefined : severityFilter,
    status: statusFilter === 'all' ? undefined : statusFilter,
    search: searchQuery || undefined,
  }, page)
  const bulkUpdate = useBulkUpdateFindings()
  const deleteFinding = useDeleteFinding()
  const createJiraIssue = useCreateJiraIssue()
  const { data: jiraConfig } = useJiraConfig()

  const findings = data?.data ?? []
  const jiraConfigured = jiraConfig?.is_configured ?? false

  useKeyboardNav(focusedIndex, setFocusedIndex, {
    itemCount: findings.length,
    enabled: !importOpen && !statusDialogOpen && !deleteDialogOpen,
    onSearch: () => searchRef.current?.focus(),
    onSelect: (i) => {
      if (findings[i]) navigate(`/secpulse/findings/${findings[i].id}`)
    },
    onEdit: (i) => {
      if (findings[i]) navigate(`/secpulse/findings/${findings[i].id}`)
    },
  })

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
      toast(t('secpulse.findingsPage.saved'), 'success')
    } catch {
      toast(t('secpulse.findingsPage.saveFailed'), 'error')
    }
  }

  async function handleBulkDelete() {
    if (selected.size === 0) return
    const ids = Array.from(selected)
    try {
      await Promise.allSettled(ids.map((id) => deleteFinding.mutateAsync(id)))
      setSelected(new Set())
      setDeleteDialogOpen(false)
      toast(t('secpulse.findingsPage.bulkDeleted', { count: ids.length }), 'success')
    } catch {
      toast(t('secpulse.findingsPage.deleteFailed'), 'error')
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

  async function handleBulkCreateJiraIssues() {
    if (selected.size === 0) return
    const ids = Array.from(selected)
    let created = 0
    let failed = 0
    for (const id of ids) {
      try {
        const result = await createJiraIssue.mutateAsync(id)
        setJiraIssueMap((prev) => new Map(prev).set(id, {
          id: '',
          org_id: '',
          finding_id: id,
          issue_key: result.issue_key,
          issue_url: result.issue_url,
          created_at: new Date().toISOString(),
        }))
        created++
      } catch {
        failed++
      }
    }
    if (created > 0) toast(t('secpulse.findingsPage.jiraTicketsCreated', { count: created }), 'success')
    if (failed > 0) toast(t('secpulse.findingsPage.jiraTicketsFailed', { count: failed }), 'error')
  }

  return (
    <div className="flex flex-col h-full">
      <ImportFindingsDialog
        open={importOpen}
        onOpenChange={setImportOpen}
      />

      {/* Bulk status change dialog */}
      <Dialog open={statusDialogOpen} onOpenChange={setStatusDialogOpen}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>{t('secpulse.findingsPage.statusChangeTitle')}</DialogTitle>
          </DialogHeader>
          <div className="py-3 space-y-3">
            <p className="text-sm text-secondary">
              {t('secpulse.findingsPage.statusChangeDesc', {
                count: selected.size,
                suffix: selected.size === 1 ? 'n' : '',
              })}
            </p>
            <Select value={bulkStatus} onValueChange={(v) => setBulkStatus(v as Finding['status'])}>
              <SelectTrigger>
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value="open">{t('secpulse.status.open')}</SelectItem>
                <SelectItem value="in_progress">{t('secpulse.status.in_progress')}</SelectItem>
                <SelectItem value="accepted_risk">{t('secpulse.status.accepted_risk')}</SelectItem>
                <SelectItem value="false_positive">{t('secpulse.status.false_positive')}</SelectItem>
                <SelectItem value="resolved">{t('secpulse.status.resolved')}</SelectItem>
              </SelectContent>
            </Select>
          </div>
          <DialogFooter>
            <Button variant="outline" onClick={() => setStatusDialogOpen(false)}>{t('common.cancel')}</Button>
            <Button onClick={() => { void handleBulkUpdate() }} disabled={bulkUpdate.isPending}>
              {bulkUpdate.isPending ? t('secpulse.findingsPage.saving') : t('secpulse.findingsPage.apply')}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      {/* Bulk delete confirm dialog */}
      <Dialog open={deleteDialogOpen} onOpenChange={setDeleteDialogOpen}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>{t('secpulse.findingsPage.deleteTitle')}</DialogTitle>
          </DialogHeader>
          <div className="py-3">
            <p className="text-sm text-secondary">
              {t('secpulse.findingsPage.deleteDesc', {
                count: selected.size,
                suffix: selected.size === 1 ? 'n' : '',
              })}
            </p>
          </div>
          <DialogFooter>
            <Button variant="outline" onClick={() => setDeleteDialogOpen(false)}>{t('common.cancel')}</Button>
            <Button
              variant="destructive"
              onClick={() => { void handleBulkDelete() }}
              disabled={deleteFinding.isPending}
            >
              {deleteFinding.isPending ? t('secpulse.findingsPage.deleting') : `${selected.size} ${t('common.delete')}`}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      <PageHeader
        title={t('secpulse.findingsPage.title')}
        description={data ? t('secpulse.findingsPage.total', { count: data.pagination.total }) : undefined}
        actions={
          <>
            <Button variant="outline" size="sm" onClick={() => setImportOpen(true)}>
              <Upload className="w-4 h-4 mr-1" />
              {t('common.import')}
            </Button>
            <Button variant="outline" size="sm" onClick={exportFindingsCsv}>
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
            placeholder={t('secpulse.findingsPage.searchPlaceholder')}
            aria-label={t('secpulse.findingsPage.searchLabel')}
            className="w-full max-w-sm rounded-md border border-border bg-surface px-3 py-1.5 text-sm placeholder:text-muted focus:outline-none focus:ring-2 focus:ring-brand"
          />
          <p className="text-xs text-muted mt-1">{t('secpulse.findingsPage.keyboardHint')}</p>
        </div>

        {/* Filters */}
        <div className="flex flex-wrap items-center gap-3">
          <Select value={severityFilter} onValueChange={(v) => { setFilters((f) => ({ ...f, severityFilter: v })); setPage(1) }}>
            <SelectTrigger className="w-40">
              <SelectValue placeholder={t('secpulse.findingsPage.allSeverities')} />
            </SelectTrigger>
            <SelectContent>
              <SelectItem value="all">{t('secpulse.findingsPage.allSeverities')}</SelectItem>
              <SelectItem value="critical">{t('secpulse.severity.critical')}</SelectItem>
              <SelectItem value="high">{t('secpulse.severity.high')}</SelectItem>
              <SelectItem value="medium">{t('secpulse.severity.medium')}</SelectItem>
              <SelectItem value="low">{t('secpulse.severity.low')}</SelectItem>
              <SelectItem value="info">{t('secpulse.severity.info')}</SelectItem>
            </SelectContent>
          </Select>

          <Select value={statusFilter} onValueChange={(v) => { setFilters((f) => ({ ...f, statusFilter: v })); setPage(1) }}>
            <SelectTrigger className="w-40">
              <SelectValue placeholder={t('secpulse.findingsPage.allStatuses')} />
            </SelectTrigger>
            <SelectContent>
              <SelectItem value="all">{t('secpulse.findingsPage.allStatuses')}</SelectItem>
              <SelectItem value="open">{t('secpulse.status.open')}</SelectItem>
              <SelectItem value="in_progress">{t('secpulse.status.in_progress')}</SelectItem>
              <SelectItem value="accepted_risk">{t('secpulse.status.accepted_risk')}</SelectItem>
              <SelectItem value="false_positive">{t('secpulse.status.false_positive')}</SelectItem>
              <SelectItem value="resolved">{t('secpulse.status.resolved')}</SelectItem>
            </SelectContent>
          </Select>

        </div>

        {isLoading && (
          <div className="space-y-2">
            {Array.from({ length: 5 }).map((_, i) => (
              <Skeleton key={i} className="h-12 w-full rounded-lg" />
            ))}
          </div>
        )}
        {isError && (
          <ErrorState
            message={error?.message}
            onRetry={() => void refetch()}
          />
        )}

        {!isLoading && !isError && findings.length === 0 && (
          <EmptyState
            icon={AlertTriangle}
            title={t('secpulse.findingsPage.noFindings')}
            description={t('secpulse.findingsPage.noFindingsDesc')}
          />
        )}

        {!isLoading && !isError && findings.length > 0 && (
          <div className="rounded-md border border-border bg-surface overflow-x-auto">
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
                      aria-label={t('secpulse.findingsPage.selectAll')}
                      className="rounded"
                    />
                  </TableHead>
                  <TableHead>{t('secpulse.findingsPage.colTitle')}</TableHead>
                  <TableHead>{t('secpulse.findingsPage.colSeverity')}</TableHead>
                  <TableHead>{t('secpulse.findingsPage.colStatus')}</TableHead>
                  <TableHead>{t('secpulse.findingsPage.colAsset')}</TableHead>
                  <TableHead>{t('secpulse.findingsPage.colCve')}</TableHead>
                  <TableHead>{t('secpulse.findingsPage.colCvss')}</TableHead>
                  {jiraConfigured && <TableHead>{t('secpulse.findingsPage.colJira')}</TableHead>}
                </TableRow>
              </TableHeader>
              <TableBody>
                {findings.map((f, index) => (
                  <TableRow
                    key={f.id}
                    tabIndex={0}
                    /* WCAG 2.1.1: onKeyDown allows keyboard activation of the row */
                    onKeyDown={(e) => {
                      if (e.key === 'Enter' || e.key === ' ') {
                        e.preventDefault()
                        navigate(`/secpulse/findings/${f.id}`)
                      }
                    }}
                    aria-selected={selected.has(f.id)}
                    className={cn(
                      'cursor-pointer hover:bg-surface2',
                      index === focusedIndex && 'ring-1 ring-brand bg-brand/10 dark:bg-muted/50',
                      selected.has(f.id) && 'bg-brand/5',
                    )}
                    onClick={() => setFocusedIndex(index)}
                  >
                    <TableCell onClick={(e) => e.stopPropagation()}>
                      <input
                        type="checkbox"
                        checked={selected.has(f.id)}
                        onChange={() => toggleSelect(f.id)}
                        aria-label={`Befund "${f.title}" auswählen`}
                        className="rounded"
                      />
                    </TableCell>
                    <TableCell
                      className="font-medium max-w-xs truncate"
                      onClick={() => navigate(`/secpulse/findings/${f.id}`)}
                    >
                      {f.title}
                    </TableCell>
                    <TableCell onClick={() => navigate(`/secpulse/findings/${f.id}`)}>
                      <Badge className={cn('capitalize', severityClass[f.severity])}>{f.severity}</Badge>
                    </TableCell>
                    <TableCell onClick={() => navigate(`/secpulse/findings/${f.id}`)}>
                      <span className="text-sm text-secondary capitalize">{f.status.replace(/_/g, ' ')}</span>
                    </TableCell>
                    <TableCell className="text-sm text-secondary" onClick={() => navigate(`/secpulse/findings/${f.id}`)}>
                      {f.asset_name ?? '—'}
                    </TableCell>
                    <TableCell className="font-mono text-xs" onClick={() => navigate(`/secpulse/findings/${f.id}`)}>
                      {f.cve_id ?? '—'}
                    </TableCell>
                    <TableCell onClick={() => navigate(`/secpulse/findings/${f.id}`)}>
                      {f.cvss_score != null ? f.cvss_score.toFixed(1) : '—'}
                    </TableCell>
                    {jiraConfigured && (
                      <TableCell onClick={(e) => e.stopPropagation()}>
                        <JiraCell
                          findingId={f.id}
                          issue={jiraIssueMap.get(f.id)}
                          isConfigured={jiraConfigured}
                        />
                      </TableCell>
                    )}
                  </TableRow>
                ))}
              </TableBody>
            </Table>
          </div>
        )}
        <Pagination
          page={page}
          totalPages={data?.pagination.total_pages ?? 1}
          onPageChange={setPage}
        />
      </div>

      <BulkActionBar
        selectedCount={selected.size}
        onClearSelection={() => setSelected(new Set())}
        actions={[
          {
            label: t('secpulse.bulk.changeStatus'),
            icon: RefreshCw,
            onClick: () => setStatusDialogOpen(true),
          },
          {
            label: t('secpulse.bulk.exportSelected'),
            icon: FileDown,
            onClick: handleExportSelected,
          },
          ...(jiraConfigured ? [{
            label: t('secpulse.bulk.createJiraIssues'),
            icon: ExternalLink,
            onClick: () => { void handleBulkCreateJiraIssues() },
            disabled: createJiraIssue.isPending,
          }] : []),
          {
            label: t('secpulse.bulk.delete'),
            icon: Trash2,
            variant: 'destructive' as const,
            onClick: () => setDeleteDialogOpen(true),
            disabled: deleteFinding.isPending,
          },
        ]}
      />
    </div>
  )
}
