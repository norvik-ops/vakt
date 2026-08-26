import { useState } from 'react'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { Download, CheckCircle2, XCircle, RefreshCw, ShieldCheck, AlertTriangle, Pencil } from 'lucide-react'
import { Button } from '../../../components/ui/button'
import { Dialog, DialogContent, DialogHeader, DialogTitle, DialogFooter } from '../../../components/ui/dialog'
import { Label } from '../../../components/ui/label'
import { Textarea } from '../../../components/ui/textarea'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '../../../components/ui/select'
import {
  Table, TableBody, TableCell, TableHead, TableHeader, TableRow,
} from '../../../components/ui/table'
import { useTranslation } from 'react-i18next'
import { apiFetch } from '../../../api/client'
import { SkeletonTable } from '../../../shared/components/SkeletonLoaders'
import { ExportButton } from '../../../shared/components/ExportButton'
import { TermTooltip } from '../../../shared/components/TermTooltip'

// K5-01/02/03: these two interfaces are the wire contract, not a wish list. Every
// field name below IS the `json:` tag of the Go struct that serialises it —
// policy.SoADedicatedEntry and policy.SoASummary in
// backend/internal/modules/vaktcomply/policy/repository_soa_dedicated.go. The
// previous version invented `group`/`title`/`justification_included`/`owner`/
// `evidence_note`, which made the whole table filter to empty and made every
// save blank the justification. Keep in step with the Go structs.
// `python3 scripts/check_fe_be_fields.py` (G15) names this drift when you run it —
// it is NOT yet wired into ci.yml or `make gates` (both files are held by another
// branch), so as of this commit nothing runs it for you. Wiring lines: ADR-0080.
interface SoADedicatedEntry {
  control_ref: string
  control_name: string
  control_group: string
  applicable: boolean
  justification?: string
  exclusion_reason?: string
  implementation_status: string
  manually_set?: boolean
  ck_control_id?: string | null
  evidence_reference?: string
  notes?: string
}

interface SoADedicatedSummary {
  version: number
  status: string
  applicable_count: number
  excluded_count: number
  implemented_count: number
  partial_count: number
  planned_count: number
  not_started_count: number
  implementation_pct: number
}

// Only the four values the backend accepts: ck_soa_entries.implementation_status
// CHECK + `validate:"oneof=not_started planned partial implemented"`. The former
// `in_progress`/`not_applicable` options were rejected with 422.
const IMPL_STATUS_COLORS: Record<string, string> = {
  implemented: 'bg-green-100 text-green-800',
  planned: 'bg-blue-100 text-blue-800',
  not_started: 'bg-gray-100 text-gray-700',
  partial: 'bg-yellow-100 text-yellow-800',
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
  // Request type in the generics, not only in the mutationFn annotation — an
  // apiFetch/useQuery/useMutation type argument is where
  // scripts/check_fe_be_fields.py looks for a wire type.
  return useMutation<SoADedicatedEntry, Error, { ref: string; input: UpdateSoAEntryInput }>({
    mutationFn: ({ ref, input }) =>
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

// policy.UpdateSoAEntryInput — same name as the Go struct so the gate can pair
// them. The PUT is a FULL overwrite — the handler
// writes every one of these columns unconditionally (repository_soa_dedicated.go:
// UpdateSoAEntry), so a field the dialog does not carry through is a field the
// save DELETES. `notes` and `ck_control_id` are therefore round-tripped even
// though the dialog does not edit them.
interface UpdateSoAEntryInput {
  applicable: boolean
  justification: string
  exclusion_reason: string
  implementation_status: string
  manually_set: boolean
  ck_control_id: string | null
  evidence_reference: string
  notes: string
}

export default function SoAPage() {
  const { t } = useTranslation()
  const { data: entries, isLoading, isError } = useSoADedicated()
  const { data: summary } = useSoASummary()
  const initMut = useInitSoA()
  const updateMut = useUpdateSoAEntry()
  const approveMut = useApproveSoA()

  const [activeGroup, setActiveGroup] = useState<string>('5')
  const [editEntry, setEditEntry] = useState<SoADedicatedEntry | null>(null)
  const [editForm, setEditForm] = useState<UpdateSoAEntryInput>({
    applicable: true,
    justification: '',
    exclusion_reason: '',
    implementation_status: 'not_started',
    manually_set: true,
    ck_control_id: null,
    evidence_reference: '',
    notes: '',
  })

  const IMPL_STATUS_LABELS: Record<string, string> = {
    not_started: t('vaktcomply.soaPage.implNotStarted'),
    planned: t('vaktcomply.soaPage.implPlanned'),
    partial: t('vaktcomply.soaPage.implPartial'),
    implemented: t('vaktcomply.soaPage.implImplemented'),
  }

  const GROUP_LABELS: Record<string, string> = {
    '5': t('vaktcomply.soaPage.groupA5'),
    '6': t('vaktcomply.soaPage.groupA6'),
    '7': t('vaktcomply.soaPage.groupA7'),
    '8': t('vaktcomply.soaPage.groupA8'),
  }

  const notInitialized = isError || (entries && entries.length === 0)

  function openEdit(e: SoADedicatedEntry) {
    setEditEntry(e)
    setEditForm({
      applicable: e.applicable,
      justification: e.justification ?? '',
      exclusion_reason: e.exclusion_reason ?? '',
      implementation_status: e.implementation_status || 'not_started',
      // A save through this dialog IS a manual decision — without this the nightly
      // evidence sync (SyncSoAImplementationStatus) overwrites the user's status.
      manually_set: true,
      ck_control_id: e.ck_control_id ?? null,
      evidence_reference: e.evidence_reference ?? '',
      notes: e.notes ?? '',
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

  const grouped = (entries ?? []).filter(e => e.control_group === activeGroup)

  // K5-02: policy.SoASummary carries neither a total nor the "excluded without a
  // documented reason" count — the two numbers this header used to invent. Both
  // follow from the entry list the page already holds, so deriving them here
  // keeps the auditor warning (and the approve block) alive instead of rendering
  // `undefined`. The backend enforces the same rule server-side on approve
  // (ErrExclusionReasonRequired), so this is a hint, not the guard.
  const totalEntries = summary ? summary.applicable_count + summary.excluded_count : undefined
  const withoutExclusionReason = (entries ?? []).filter(
    e => !e.applicable && !(e.exclusion_reason ?? '').trim(),
  ).length

  if (isLoading) return <div className="p-8"><SkeletonTable rows={8} cols={5} /></div>

  if (notInitialized) {
    return (
      <div className="p-8 space-y-6">
        <div>
          <h1 className="text-2xl font-bold"><TermTooltip term="SoA" explanation="Statement of Applicability — ISO 27001:2022 Klausel 6.1.3: dokumentierte Aussage über die Anwendbarkeit aller 93 Maßnahmen aus Anhang A, inkl. Begründung für Ausschlüsse.">Statement of Applicability</TermTooltip></h1>
          <p className="text-gray-500 text-sm mt-1">ISO 27001:2022 {t('vaktcomply.soaPage.subtitle')}</p>
        </div>
        <div className="flex flex-col items-center justify-center bg-gray-50 border rounded-xl p-12 gap-4">
          <ShieldCheck className="h-12 w-12 text-gray-300" />
          <h2 className="text-lg font-semibold text-gray-700">{t('vaktcomply.soaPage.notInitTitle')}</h2>
          <p className="text-sm text-gray-500 text-center max-w-sm">
            {t('vaktcomply.soaPage.notInitDesc')}
          </p>
          <Button onClick={() => { initMut.mutate(); }} disabled={initMut.isPending}>
            {initMut.isPending ? t('vaktcomply.soaPage.initializingBtn') : t('vaktcomply.soaPage.initializeBtn')}
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
          <p className="text-gray-500 text-sm mt-1">ISO 27001:2022 {t('vaktcomply.soaPage.subtitleShort')}</p>
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
            label={t('common.word')}
            format="docx"
          />
          {summary?.status === 'draft' && (
            <Button
              size="sm"
              onClick={() => { approveMut.mutate(); }}
              disabled={approveMut.isPending || withoutExclusionReason > 0}
              className="bg-green-600 hover:bg-green-700 text-white"
              title={withoutExclusionReason > 0 ? t('vaktcomply.soaPage.approveBlockedTitle') : undefined}
            >
              <CheckCircle2 className="h-4 w-4 mr-1.5" />
              {approveMut.isPending ? t('vaktcomply.soaPage.approvingBtn') : t('vaktcomply.soaPage.approveBtn')}
            </Button>
          )}
        </div>
      </div>

      {/* Version banner */}
      {summary && (
        <div className={`flex items-center gap-3 px-4 py-3 rounded-lg border text-sm ${
          summary.status === 'approved'
            ? 'bg-green-50 border-green-200 text-green-800'
            : 'bg-amber-50 border-amber-200 text-amber-800'
        }`}>
          {summary.status === 'approved'
            ? <CheckCircle2 className="h-4 w-4 shrink-0" />
            : <RefreshCw className="h-4 w-4 shrink-0" />}
          <span>
            {summary.status === 'approved'
              ? t('vaktcomply.soaPage.versionApproved', { version: summary.version })
              : t('vaktcomply.soaPage.versionDraft', { version: summary.version })}
          </span>
          {withoutExclusionReason > 0 && (
            <span className="ml-auto flex items-center gap-1 text-amber-700">
              <AlertTriangle className="h-4 w-4" />
              {t('vaktcomply.soaPage.exclusionsWithoutReason', { count: withoutExclusionReason })}
            </span>
          )}
        </div>
      )}

      {/* Stats */}
      <div className="grid grid-cols-4 gap-4">
        <div className="bg-white border rounded-lg p-4">
          <div className="text-2xl font-bold">{totalEntries ?? 93}</div>
          <div className="text-xs text-gray-500 mt-0.5">{t('vaktcomply.soaPage.statTotal')}</div>
        </div>
        <div className="bg-green-50 border border-green-200 rounded-lg p-4">
          <div className="text-2xl font-bold text-green-700">{summary?.applicable_count ?? 0}</div>
          <div className="text-xs text-green-600 mt-0.5">{t('vaktcomply.soaPage.statApplicable')}</div>
        </div>
        <div className="bg-gray-50 border rounded-lg p-4">
          <div className="text-2xl font-bold text-gray-500">{summary?.excluded_count ?? 0}</div>
          <div className="text-xs text-gray-500 mt-0.5">{t('vaktcomply.soaPage.statExcluded')}</div>
        </div>
        <div className="bg-amber-50 border border-amber-200 rounded-lg p-4">
          <div className="text-2xl font-bold text-amber-700">{withoutExclusionReason}</div>
          <div className="text-xs text-amber-600 mt-0.5">{t('vaktcomply.soaPage.statWithoutReason')}</div>
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
              <TableHead className="w-20">{t('vaktcomply.soaPage.colRef')}</TableHead>
              <TableHead>{t('vaktcomply.soaPage.colMeasure')}</TableHead>
              <TableHead className="w-32">{t('vaktcomply.soaPage.colStatus')}</TableHead>
              <TableHead className="w-24 text-center">{t('vaktcomply.soaPage.colApplicable')}</TableHead>
              <TableHead className="w-16"></TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {grouped.length === 0 && (
              <TableRow>
                <TableCell colSpan={5} className="text-center text-gray-400 py-8">{t('vaktcomply.soaPage.noEntries')}</TableCell>
              </TableRow>
            )}
            {grouped.map(e => (
              <TableRow key={e.control_ref} className="hover:bg-gray-50">
                <TableCell className="font-mono text-xs text-gray-500">{e.control_ref}</TableCell>
                <TableCell>
                  <div className="text-sm font-medium">{e.control_name}</div>
                  {e.applicable && e.justification && (
                    <div className="text-xs text-gray-400 mt-0.5 line-clamp-1">{e.justification}</div>
                  )}
                  {!e.applicable && e.exclusion_reason && (
                    <div className="text-xs text-gray-400 mt-0.5 line-clamp-1 italic">{e.exclusion_reason}</div>
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
              {editEntry?.control_ref} — {editEntry?.control_name}
            </DialogTitle>
          </DialogHeader>
          <div className="space-y-4 py-2">
            <div className="space-y-1.5">
              <Label>{t('vaktcomply.soaPage.editLabelApplicable')}</Label>
              <Select
                value={editForm.applicable ? 'yes' : 'no'}
                onValueChange={(v) => { setEditForm(f => ({ ...f, applicable: v === 'yes' })); }}
              >
                <SelectTrigger>
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="yes">{t('vaktcomply.soaPage.editApplicableYes')}</SelectItem>
                  <SelectItem value="no">{t('vaktcomply.soaPage.editApplicableNo')}</SelectItem>
                </SelectContent>
              </Select>
            </div>
            {editForm.applicable ? (
              <>
                <div className="space-y-1.5">
                  <Label>{t('vaktcomply.soaPage.editLabelJustificationIncluded')}</Label>
                  <Textarea
                    rows={2}
                    placeholder={t('vaktcomply.soaPage.editPlaceholderJustificationIncluded')}
                    value={editForm.justification}
                    onChange={(e) => { setEditForm(f => ({ ...f, justification: e.target.value })); }}
                  />
                </div>
                <div className="space-y-1.5">
                  <Label>{t('vaktcomply.soaPage.editLabelImplStatus')}</Label>
                  <Select
                    value={editForm.implementation_status}
                    onValueChange={(v) => { setEditForm(f => ({ ...f, implementation_status: v })); }}
                  >
                    <SelectTrigger><SelectValue /></SelectTrigger>
                    <SelectContent>
                      {Object.entries(IMPL_STATUS_LABELS).map(([v, l]) => (
                        <SelectItem key={v} value={v}>{l}</SelectItem>
                      ))}
                    </SelectContent>
                  </Select>
                </div>
                {/* ck_soa_entries has no owner column and UpdateSoAEntryInput binds
                    no owner field — the input that used to sit here discarded
                    whatever was typed into it on every save (K5-03). Removed
                    rather than silently dropped; an owner column is a migration
                    and therefore a separate decision. */}
                <div className="space-y-1.5">
                  <Label>{t('vaktcomply.soaPage.editLabelEvidenceNote')}</Label>
                  <Textarea
                    rows={2}
                    placeholder={t('vaktcomply.soaPage.editPlaceholderEvidenceNote')}
                    value={editForm.evidence_reference}
                    onChange={(e) => { setEditForm(f => ({ ...f, evidence_reference: e.target.value })); }}
                  />
                </div>
              </>
            ) : (
              <div className="space-y-1.5">
                <Label>{t('vaktcomply.soaPage.editLabelJustificationExcluded')}</Label>
                <Textarea
                  rows={3}
                  placeholder={t('vaktcomply.soaPage.editPlaceholderJustificationExcluded')}
                  value={editForm.exclusion_reason}
                  onChange={(e) => { setEditForm(f => ({ ...f, exclusion_reason: e.target.value })); }}
                />
              </div>
            )}
          </div>
          <DialogFooter>
            <Button variant="outline" onClick={() => { setEditEntry(null); }}>{t('common.cancel')}</Button>
            <Button onClick={handleSaveEdit} disabled={updateMut.isPending}>
              {updateMut.isPending ? t('vaktcomply.soaPage.savingBtn') : t('common.save')}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </div>
  )
}
