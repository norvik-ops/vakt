/**
 * ScheduledReportsPage — geplante Berichte verwalten.
 * Backend: /api/v1/reports/scheduled
 */
import { useState } from 'react'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { Plus, Pencil, Trash2, Play, Calendar, X } from 'lucide-react'
import { PageHeader } from '../shared/components/PageHeader'
import { Button } from '../components/ui/button'
import { Input } from '../components/ui/input'
import { Label } from '../components/ui/label'
import { Badge } from '../components/ui/badge'
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
  DialogFooter,
} from '../components/ui/dialog'
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
} from '../components/ui/alert-dialog'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '../components/ui/select'
import { apiFetch } from '../api/client'
import { toast } from '../shared/hooks/useToast'

// ─── Types ────────────────────────────────────────────────────────────────────

type ReportType = 'compliance' | 'findings' | 'risk' | 'board_report'
type Schedule = 'weekly' | 'monthly' | 'quarterly'
type Format = 'pdf' | 'csv'

interface ScheduledReport {
  id: string
  name: string
  type: ReportType
  schedule: Schedule
  recipients: string[]
  format: Format
  next_run_at: string | null
  last_run_at: string | null
  created_at: string
}

type CreateScheduledReportInput = Omit<ScheduledReport, 'id' | 'created_at' | 'next_run_at' | 'last_run_at'>

// ─── API hooks ────────────────────────────────────────────────────────────────

const BASE = '/reports/scheduled'

function useScheduledReports() {
  return useQuery<ScheduledReport[]>({
    queryKey: ['scheduled-reports'],
    queryFn: () => apiFetch<ScheduledReport[]>(BASE),
    staleTime: 30_000,
  })
}

function useCreateScheduledReport() {
  const qc = useQueryClient()
  return useMutation<ScheduledReport, Error, CreateScheduledReportInput>({
    mutationFn: (data) =>
      apiFetch<ScheduledReport>(BASE, {
        method: 'POST',
        body: JSON.stringify(data),
      }),
    onSuccess: () => void qc.invalidateQueries({ queryKey: ['scheduled-reports'] }),
  })
}

function useUpdateScheduledReport() {
  const qc = useQueryClient()
  return useMutation<ScheduledReport, Error, { id: string } & CreateScheduledReportInput>({
    mutationFn: ({ id, ...data }) =>
      apiFetch<ScheduledReport>(`${BASE}/${id}`, {
        method: 'PUT',
        body: JSON.stringify(data),
      }),
    onSuccess: () => void qc.invalidateQueries({ queryKey: ['scheduled-reports'] }),
  })
}

function useDeleteScheduledReport() {
  const qc = useQueryClient()
  return useMutation<void, Error, string>({
    mutationFn: (id) =>
      apiFetch<void>(`${BASE}/${id}`, { method: 'DELETE' }),
    onSuccess: () => void qc.invalidateQueries({ queryKey: ['scheduled-reports'] }),
  })
}

function useRunReport() {
  return useMutation<void, Error, string>({
    mutationFn: (id) =>
      apiFetch<void>(`${BASE}/${id}/run`, { method: 'POST' }),
  })
}

// ─── Labels ───────────────────────────────────────────────────────────────────

const REPORT_TYPE_LABELS: Record<ReportType, string> = {
  compliance:   'Compliance-Übersicht',
  findings:     'Findings-Report',
  risk:         'Risk-Report',
  board_report: 'Management Board-Bericht (PDF)',
}

const SCHEDULE_LABELS: Record<Schedule, string> = {
  weekly:    'Wöchentlich (Montag)',
  monthly:   'Monatlich (1. des Monats)',
  quarterly: 'Vierteljährlich',
}

const FORMAT_LABELS: Record<Format, string> = {
  pdf: 'PDF',
  csv: 'CSV',
}

// ─── Date helpers ─────────────────────────────────────────────────────────────

function formatDate(iso: string | null) {
  if (!iso) return '—'
  return new Date(iso).toLocaleDateString('de-DE', {
    day: '2-digit',
    month: '2-digit',
    year: 'numeric',
  })
}

// ─── Chips input for email addresses ──────────────────────────────────────────

interface ChipsInputProps {
  value: string[]
  onChange: (v: string[]) => void
}

function ChipsInput({ value, onChange }: ChipsInputProps) {
  const [input, setInput] = useState('')

  function addChip() {
    const trimmed = input.trim()
    if (!trimmed || value.includes(trimmed)) { setInput(''); return }
    onChange([...value, trimmed])
    setInput('')
  }

  function handleKeyDown(e: React.KeyboardEvent<HTMLInputElement>) {
    if (e.key === 'Enter' || e.key === ',') {
      e.preventDefault()
      addChip()
    }
    if (e.key === 'Backspace' && !input && value.length > 0) {
      onChange(value.slice(0, -1))
    }
  }

  function removeChip(chip: string) {
    onChange(value.filter((v) => v !== chip))
  }

  return (
    <div className="min-h-[38px] border border-border rounded-md px-2 py-1 flex flex-wrap gap-1 focus-within:ring-2 focus-within:ring-brand/30 bg-background">
      {value.map((chip) => (
        <span
          key={chip}
          className="flex items-center gap-1 bg-brand/10 text-brand text-xs px-2 py-0.5 rounded-full"
        >
          {chip}
          <button
            type="button"
            onClick={() => removeChip(chip)}
            className="hover:text-red-500 transition-colors"
            aria-label={`${chip} entfernen`}
          >
            <X className="w-3 h-3" />
          </button>
        </span>
      ))}
      <input
        type="email"
        value={input}
        onChange={(e) => setInput(e.target.value)}
        onKeyDown={handleKeyDown}
        onBlur={addChip}
        placeholder={value.length === 0 ? 'E-Mail eingeben, Enter drücken' : ''}
        className="flex-1 min-w-[160px] text-sm outline-none bg-transparent placeholder:text-muted-foreground"
      />
    </div>
  )
}

// ─── Report Form Dialog ───────────────────────────────────────────────────────

interface ReportDialogProps {
  open: boolean
  onClose: () => void
  initial?: ScheduledReport
  onSave: (data: CreateScheduledReportInput) => void
  isSaving?: boolean
}

const emptyForm: CreateScheduledReportInput = {
  name: '',
  type: 'compliance',
  schedule: 'monthly',
  recipients: [],
  format: 'pdf',
}

function ReportDialog({ open, onClose, initial, onSave, isSaving }: ReportDialogProps) {
  const [form, setForm] = useState<CreateScheduledReportInput>(() =>
    initial
      ? { name: initial.name, type: initial.type, schedule: initial.schedule, recipients: initial.recipients, format: initial.format }
      : { ...emptyForm }
  )

  function handleSave() {
    if (!form.name.trim() || form.recipients.length === 0) return
    onSave(form)
  }

  function handleOpenChange(v: boolean) {
    if (!v) onClose()
  }

  const isEdit = !!initial
  const canSave = form.name.trim() !== '' && form.recipients.length > 0

  return (
    <Dialog open={open} onOpenChange={handleOpenChange}>
      <DialogContent className="sm:max-w-lg">
        <DialogHeader>
          <DialogTitle>{isEdit ? 'Bericht bearbeiten' : 'Bericht planen'}</DialogTitle>
        </DialogHeader>

        <div className="space-y-4 py-2">
          <div className="space-y-1.5">
            <Label htmlFor="rep-name">Name des Berichts</Label>
            <Input
              id="rep-name"
              value={form.name}
              onChange={(e) => setForm({ ...form, name: e.target.value })}
              placeholder="z.B. Monatlicher Compliance-Bericht"
            />
          </div>

          <div className="space-y-1.5">
            <Label>Typ</Label>
            <Select
              value={form.type}
              onValueChange={(v) => setForm({ ...form, type: v as ReportType })}
            >
              <SelectTrigger>
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                {(Object.entries(REPORT_TYPE_LABELS) as [ReportType, string][]).map(([v, l]) => (
                  <SelectItem key={v} value={v}>{l}</SelectItem>
                ))}
              </SelectContent>
            </Select>
          </div>

          <div className="space-y-1.5">
            <Label>Zeitplan</Label>
            <Select
              value={form.schedule}
              onValueChange={(v) => setForm({ ...form, schedule: v as Schedule })}
            >
              <SelectTrigger>
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                {(Object.entries(SCHEDULE_LABELS) as [Schedule, string][]).map(([v, l]) => (
                  <SelectItem key={v} value={v}>{l}</SelectItem>
                ))}
              </SelectContent>
            </Select>
          </div>

          <div className="space-y-1.5">
            <Label>Empfänger</Label>
            <ChipsInput
              value={form.recipients}
              onChange={(v) => setForm({ ...form, recipients: v })}
            />
            <p className="text-[11px] text-secondary">
              E-Mail eingeben und Enter drücken. Mehrere Empfänger möglich.
            </p>
          </div>

          <div className="space-y-1.5">
            <Label>Format</Label>
            <Select
              value={form.format}
              onValueChange={(v) => setForm({ ...form, format: v as Format })}
            >
              <SelectTrigger>
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                {(Object.entries(FORMAT_LABELS) as [Format, string][]).map(([v, l]) => (
                  <SelectItem key={v} value={v}>{l}</SelectItem>
                ))}
              </SelectContent>
            </Select>
          </div>

          {(!form.name.trim() || form.recipients.length === 0) && (
            <p className="text-[11px] text-amber-600">
              Name und mindestens ein Empfänger sind erforderlich.
            </p>
          )}
        </div>

        <DialogFooter>
          <Button variant="outline" onClick={onClose}>Abbrechen</Button>
          <Button onClick={handleSave} disabled={!canSave || isSaving}>
            {isSaving ? (
              <>
                <div className="w-3 h-3 border-2 border-current border-t-transparent rounded-full animate-spin mr-1.5" />
                Wird gespeichert…
              </>
            ) : 'Speichern'}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}

// ─── Report Card ──────────────────────────────────────────────────────────────

interface ReportCardProps {
  report: ScheduledReport
  onEdit: () => void
  onDelete: () => void
  onRunNow: () => void
  isRunning?: boolean
}

function ReportCard({ report, onEdit, onDelete, onRunNow, isRunning }: ReportCardProps) {
  return (
    <div className="bg-surface border border-border rounded-xl p-5 flex flex-col gap-3 hover:border-brand/30 transition-colors">
      <div className="flex items-start justify-between gap-2">
        <div>
          <h3 className="font-semibold text-primary text-sm">{report.name}</h3>
          <p className="text-xs text-secondary mt-0.5">{REPORT_TYPE_LABELS[report.type]}</p>
        </div>
        <div className="flex items-center gap-1 shrink-0">
          <Button
            size="sm"
            variant="ghost"
            className="h-7 w-7 p-0"
            title="Jetzt senden"
            onClick={onRunNow}
            disabled={isRunning}
          >
            <Play className="w-3.5 h-3.5" aria-hidden="true" />
            <span className="sr-only">Jetzt senden</span>
          </Button>
          <Button
            size="sm"
            variant="ghost"
            className="h-7 w-7 p-0"
            title="Bearbeiten"
            onClick={onEdit}
          >
            <Pencil className="w-3.5 h-3.5" aria-hidden="true" />
            <span className="sr-only">Bearbeiten</span>
          </Button>
          <Button
            size="sm"
            variant="ghost"
            className="h-7 w-7 p-0 text-secondary hover:text-red-500 hover:bg-red-500/10"
            title="Löschen"
            onClick={onDelete}
          >
            <Trash2 className="w-3.5 h-3.5" aria-hidden="true" />
            <span className="sr-only">Löschen</span>
          </Button>
        </div>
      </div>

      <div className="grid grid-cols-2 gap-x-4 gap-y-1.5 text-xs">
        <div className="text-secondary">Zeitplan</div>
        <div className="text-primary font-medium">{SCHEDULE_LABELS[report.schedule]}</div>

        <div className="text-secondary">Nächste Ausführung</div>
        <div className="text-primary font-medium">{formatDate(report.next_run_at)}</div>

        <div className="text-secondary">Letzte Ausführung</div>
        <div className="text-primary font-medium">{formatDate(report.last_run_at)}</div>

        <div className="text-secondary">Format</div>
        <div>
          <Badge variant="secondary" className="text-[10px]">{FORMAT_LABELS[report.format]}</Badge>
        </div>

        <div className="text-secondary">Empfänger</div>
        <div className="flex flex-wrap gap-1">
          {report.recipients.map((r) => (
            <span key={r} className="text-[10px] bg-surface2 border border-border rounded px-1.5 py-0.5 text-primary">
              {r}
            </span>
          ))}
        </div>
      </div>
    </div>
  )
}

// ─── Empty State ──────────────────────────────────────────────────────────────

function EmptyReports({ onCreate }: { onCreate: () => void }) {
  return (
    <div className="flex flex-col items-center justify-center py-20 gap-4">
      <div className="p-4 rounded-full bg-surface2">
        <Calendar className="w-8 h-8 text-secondary" aria-hidden="true" />
      </div>
      <div className="text-center">
        <p className="font-semibold text-primary">Noch keine geplanten Berichte</p>
        <p className="text-sm text-secondary mt-1 max-w-sm">
          Planen Sie regelmäßige Berichte, die automatisch per E-Mail versendet werden.
        </p>
      </div>
      <Button onClick={onCreate}>
        <Plus className="w-4 h-4 mr-1.5" />
        Bericht planen
      </Button>
    </div>
  )
}

// ─── Main Page ────────────────────────────────────────────────────────────────

export default function ScheduledReportsPage() {
  const { data: reports, isLoading, isError } = useScheduledReports()
  const createReport = useCreateScheduledReport()
  const updateReport = useUpdateScheduledReport()
  const deleteReport = useDeleteScheduledReport()
  const runReport = useRunReport()

  const [dialogOpen, setDialogOpen] = useState(false)
  const [editTarget, setEditTarget] = useState<ScheduledReport | undefined>()
  const [deleteTarget, setDeleteTarget] = useState<ScheduledReport | null>(null)
  const [runningId, setRunningId] = useState<string | null>(null)

  function openCreate() {
    setEditTarget(undefined)
    setDialogOpen(true)
  }

  function openEdit(r: ScheduledReport) {
    setEditTarget(r)
    setDialogOpen(true)
  }

  async function handleSave(data: CreateScheduledReportInput) {
    if (editTarget) {
      try {
        await updateReport.mutateAsync({ id: editTarget.id, ...data })
        toast('Bericht aktualisiert', 'success')
        setDialogOpen(false)
        setEditTarget(undefined)
      } catch (err) {
        toast(err instanceof Error ? err.message : 'Speichern fehlgeschlagen', 'error')
      }
    } else {
      try {
        await createReport.mutateAsync(data)
        toast('Bericht geplant', 'success')
        setDialogOpen(false)
      } catch (err) {
        toast(err instanceof Error ? err.message : 'Erstellen fehlgeschlagen', 'error')
      }
    }
  }

  async function handleDelete() {
    if (!deleteTarget) return
    try {
      await deleteReport.mutateAsync(deleteTarget.id)
      toast('Bericht gelöscht', 'success')
    } catch (err) {
      toast(err instanceof Error ? err.message : 'Löschen fehlgeschlagen', 'error')
    } finally {
      setDeleteTarget(null)
    }
  }

  async function handleRunNow(report: ScheduledReport) {
    setRunningId(report.id)
    try {
      await runReport.mutateAsync(report.id)
      toast(`„${report.name}" wurde zur sofortigen Ausführung eingeplant`, 'success')
    } catch (err) {
      toast(err instanceof Error ? err.message : 'Ausführung fehlgeschlagen', 'error')
    } finally {
      setRunningId(null)
    }
  }

  const reportList = reports ?? []

  return (
    <div className="flex flex-col h-full">
      <PageHeader
        title="Geplante Berichte"
        description="Automatisch versendete Compliance- und Sicherheitsberichte."
        actions={
          reportList.length > 0 ? (
            <Button onClick={openCreate}>
              <Plus className="w-4 h-4 mr-1.5" />
              Bericht planen
            </Button>
          ) : undefined
        }
      />

      <div className="flex-1 p-6 overflow-auto">
        {isLoading && (
          <div className="flex items-center justify-center h-32">
            <div className="w-5 h-5 border-2 border-brand border-t-transparent rounded-full animate-spin" />
          </div>
        )}

        {isError && (
          <p className="text-sm text-red-500">
            Fehler beim Laden der Berichte.
          </p>
        )}

        {!isLoading && !isError && reportList.length === 0 && (
          <EmptyReports onCreate={openCreate} />
        )}

        {!isLoading && !isError && reportList.length > 0 && (
          <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4 max-w-5xl">
            {reportList.map((r) => (
              <ReportCard
                key={r.id}
                report={r}
                onEdit={() => openEdit(r)}
                onDelete={() => setDeleteTarget(r)}
                onRunNow={() => { void handleRunNow(r) }}
                isRunning={runningId === r.id}
              />
            ))}
          </div>
        )}
      </div>

      {/* Create / Edit Dialog */}
      {dialogOpen && (
        <ReportDialog
          open={dialogOpen}
          onClose={() => { setDialogOpen(false); setEditTarget(undefined) }}
          initial={editTarget}
          onSave={(data) => { void handleSave(data) }}
          isSaving={createReport.isPending || updateReport.isPending}
        />
      )}

      {/* Delete Confirmation */}
      <AlertDialog open={!!deleteTarget} onOpenChange={(v) => { if (!v) setDeleteTarget(null) }}>
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>Bericht löschen?</AlertDialogTitle>
            <AlertDialogDescription>
              Der geplante Bericht <strong>{deleteTarget?.name}</strong> wird dauerhaft entfernt.
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel>Abbrechen</AlertDialogCancel>
            <AlertDialogAction
              onClick={() => { void handleDelete() }}
              className="bg-red-600 hover:bg-red-700 focus:ring-red-600"
              disabled={deleteReport.isPending}
            >
              {deleteReport.isPending ? 'Wird gelöscht…' : 'Löschen'}
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </div>
  )
}
