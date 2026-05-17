import { useState, useRef } from 'react'
import { useSavedFilters } from '../../../shared/hooks/useSavedFilters'
import { useNavigate } from 'react-router-dom'
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
      >
        {issue.issue_key}
        <ExternalLink className="w-3 h-3" />
      </a>
    )
  }

  return (
    <button
      onClick={(e) => {
        e.stopPropagation()
        createIssue.mutate(findingId, {
          onSuccess: (data) => {
            toast(`Ticket ${data.issue_key} erstellt`, 'success')
          },
          onError: (err) => {
            toast(err.message, 'error')
          },
        })
      }}
      disabled={createIssue.isPending}
      title="Jira-Ticket erstellen"
      className="inline-flex items-center gap-1 text-xs text-secondary hover:text-brand transition-colors disabled:opacity-50"
    >
      <ExternalLink className="w-3.5 h-3.5" />
      Ticket
    </button>
  )
}

export default function FindingsPage() {
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
      toast('Gespeichert', 'success')
    } catch {
      toast('Etwas ist schiefgelaufen', 'error')
    }
  }

  async function handleBulkDelete() {
    if (selected.size === 0) return
    const ids = Array.from(selected)
    try {
      await Promise.allSettled(ids.map((id) => deleteFinding.mutateAsync(id)))
      setSelected(new Set())
      setDeleteDialogOpen(false)
      toast(`${ids.length} Befund(e) gelöscht`, 'success')
    } catch {
      toast('Löschen teilweise fehlgeschlagen', 'error')
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
    if (created > 0) toast(`${created} Jira-Ticket(s) erstellt`, 'success')
    if (failed > 0) toast(`${failed} Ticket(s) fehlgeschlagen`, 'error')
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
            <DialogTitle>Status ändern</DialogTitle>
          </DialogHeader>
          <div className="py-3 space-y-3">
            <p className="text-sm text-secondary">
              Neuen Status für {selected.size} ausgewählte{selected.size === 1 ? 'n Befund' : ' Befunde'} setzen:
            </p>
            <Select value={bulkStatus} onValueChange={(v) => setBulkStatus(v as Finding['status'])}>
              <SelectTrigger>
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value="open">Offen</SelectItem>
                <SelectItem value="in_progress">In Bearbeitung</SelectItem>
                <SelectItem value="accepted_risk">Akzeptiertes Risiko</SelectItem>
                <SelectItem value="false_positive">Falsch positiv</SelectItem>
                <SelectItem value="resolved">Behoben</SelectItem>
              </SelectContent>
            </Select>
          </div>
          <DialogFooter>
            <Button variant="outline" onClick={() => setStatusDialogOpen(false)}>Abbrechen</Button>
            <Button onClick={() => { void handleBulkUpdate() }} disabled={bulkUpdate.isPending}>
              {bulkUpdate.isPending ? 'Wird gespeichert…' : 'Anwenden'}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      {/* Bulk delete confirm dialog */}
      <Dialog open={deleteDialogOpen} onOpenChange={setDeleteDialogOpen}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>Befunde löschen</DialogTitle>
          </DialogHeader>
          <div className="py-3">
            <p className="text-sm text-secondary">
              Möchtest du {selected.size} ausgewählte{selected.size === 1 ? 'n Befund' : ' Befunde'} endgültig löschen?
              Diese Aktion kann nicht rückgängig gemacht werden.
            </p>
          </div>
          <DialogFooter>
            <Button variant="outline" onClick={() => setDeleteDialogOpen(false)}>Abbrechen</Button>
            <Button
              variant="destructive"
              onClick={() => { void handleBulkDelete() }}
              disabled={deleteFinding.isPending}
            >
              {deleteFinding.isPending ? 'Wird gelöscht…' : `${selected.size} löschen`}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      <PageHeader
        title="Sicherheitsbefunde"
        description={data ? `${data.pagination.total} Gesamt-Befunde` : undefined}
        actions={
          <>
            <Button variant="outline" size="sm" onClick={() => setImportOpen(true)}>
              <Upload className="w-4 h-4 mr-1" />
              Importieren
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
            placeholder="Suchen… (Titel, CVE, Asset)"
            aria-label="Suche nach Findings"
            className="w-full max-w-sm rounded-md border border-border bg-surface px-3 py-1.5 text-sm placeholder:text-muted focus:outline-none focus:ring-2 focus:ring-brand"
          />
          <p className="text-xs text-muted mt-1">j/k navigieren · Enter öffnen · e bearbeiten · / suchen</p>
        </div>

        {/* Filters */}
        <div className="flex flex-wrap items-center gap-3">
          <Select value={severityFilter} onValueChange={(v) => { setFilters((f) => ({ ...f, severityFilter: v })); setPage(1) }}>
            <SelectTrigger className="w-40">
              <SelectValue placeholder="Alle Schweregrade" />
            </SelectTrigger>
            <SelectContent>
              <SelectItem value="all">Alle Schweregrade</SelectItem>
              <SelectItem value="critical">Kritisch</SelectItem>
              <SelectItem value="high">Hoch</SelectItem>
              <SelectItem value="medium">Mittel</SelectItem>
              <SelectItem value="low">Niedrig</SelectItem>
              <SelectItem value="info">Info</SelectItem>
            </SelectContent>
          </Select>

          <Select value={statusFilter} onValueChange={(v) => { setFilters((f) => ({ ...f, statusFilter: v })); setPage(1) }}>
            <SelectTrigger className="w-40">
              <SelectValue placeholder="Alle Status" />
            </SelectTrigger>
            <SelectContent>
              <SelectItem value="all">Alle Status</SelectItem>
              <SelectItem value="open">Offen</SelectItem>
              <SelectItem value="in_progress">In Bearbeitung</SelectItem>
              <SelectItem value="accepted_risk">Akzeptiertes Risiko</SelectItem>
              <SelectItem value="false_positive">Falsch positiv</SelectItem>
              <SelectItem value="resolved">Behoben</SelectItem>
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
            title="Keine Befunde"
            description="Keine Befunde entsprechen den aktuellen Filtern."
          />
        )}

        {!isLoading && !isError && findings.length > 0 && (
          <div className="rounded-md border border-border bg-surface overflow-x-auto">
            <Table>
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
                      aria-label="Alle Befunde auswählen"
                      className="rounded"
                    />
                  </TableHead>
                  <TableHead>Titel</TableHead>
                  <TableHead>Schweregrad</TableHead>
                  <TableHead>Status</TableHead>
                  <TableHead>Asset</TableHead>
                  <TableHead>CVE</TableHead>
                  <TableHead>CVSS</TableHead>
                  {jiraConfigured && <TableHead>Jira</TableHead>}
                </TableRow>
              </TableHeader>
              <TableBody>
                {findings.map((f, index) => (
                  <TableRow
                    key={f.id}
                    tabIndex={0}
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
            label: 'Status ändern',
            icon: RefreshCw,
            onClick: () => setStatusDialogOpen(true),
          },
          {
            label: 'Exportieren',
            icon: FileDown,
            onClick: handleExportSelected,
          },
          ...(jiraConfigured ? [{
            label: 'Jira-Tickets erstellen',
            icon: ExternalLink,
            onClick: () => { void handleBulkCreateJiraIssues() },
            disabled: createJiraIssue.isPending,
          }] : []),
          {
            label: 'Löschen',
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
