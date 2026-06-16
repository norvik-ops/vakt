import { useState } from 'react'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { Download, CheckCircle2, XCircle, RefreshCw, ShieldCheck, AlertTriangle, Pencil } from 'lucide-react'
import { Button } from '../../../components/ui/button'
import { Dialog, DialogContent, DialogHeader, DialogTitle, DialogFooter } from '../../../components/ui/dialog'
import { Input } from '../../../components/ui/input'
import { Label } from '../../../components/ui/label'
import { Textarea } from '../../../components/ui/textarea'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '../../../components/ui/select'
import {
  Table, TableBody, TableCell, TableHead, TableHeader, TableRow,
} from '../../../components/ui/table'
import { apiFetch } from '../../../api/client'
import { SkeletonTable } from '../../../shared/components/SkeletonLoaders'
import { ExportButton } from '../../../shared/components/ExportButton'
import { TermTooltip } from '../../../shared/components/TermTooltip'

interface SoADedicatedEntry {
  control_ref: string
  group: string
  title: string
  description: string
  applicable: boolean
  justification_included: string
  justification_excluded: string
  implementation_status: string
  owner: string
  evidence_note: string
}

interface SoADedicatedSummary {
  total: number
  applicable: number
  excluded: number
  without_exclusion_reason: number
  version_status: string
  draft_version: number
  approved_version: number
}

const IMPL_STATUS_LABELS: Record<string, string> = {
  not_started: 'Nicht begonnen',
  in_progress: 'In Bearbeitung',
  implemented: 'Umgesetzt',
  not_applicable: 'Nicht anwendbar',
  partial: 'Teilweise',
}

const IMPL_STATUS_COLORS: Record<string, string> = {
  implemented: 'bg-green-100 text-green-800',
  in_progress: 'bg-blue-100 text-blue-800',
  not_started: 'bg-gray-100 text-gray-700',
  not_applicable: 'bg-gray-50 text-gray-400',
  partial: 'bg-yellow-100 text-yellow-800',
}

const GROUP_LABELS: Record<string, string> = {
  '5': 'A.5 Organisatorische Maßnahmen',
  '6': 'A.6 Personenbezogene Maßnahmen',
  '7': 'A.7 Physische Maßnahmen',
  '8': 'A.8 Technologische Maßnahmen',
}

function useSoADedicated() {
  return useQuery<SoADedicatedEntry[]>({
    queryKey: ['vaktcomply', 'soa-dedicated'],
    queryFn: () => apiFetch<SoADedicatedEntry[]>('/vaktcomply/soa/entries'),
    staleTime: 2 * 60 * 1000,
    retry: false,
  })
}

function useSoASummary() {
  return useQuery<SoADedicatedSummary>({
    queryKey: ['vaktcomply', 'soa-dedicated-summary'],
    queryFn: () => apiFetch<SoADedicatedSummary>('/vaktcomply/soa/summary'),
    staleTime: 60 * 1000,
    retry: false,
  })
}

function useInitSoA() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: () => apiFetch<void>('/vaktcomply/soa/init', { method: 'POST' }),
    onSuccess: () => {
      void qc.invalidateQueries({ queryKey: ['vaktcomply', 'soa-dedicated'] })
      void qc.invalidateQueries({ queryKey: ['vaktcomply', 'soa-dedicated-summary'] })
    },
  })
}

function useUpdateSoAEntry() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: ({ ref, input }: { ref: string; input: Partial<SoADedicatedEntry> }) =>
      apiFetch<SoADedicatedEntry>(`/vaktcomply/soa/entries/${encodeURIComponent(ref)}`, {
        method: 'PUT',
        body: JSON.stringify(input),
      }),
    onSuccess: () => {
      void qc.invalidateQueries({ queryKey: ['vaktcomply', 'soa-dedicated'] })
      void qc.invalidateQueries({ queryKey: ['vaktcomply', 'soa-dedicated-summary'] })
    },
  })
}

function useApproveSoA() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: () => apiFetch<void>('/vaktcomply/soa/approve', { method: 'POST' }),
    onSuccess: () => {
      void qc.invalidateQueries({ queryKey: ['vaktcomply', 'soa-dedicated-summary'] })
    },
  })
}

interface EditForm {
  applicable: boolean
  justification_included: string
  justification_excluded: string
  implementation_status: string
  owner: string
  evidence_note: string
}

export default function SoAPage() {
  const { data: entries, isLoading, isError } = useSoADedicated()
  const { data: summary } = useSoASummary()
  const initMut = useInitSoA()
  const updateMut = useUpdateSoAEntry()
  const approveMut = useApproveSoA()

  const [activeGroup, setActiveGroup] = useState<string>('5')
  const [editEntry, setEditEntry] = useState<SoADedicatedEntry | null>(null)
  const [editForm, setEditForm] = useState<EditForm>({
    applicable: true,
    justification_included: '',
    justification_excluded: '',
    implementation_status: 'not_started',
    owner: '',
    evidence_note: '',
  })

  const notInitialized = isError || (entries && entries.length === 0)

  function openEdit(e: SoADedicatedEntry) {
    setEditEntry(e)
    setEditForm({
      applicable: e.applicable,
      justification_included: e.justification_included,
      justification_excluded: e.justification_excluded,
      implementation_status: e.implementation_status || 'not_started',
      owner: e.owner,
      evidence_note: e.evidence_note,
    })
  }

  function handleSaveEdit() {
    if (!editEntry) return
    updateMut.mutate(
      { ref: editEntry.control_ref, input: editForm },
      { onSuccess: () => { setEditEntry(null); } },
    )
  }

  function handleExport(format: 'pdf' | 'csv') {
    const url = `/api/v1/vaktcomply/soa/export?format=${format}`
    const a = document.createElement('a')
    a.href = url
    a.download = `soa-${new Date().toISOString().slice(0, 10)}.${format}`
    document.body.appendChild(a)
    a.click()
    a.remove()
  }

  const grouped = (entries ?? []).filter(e => e.group === activeGroup)

  if (isLoading) return <div className="p-8"><SkeletonTable rows={8} cols={5} /></div>

  if (notInitialized) {
    return (
      <div className="p-8 space-y-6">
        <div>
          <h1 className="text-2xl font-bold"><TermTooltip term="SoA" explanation="Statement of Applicability — ISO 27001:2022 Klausel 6.1.3: dokumentierte Aussage über die Anwendbarkeit aller 93 Maßnahmen aus Anhang A, inkl. Begründung für Ausschlüsse.">Statement of Applicability</TermTooltip></h1>
          <p className="text-gray-500 text-sm mt-1">ISO 27001:2022 Anhang A — Anwendbarkeit aller 93 Maßnahmen (Klausel 6.1.3)</p>
        </div>
        <div className="flex flex-col items-center justify-center bg-gray-50 border rounded-xl p-12 gap-4">
          <ShieldCheck className="h-12 w-12 text-gray-300" />
          <h2 className="text-lg font-semibold text-gray-700">SoA noch nicht initialisiert</h2>
          <p className="text-sm text-gray-500 text-center max-w-sm">
            Erstellt eine SoA-Version mit allen 93 ISO 27001:2022 Anhang A-Maßnahmen als Ausgangsbasis.
          </p>
          <Button onClick={() => { initMut.mutate(); }} disabled={initMut.isPending}>
            {initMut.isPending ? 'Wird erstellt…' : 'SoA initialisieren'}
          </Button>
        </div>
      </div>
    )
  }

  return (
    <div className="p-6 space-y-6">
      {/* Header */}
      <div className="flex items-start justify-between gap-4">
        <div>
          <h1 className="text-2xl font-bold"><TermTooltip term="SoA" explanation="Statement of Applicability — ISO 27001:2022 Klausel 6.1.3: dokumentierte Aussage über die Anwendbarkeit aller 93 Maßnahmen aus Anhang A, inkl. Begründung für Ausschlüsse.">Statement of Applicability</TermTooltip></h1>
          <p className="text-gray-500 text-sm mt-1">ISO 27001:2022 Anhang A — Klausel 6.1.3</p>
        </div>
        <div className="flex items-center gap-2">
          <Button variant="outline" size="sm" onClick={() => { handleExport('csv'); }}>
            <Download className="h-4 w-4 mr-1.5" />
            CSV
          </Button>
          <Button variant="outline" size="sm" onClick={() => { handleExport('pdf'); }}>
            <Download className="h-4 w-4 mr-1.5" />
            PDF
          </Button>
          <ExportButton
            endpoint="/api/v1/vaktcomply/soa/export.xlsx"
            filename={`soa-${new Date().toISOString().slice(0, 10)}`}
            label="XLSX"
            format="xlsx"
          />
          <ExportButton
            endpoint="/api/v1/vaktcomply/soa/export.docx"
            filename={`soa-${new Date().toISOString().slice(0, 10)}`}
            label="Word"
            format="docx"
          />
          {summary?.version_status === 'draft' && (
            <Button
              size="sm"
              onClick={() => { approveMut.mutate(); }}
              disabled={approveMut.isPending || (summary?.without_exclusion_reason ?? 0) > 0}
              className="bg-green-600 hover:bg-green-700 text-white"
              title={(summary?.without_exclusion_reason ?? 0) > 0 ? 'Zuerst alle Ausschlüsse begründen' : undefined}
            >
              <CheckCircle2 className="h-4 w-4 mr-1.5" />
              {approveMut.isPending ? 'Wird genehmigt…' : 'Version genehmigen'}
            </Button>
          )}
        </div>
      </div>

      {/* Version banner */}
      {summary && (
        <div className={`flex items-center gap-3 px-4 py-3 rounded-lg border text-sm ${
          summary.version_status === 'approved'
            ? 'bg-green-50 border-green-200 text-green-800'
            : 'bg-amber-50 border-amber-200 text-amber-800'
        }`}>
          {summary.version_status === 'approved'
            ? <CheckCircle2 className="h-4 w-4 shrink-0" />
            : <RefreshCw className="h-4 w-4 shrink-0" />}
          <span>
            {summary.version_status === 'approved'
              ? `Version ${summary.approved_version} genehmigt`
              : `Entwurf Version ${summary.draft_version} — noch nicht genehmigt`}
          </span>
          {(summary?.without_exclusion_reason ?? 0) > 0 && (
            <span className="ml-auto flex items-center gap-1 text-amber-700">
              <AlertTriangle className="h-4 w-4" />
              {summary.without_exclusion_reason} Ausschlüsse ohne Begründung
            </span>
          )}
        </div>
      )}

      {/* Stats */}
      <div className="grid grid-cols-4 gap-4">
        <div className="bg-white border rounded-lg p-4">
          <div className="text-2xl font-bold">{summary?.total ?? 93}</div>
          <div className="text-xs text-gray-500 mt-0.5">Kontrollen gesamt</div>
        </div>
        <div className="bg-green-50 border border-green-200 rounded-lg p-4">
          <div className="text-2xl font-bold text-green-700">{summary?.applicable ?? 0}</div>
          <div className="text-xs text-green-600 mt-0.5">Anwendbar</div>
        </div>
        <div className="bg-gray-50 border rounded-lg p-4">
          <div className="text-2xl font-bold text-gray-500">{summary?.excluded ?? 0}</div>
          <div className="text-xs text-gray-500 mt-0.5">Ausgeschlossen</div>
        </div>
        <div className="bg-amber-50 border border-amber-200 rounded-lg p-4">
          <div className="text-2xl font-bold text-amber-700">{summary?.without_exclusion_reason ?? 0}</div>
          <div className="text-xs text-amber-600 mt-0.5">Ohne Begründung</div>
        </div>
      </div>

      {/* Group tabs */}
      <div className="flex gap-1 border-b">
        {Object.entries(GROUP_LABELS).map(([g, label]) => (
          <button
            key={g}
            onClick={() => { setActiveGroup(g); }}
            className={`px-4 py-2 text-sm font-medium border-b-2 -mb-px transition-colors ${
              activeGroup === g
                ? 'border-blue-600 text-blue-600'
                : 'border-transparent text-gray-500 hover:text-gray-700'
            }`}
          >
            {label}
          </button>
        ))}
      </div>

      {/* Table */}
      <div className="bg-white rounded-lg border overflow-x-auto">
        <Table>
          <TableHeader>
            <TableRow>
              <TableHead className="w-20">Ref.</TableHead>
              <TableHead>Maßnahme</TableHead>
              <TableHead className="w-32">Status</TableHead>
              <TableHead className="w-24 text-center">Anwendbar</TableHead>
              <TableHead className="w-16"></TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {grouped.length === 0 && (
              <TableRow>
                <TableCell colSpan={5} className="text-center text-gray-400 py-8">Keine Einträge</TableCell>
              </TableRow>
            )}
            {grouped.map(e => (
              <TableRow key={e.control_ref} className="hover:bg-gray-50">
                <TableCell className="font-mono text-xs text-gray-500">{e.control_ref}</TableCell>
                <TableCell>
                  <div className="text-sm font-medium">{e.title}</div>
                  {e.applicable && e.justification_included && (
                    <div className="text-xs text-gray-400 mt-0.5 line-clamp-1">{e.justification_included}</div>
                  )}
                  {!e.applicable && e.justification_excluded && (
                    <div className="text-xs text-gray-400 mt-0.5 line-clamp-1 italic">{e.justification_excluded}</div>
                  )}
                </TableCell>
                <TableCell>
                  {e.applicable && (
                    <span className={`text-xs px-2 py-0.5 rounded-full ${IMPL_STATUS_COLORS[e.implementation_status] ?? 'bg-gray-100 text-gray-700'}`}>
                      {IMPL_STATUS_LABELS[e.implementation_status] ?? e.implementation_status}
                    </span>
                  )}
                </TableCell>
                <TableCell className="text-center">
                  {e.applicable
                    ? <CheckCircle2 className="h-5 w-5 text-green-500 mx-auto" />
                    : <XCircle className="h-5 w-5 text-gray-300 mx-auto" />}
                </TableCell>
                <TableCell>
                  <Button size="icon" variant="ghost" className="h-7 w-7" onClick={() => { openEdit(e); }}>
                    <Pencil className="h-3.5 w-3.5" />
                  </Button>
                </TableCell>
              </TableRow>
            ))}
          </TableBody>
        </Table>
      </div>

      {/* Edit Dialog */}
      <Dialog open={editEntry !== null} onOpenChange={(open) => { if (!open) setEditEntry(null); }}>
        <DialogContent className="max-w-lg max-h-[90vh] overflow-y-auto">
          <DialogHeader>
            <DialogTitle>
              {editEntry?.control_ref} — {editEntry?.title}
            </DialogTitle>
          </DialogHeader>
          <div className="space-y-4 py-2">
            <div className="space-y-1.5">
              <Label>Anwendbar</Label>
              <Select
                value={editForm.applicable ? 'yes' : 'no'}
                onValueChange={(v) => { setEditForm(f => ({ ...f, applicable: v === 'yes' })); }}
              >
                <SelectTrigger>
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="yes">Ja — anwendbar</SelectItem>
                  <SelectItem value="no">Nein — nicht anwendbar</SelectItem>
                </SelectContent>
              </Select>
            </div>
            {editForm.applicable ? (
              <>
                <div className="space-y-1.5">
                  <Label>Begründung der Aufnahme</Label>
                  <Textarea
                    rows={2}
                    placeholder="Warum ist diese Maßnahme anwendbar?"
                    value={editForm.justification_included}
                    onChange={(e) => { setEditForm(f => ({ ...f, justification_included: e.target.value })); }}
                  />
                </div>
                <div className="space-y-1.5">
                  <Label>Umsetzungsstatus</Label>
                  <Select
                    value={editForm.implementation_status}
                    onValueChange={(v) => { setEditForm(f => ({ ...f, implementation_status: v })); }}
                  >
                    <SelectTrigger><SelectValue /></SelectTrigger>
                    <SelectContent>
                      {Object.entries(IMPL_STATUS_LABELS).filter(([v]) => v !== 'not_applicable').map(([v, l]) => (
                        <SelectItem key={v} value={v}>{l}</SelectItem>
                      ))}
                    </SelectContent>
                  </Select>
                </div>
                <div className="space-y-1.5">
                  <Label>Verantwortliche Person</Label>
                  <Input
                    placeholder="Name oder Team"
                    value={editForm.owner}
                    onChange={(e) => { setEditForm(f => ({ ...f, owner: e.target.value })); }}
                  />
                </div>
                <div className="space-y-1.5">
                  <Label>Nachweisnotiz</Label>
                  <Textarea
                    rows={2}
                    placeholder="Verweis auf Dokumentation, Policy, o.ä."
                    value={editForm.evidence_note}
                    onChange={(e) => { setEditForm(f => ({ ...f, evidence_note: e.target.value })); }}
                  />
                </div>
              </>
            ) : (
              <div className="space-y-1.5">
                <Label>Begründung des Ausschlusses *</Label>
                <Textarea
                  rows={3}
                  placeholder="Warum ist diese Maßnahme nicht anwendbar? (Pflichtfeld für Genehmigung)"
                  value={editForm.justification_excluded}
                  onChange={(e) => { setEditForm(f => ({ ...f, justification_excluded: e.target.value })); }}
                />
              </div>
            )}
          </div>
          <DialogFooter>
            <Button variant="outline" onClick={() => { setEditEntry(null); }}>Abbrechen</Button>
            <Button onClick={handleSaveEdit} disabled={updateMut.isPending}>
              {updateMut.isPending ? 'Speichern…' : 'Speichern'}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </div>
  )
}
