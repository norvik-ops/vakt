import { useState, useRef } from 'react'
import { useSavedFilters } from '../../../shared/hooks/useSavedFilters'
import { useNavigate } from 'react-router-dom'
import { Download, AlertTriangle, Upload } from 'lucide-react'
import { PageHeader } from '../../../shared/components/PageHeader'
import { EmptyState } from '../../../shared/components/EmptyState'
import { Pagination } from '../../../shared/components/Pagination'
import { Button } from '../../../components/ui/button'
import { Badge } from '../../../components/ui/badge'
import { Select, SelectTrigger, SelectContent, SelectItem, SelectValue } from '../../../components/ui/select'
import { Table, TableHeader, TableBody, TableRow, TableHead, TableCell } from '../../../components/ui/table'
import { useFindings, exportFindingsCsv } from '../hooks/useFindings'
import { useBulkUpdateFindings } from '../hooks/useFindings'
import type { Finding } from '../types'
import { cn } from '../../../lib/utils'
import { ImportFindingsDialog } from '../components/ImportFindingsDialog'
import { useKeyboardNav } from '../../../shared/hooks/useKeyboardNav'

const severityClass: Record<Finding['severity'], string> = {
  info:     'bg-[#374151] text-[#94a3b8] border-transparent',
  low:      'bg-[#1e3a5f] text-[#93c5fd] border-transparent',
  medium:   'bg-[#78350f] text-[#f59e0b] border-transparent',
  high:     'bg-[#7c2d12] text-[#f97316] border-transparent',
  critical: 'bg-[#7f1d1d] text-[#ef4444] border-transparent',
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
  const [focusedIndex, setFocusedIndex] = useState(-1)
  const [page, setPage] = useState(1)
  const searchRef = useRef<HTMLInputElement>(null)

  const { data, isLoading, error } = useFindings({
    severity: severityFilter === 'all' ? undefined : severityFilter,
    status: statusFilter === 'all' ? undefined : statusFilter,
    search: searchQuery || undefined,
  }, page)
  const bulkUpdate = useBulkUpdateFindings()

  const findings = data?.data ?? []

  useKeyboardNav(focusedIndex, setFocusedIndex, {
    itemCount: findings.length,
    enabled: !importOpen,
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
    await bulkUpdate.mutateAsync({ ids: Array.from(selected), status: bulkStatus })
    setSelected(new Set())
  }

  return (
    <div className="flex flex-col h-full">
      <ImportFindingsDialog
        open={importOpen}
        onOpenChange={setImportOpen}
      />
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

          {selected.size > 0 && (
            <div className="flex items-center gap-2 ml-auto">
              <span className="text-sm text-secondary">{selected.size} ausgewählt</span>
              <Select value={bulkStatus} onValueChange={(v) => setBulkStatus(v as Finding['status'])}>
                <SelectTrigger className="w-40">
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
              <Button size="sm" onClick={() => { void handleBulkUpdate() }} disabled={bulkUpdate.isPending}>
                Anwenden
              </Button>
            </div>
          )}
        </div>

        {isLoading && (
          <div className="flex justify-center py-16">
            <div className="w-5 h-5 border-2 border-brand border-t-transparent rounded-full animate-spin" />
          </div>
        )}
        {error && <p className="text-sm text-red-600">Error: {error.message}</p>}

        {!isLoading && !error && findings.length === 0 && (
          <EmptyState
            icon={AlertTriangle}
            title="Keine Befunde"
            description="Keine Befunde entsprechen den aktuellen Filtern."
          />
        )}

        {!isLoading && !error && findings.length > 0 && (
          <div className="rounded-md border border-border bg-surface overflow-hidden">
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead className="w-10">
                    <input
                      type="checkbox"
                      checked={selected.size === findings.length && findings.length > 0}
                      onChange={toggleAll}
                      className="rounded"
                    />
                  </TableHead>
                  <TableHead>Titel</TableHead>
                  <TableHead>Schweregrad</TableHead>
                  <TableHead>Status</TableHead>
                  <TableHead>Asset</TableHead>
                  <TableHead>CVE</TableHead>
                  <TableHead>CVSS</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {findings.map((f, index) => (
                  <TableRow
                    key={f.id}
                    tabIndex={0}
                    className={cn(
                      'cursor-pointer hover:bg-surface2',
                      index === focusedIndex && 'ring-1 ring-brand bg-[#eef2ff] dark:bg-[#1E2235]'
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
    </div>
  )
}
