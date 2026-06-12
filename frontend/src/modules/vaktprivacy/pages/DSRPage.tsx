import { useState } from 'react'
import { Users, Plus, Pencil, Trash2, AlertTriangle, Download, ShieldCheck, CheckCircle2 } from 'lucide-react'
import { Spinner } from '../../../components/Spinner'
import { Button } from '../../../components/ui/button'
import { Card, CardContent } from '../../../components/ui/card'
import { Badge } from '../../../components/ui/badge'
import { Dialog, DialogContent, DialogHeader, DialogTitle, DialogFooter } from '../../../components/ui/dialog'
import { AlertDialog, AlertDialogContent, AlertDialogHeader, AlertDialogTitle, AlertDialogDescription, AlertDialogFooter, AlertDialogCancel, AlertDialogAction } from '../../../components/ui/alert-dialog'
import { Input } from '../../../components/ui/input'
import { Label } from '../../../components/ui/label'
import { Textarea } from '../../../components/ui/textarea'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '../../../components/ui/select'
import { PageHeader } from '../../../shared/components/PageHeader'
import { EmptyState } from '../../../shared/components/EmptyState'
import { InfoBanner } from '../../../shared/components/InfoBanner'
import { TermTooltip } from '../../../shared/components/TermTooltip'
import { useDSRs, useCreateDSR, useUpdateDSR, useDeleteDSR, useDSRSummary, useResolveDSR } from '../hooks/useDSRs'
import { ComplianceTooltip } from '../../../shared/components/ComplianceTooltip'
import type { DSR, DSRType, DSRStatus, CreateDSRInput, UpdateDSRInput, ResolveDSRInput } from '../types'
import { useFormatDate } from '../../../shared/hooks/useFormatDate'

const TYPE_LABELS: Record<DSRType, string> = {
  access: 'Auskunft (Art. 15)',
  erasure: 'Löschung (Art. 17)',
  portability: 'Datenübertragbarkeit (Art. 20)',
  objection: 'Widerspruch (Art. 21)',
  rectification: 'Berichtigung (Art. 16)',
  restriction: 'Einschränkung (Art. 18)',
  no_profiling: 'Kein Profiling (Art. 22)',
}

const STATUS_LABELS: Record<DSRStatus, string> = {
  open: 'Offen',
  in_progress: 'In Bearbeitung',
  completed: 'Abgeschlossen',
  rejected: 'Abgelehnt',
  extended: 'Verlängert',
  overdue: 'Überfällig',
}

const STATUS_CLASS: Record<DSRStatus, string> = {
  open: 'bg-yellow-500/20 text-yellow-400 border-yellow-500/30',
  in_progress: 'bg-blue-500/20 text-blue-400 border-blue-500/30',
  completed: 'bg-green-500/20 text-green-400 border-green-500/30',
  rejected: 'bg-secondary text-secondary-foreground',
  extended: 'bg-purple-500/20 text-purple-400 border-purple-500/30',
  overdue: 'bg-red-500/20 text-red-400 border-red-500/30',
}

function isOverdue(dueDate?: string): boolean {
  if (!dueDate) return false
  return new Date(dueDate) < new Date()
}

function daysUntil(dateStr?: string): number | null {
  if (!dateStr) return null
  const diff = new Date(dateStr).getTime() - Date.now()
  return Math.ceil(diff / (1000 * 60 * 60 * 24))
}

interface CreateFormState {
  requester_name: string
  requester_email: string
  type: DSRType
  description: string
  notes: string
  channel: string
  reference_id: string
}

interface EditFormState {
  status: DSRStatus
  notes: string
}

function emptyCreateForm(): CreateFormState {
  return {
    requester_name: '',
    requester_email: '',
    type: 'access',
    description: '',
    notes: '',
    channel: '',
    reference_id: '',
  }
}

function DSRCard({
  dsr,
  onEdit,
  onDelete,
  onErasure,
  onResolve,
}: {
  dsr: DSR
  onEdit: (d: DSR) => void
  onDelete: (id: string) => void
  onErasure?: (id: string) => void
  onResolve: (d: DSR) => void
}) {
  const { formatDate } = useFormatDate()
  const overdue = isOverdue(dsr.due_date) && dsr.status !== 'completed' && dsr.status !== 'rejected' && dsr.status !== 'extended'
  const days = daysUntil(dsr.due_date)
  const receivedDate = formatDate(dsr.received_at, { year: 'numeric', month: 'short', day: 'numeric' })

  return (
    <Card className={overdue || dsr.status === 'overdue' ? 'border-red-500/30' : ''}>
      <CardContent className="pt-5 space-y-2">
        <div className="flex items-start justify-between gap-2">
          <div className="min-w-0">
            <p className="font-medium text-sm truncate">{dsr.requester_name}</p>
            <p className="text-xs text-muted-foreground truncate">{dsr.requester_email}</p>
          </div>
          <Badge className={STATUS_CLASS[dsr.status]}>{STATUS_LABELS[dsr.status]}</Badge>
        </div>

        <div className="flex flex-wrap gap-1.5">
          <Badge variant="outline" className="text-xs font-normal">{TYPE_LABELS[dsr.type]}</Badge>
          {dsr.channel && <Badge variant="outline" className="text-xs font-normal text-gray-400">{dsr.channel}</Badge>}
        </div>

        {dsr.description && (
          <p className="text-xs text-muted-foreground line-clamp-2">{dsr.description}</p>
        )}

        <div className="flex flex-wrap gap-3 text-xs text-muted-foreground">
          <span>Eingegangen: {receivedDate}</span>
          {dsr.due_date && (
            <span className={overdue ? 'text-red-400 font-medium' : days !== null && days <= 7 ? 'text-amber-500 font-medium' : ''}>
              {overdue ? (
                <><AlertTriangle className="w-3 h-3 inline mr-0.5" />Frist abgelaufen</>
              ) : days !== null ? (
                <>Frist: {days > 0 ? `noch ${days}d` : 'heute'} ({formatDate(dsr.due_date)})</>
              ) : null}
            </span>
          )}
          {dsr.extension_due_at && dsr.status === 'extended' && (
            <span className="text-purple-400">
              Verlängerung bis: {formatDate(dsr.extension_due_at)}
            </span>
          )}
        </div>

        {dsr.notes && (
          <p className="text-xs text-muted-foreground italic line-clamp-1">{dsr.notes}</p>
        )}

        <div className="flex justify-end gap-1 pt-1 flex-wrap">
          {dsr.type === 'erasure' && dsr.status !== 'completed' && dsr.status !== 'rejected' && onErasure && (
            <Button
              size="sm"
              variant="outline"
              className="h-7 text-xs gap-1 text-green-400 border-green-500/30 hover:bg-green-500/10"
              onClick={() => { onErasure(dsr.id); }}
            >
              <ShieldCheck className="w-3.5 h-3.5" />
              Löschung
            </Button>
          )}
          {(dsr.status === 'open' || dsr.status === 'in_progress' || dsr.status === 'overdue') && (
            <Button
              size="sm"
              variant="outline"
              className="h-7 text-xs gap-1"
              onClick={() => { onResolve(dsr); }}
            >
              <CheckCircle2 className="w-3.5 h-3.5" />
              Abschließen
            </Button>
          )}
          <Button size="icon" variant="ghost" className="h-7 w-7" aria-label="Bearbeiten" onClick={() => { onEdit(dsr); }}>
            <Pencil className="w-3.5 h-3.5" />
          </Button>
          <Button
            size="icon"
            variant="ghost"
            className="h-7 w-7 text-destructive hover:text-destructive"
            aria-label="Löschen"
            onClick={() => { onDelete(dsr.id); }}
          >
            <Trash2 className="w-3.5 h-3.5" />
          </Button>
        </div>
      </CardContent>
    </Card>
  )
}

export default function DSRPage() {
  const [dialogMode, setDialogMode] = useState<'create' | 'edit' | null>(null)
  const [editId, setEditId] = useState<string | null>(null)
  const [createForm, setCreateForm] = useState<CreateFormState>(emptyCreateForm())
  const [editForm, setEditForm] = useState<EditFormState>({ status: 'open', notes: '' })
  const [deleteId, setDeleteId] = useState<string | null>(null)
  const [erasureId, setErasureId] = useState<string | null>(null)
  const [erasureNote, setErasureNote] = useState('')
  const [resolveTarget, setResolveTarget] = useState<DSR | null>(null)
  const [resolveForm, setResolveForm] = useState<ResolveDSRInput>({ resolution_type: 'completed', resolution_notes: '', extension_reason: '' })

  const { data: dsrs, isLoading, isError } = useDSRs()
  const { data: summary } = useDSRSummary()
  const createDSR = useCreateDSR()
  const updateDSR = useUpdateDSR()
  const deleteDSR = useDeleteDSR()
  const resolveDSR = useResolveDSR()

  function openCreate() {
    setCreateForm(emptyCreateForm())
    setEditId(null)
    setDialogMode('create')
  }

  function openEdit(dsr: DSR) {
    setEditForm({ status: dsr.status, notes: dsr.notes ?? '' })
    setEditId(dsr.id)
    setDialogMode('edit')
  }

  function openResolve(dsr: DSR) {
    setResolveTarget(dsr)
    setResolveForm({ resolution_type: 'completed', resolution_notes: '', extension_reason: '' })
  }

  function handleErasureOpen(id: string) {
    setErasureId(id)
    setErasureNote('')
  }

  function handleErasureConfirm() {
    if (!erasureId) return
    const id = erasureId
    setErasureId(null)
    updateDSR.mutate({
      id,
      input: { status: 'completed', notes: erasureNote || 'Löschung ausgeführt (Art. 17 DSGVO).' },
    })
  }

  function confirmDelete() {
    if (deleteId) deleteDSR.mutate(deleteId)
    setDeleteId(null)
  }

  function handleSubmit() {
    if (dialogMode === 'create') {
      const payload: CreateDSRInput = {
        requester_name: createForm.requester_name,
        requester_email: createForm.requester_email,
        type: createForm.type,
        description: createForm.description || undefined,
        notes: createForm.notes || undefined,
        channel: createForm.channel || undefined,
        reference_id: createForm.reference_id || undefined,
      }
      createDSR.mutate(payload, { onSuccess: () => { setDialogMode(null); } })
    } else if (dialogMode === 'edit' && editId) {
      const payload: UpdateDSRInput = {
        status: editForm.status,
        notes: editForm.notes || undefined,
      }
      updateDSR.mutate({ id: editId, input: payload }, { onSuccess: () => { setDialogMode(null); } })
    }
  }

  function handleResolve() {
    if (!resolveTarget) return
    resolveDSR.mutate(
      { id: resolveTarget.id, input: resolveForm },
      { onSuccess: () => { setResolveTarget(null); } },
    )
  }

  function handlePDFExport() {
    const a = document.createElement('a')
    a.href = '/api/v1/vaktprivacy/dsr/export?format=pdf'
    a.download = `dsr-auditlog-${new Date().toISOString().slice(0, 10)}.pdf`
    document.body.appendChild(a)
    a.click()
    a.remove()
  }

  const isPending = createDSR.isPending || updateDSR.isPending
  const canSubmitCreate = createForm.requester_name && createForm.requester_email && !isPending

  const openDSRs = dsrs?.filter((d) => d.status === 'open' || d.status === 'in_progress' || d.status === 'overdue' || d.status === 'extended') ?? []
  const closedDSRs = dsrs?.filter((d) => d.status === 'completed' || d.status === 'rejected') ?? []
  const overdueCount = summary?.overdue_count ?? openDSRs.filter(d => isOverdue(d.due_date)).length

  return (
    <div className="flex flex-col h-full">
      <PageHeader
        title="Datenschutzanfragen (DSR)"
        description="Art. 15–22 DSGVO — Verwaltung von Betroffenenrechten."
        actions={
          <div className="flex items-center gap-2">
            <Button variant="outline" onClick={handlePDFExport}>
              <Download className="w-4 h-4 mr-1" />
              Audit-PDF
            </Button>
            <Button variant="outline" onClick={() => {
              void fetch('/api/v1/vaktprivacy/dsr/export?format=csv', { credentials: 'include' })
                .then(res => res.blob())
                .then(blob => {
                  const url = URL.createObjectURL(blob)
                  const a = document.createElement('a')
                  a.href = url
                  a.download = `dsr-export-${new Date().toISOString().slice(0, 10)}.csv`
                  document.body.appendChild(a)
                  a.click()
                  a.remove()
                  URL.revokeObjectURL(url)
                })
            }}>
              <Download className="w-4 h-4 mr-1" />
              CSV
            </Button>
            <Button onClick={openCreate}>
              <Plus className="w-4 h-4 mr-1" />
              DSR anlegen
            </Button>
          </div>
        }
      />

      <InfoBanner icon={Users} title="Betroffenenrechte nach DSGVO (Art. 12)">
        <p><TermTooltip term="DSR" explanation="Data Subject Request — Betroffenenanfrage nach Art. 15–22 DSGVO: Auskunft, Berichtigung, Löschung, Einschränkung, Widerspruch, Datenübertragbarkeit.">Datenschutzanfragen</TermTooltip> müssen innerhalb von <strong>30 Tagen</strong> beantwortet werden — bei komplexen Anfragen maximal 60 Tage mit Begründung (Art. 12 Abs. 3 DSGVO).</p>
      </InfoBanner>

      {/* Summary stats */}
      {summary && (
        <div className="px-6 py-3 grid grid-cols-4 gap-3">
          <div className="bg-white border rounded-lg p-3 text-center">
            <div className="text-xl font-bold">{summary.open_count}</div>
            <div className="text-xs text-gray-500">Offen</div>
          </div>
          <div className={`border rounded-lg p-3 text-center ${summary.overdue_count > 0 ? 'bg-red-50 border-red-200' : 'bg-white'}`}>
            <div className={`text-xl font-bold ${summary.overdue_count > 0 ? 'text-red-600' : ''}`}>{summary.overdue_count}</div>
            <div className="text-xs text-gray-500">Überfällig</div>
          </div>
          <div className="bg-white border rounded-lg p-3 text-center">
            <div className="text-xl font-bold">{summary.fulfilled_last_12m}</div>
            <div className="text-xs text-gray-500">Erfüllt (12M)</div>
          </div>
          <div className="bg-white border rounded-lg p-3 text-center">
            <div className="text-xl font-bold">{summary.on_time_rate_pct}%</div>
            <div className="text-xs text-gray-500">Pünktlichkeit</div>
          </div>
        </div>
      )}

      <div className="flex-1 p-6 space-y-6">
        {isLoading && (
          <div className="flex items-center justify-center h-48">
            <Spinner size="lg" color="primary" />
          </div>
        )}

        {isError && (
          <div className="text-sm text-red-400 p-4 bg-red-500/10 rounded-lg">
            Fehler beim Laden der Datenschutzanfragen.
          </div>
        )}

        {!isLoading && !isError && dsrs?.length === 0 && (
          <EmptyState
            icon={Users}
            title="Keine Datenschutzanfragen"
            description="Dokumentieren Sie Betroffenenanfragen gemäß Art. 12-22 DSGVO und verfolgen Sie die 30-Tage-Frist."
            action={
              <Button onClick={openCreate}>
                <Plus className="w-4 h-4 mr-1" />
                DSR anlegen
              </Button>
            }
          />
        )}

        {!isLoading && !isError && dsrs && dsrs.length > 0 && (
          <>
            {overdueCount > 0 && (
              <div className="flex items-start gap-3 p-4 bg-red-500/10 border border-red-500/30 rounded-lg">
                <AlertTriangle className="w-5 h-5 text-red-500 shrink-0 mt-0.5" />
                <div>
                  <p className="text-sm font-semibold text-red-500">
                    {overdueCount} Anfrage{overdueCount > 1 ? 'n' : ''} — 30-Tage-Frist abgelaufen
                  </p>
                  <p className="text-xs text-secondary mt-0.5">
                    Anfragen müssen innerhalb von 30 Tagen beantwortet werden (Art. 12 DSGVO).
                  </p>
                </div>
              </div>
            )}

            {openDSRs.length > 0 && (
              <div className="space-y-3">
                <h2 className="text-sm font-semibold text-secondary">Offene Anfragen ({openDSRs.length})</h2>
                <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 gap-4">
                  {openDSRs.map((d) => (
                    <DSRCard key={d.id} dsr={d} onEdit={openEdit} onDelete={(id) => { setDeleteId(id); }} onErasure={handleErasureOpen} onResolve={openResolve} />
                  ))}
                </div>
              </div>
            )}

            {closedDSRs.length > 0 && (
              <div className="space-y-3">
                <h2 className="text-sm font-semibold text-muted-foreground">Abgeschlossene Anfragen</h2>
                <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 gap-4">
                  {closedDSRs.map((d) => (
                    <DSRCard key={d.id} dsr={d} onEdit={openEdit} onDelete={(id) => { setDeleteId(id); }} onErasure={handleErasureOpen} onResolve={openResolve} />
                  ))}
                </div>
              </div>
            )}
          </>
        )}
      </div>

      {/* Erasure Dialog */}
      <Dialog open={erasureId !== null} onOpenChange={(open) => { if (!open) { setErasureId(null); } }}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>Löschung bestätigen (Art. 17 DSGVO)</DialogTitle>
          </DialogHeader>
          <p className="text-sm text-muted-foreground">
            Bestätigen Sie, dass die Daten der betroffenen Person gelöscht wurden.
          </p>
          <div className="space-y-2">
            <Label htmlFor="erasure-note">Nachweis / Notiz</Label>
            <Textarea
              id="erasure-note"
              placeholder="z.B. Kundendatensätze in DB gelöscht, Backups werden nach 30 Tagen überschrieben."
              value={erasureNote}
              onChange={(e) => { setErasureNote(e.target.value); }}
              rows={3}
            />
          </div>
          <DialogFooter>
            <Button variant="ghost" onClick={() => { setErasureId(null); }}>Abbrechen</Button>
            <Button onClick={handleErasureConfirm} className="bg-green-600 hover:bg-green-700 text-white">
              <ShieldCheck className="w-4 h-4 mr-1.5" />
              Löschung bestätigen
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      {/* Resolve Dialog */}
      <Dialog open={resolveTarget !== null} onOpenChange={(open) => { if (!open) { setResolveTarget(null); } }}>
        <DialogContent className="max-w-lg">
          <DialogHeader>
            <DialogTitle>Anfrage abschließen — {resolveTarget?.requester_name}</DialogTitle>
          </DialogHeader>
          <div className="space-y-4 py-2">
            <div className="space-y-1.5">
              <Label>Ergebnis *</Label>
              <Select
                value={resolveForm.resolution_type}
                onValueChange={(v) => { setResolveForm(f => ({ ...f, resolution_type: v as DSRStatus })); }}
              >
                <SelectTrigger><SelectValue /></SelectTrigger>
                <SelectContent>
                  <SelectItem value="completed">Erfüllt</SelectItem>
                  <SelectItem value="rejected">Abgelehnt (mit Begründung)</SelectItem>
                  <SelectItem value="extended">Verlängert (+60 Tage, Art. 12 Abs. 3)</SelectItem>
                </SelectContent>
              </Select>
            </div>
            {resolveForm.resolution_type === 'extended' && (
              <div className="space-y-1.5">
                <Label>Begründung der Verlängerung *</Label>
                <Textarea
                  rows={2}
                  placeholder="Begründung (Pflicht bei Verlängerung nach Art. 12 Abs. 3 DSGVO)"
                  value={resolveForm.extension_reason}
                  onChange={(e) => { setResolveForm(f => ({ ...f, extension_reason: e.target.value })); }}
                />
              </div>
            )}
            <div className="space-y-1.5">
              <Label>Notizen / Nachweis</Label>
              <Textarea
                rows={3}
                placeholder="Maßnahmen, Kommentare …"
                value={resolveForm.resolution_notes}
                onChange={(e) => { setResolveForm(f => ({ ...f, resolution_notes: e.target.value })); }}
              />
            </div>
          </div>
          <DialogFooter>
            <Button variant="outline" onClick={() => { setResolveTarget(null); }}>Abbrechen</Button>
            <Button
              onClick={handleResolve}
              disabled={resolveDSR.isPending || (resolveForm.resolution_type === 'extended' && !resolveForm.extension_reason?.trim())}
            >
              {resolveDSR.isPending ? 'Speichern…' : 'Abschließen'}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      <AlertDialog open={deleteId !== null} onOpenChange={(open) => { if (!open) { setDeleteId(null); } }}>
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>Datenschutzanfrage löschen?</AlertDialogTitle>
            <AlertDialogDescription>Diese Aktion kann nicht rückgängig gemacht werden.</AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel onClick={() => { setDeleteId(null); }}>Abbrechen</AlertDialogCancel>
            <AlertDialogAction onClick={confirmDelete} className="bg-destructive text-destructive-foreground hover:bg-destructive/90">Löschen</AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>

      {/* Create Dialog */}
      <Dialog open={dialogMode === 'create'} onOpenChange={(open) => { if (!open) { setDialogMode(null); } }}>
        <DialogContent className="max-w-lg max-h-[90vh] overflow-y-auto">
          <DialogHeader>
            <DialogTitle><ComplianceTooltip term="dsr">Datenschutzanfrage anlegen</ComplianceTooltip></DialogTitle>
          </DialogHeader>
          <div className="space-y-4 py-2">
            <div className="p-3 rounded-lg bg-blue-500/10 text-blue-400 text-xs">
              Die 30-Tage-Antwortfrist beginnt ab heute (Art. 12 DSGVO).
            </div>
            <div className="space-y-1.5">
              <Label>Name der anfragenden Person *</Label>
              <Input
                placeholder="z.B. Max Mustermann"
                value={createForm.requester_name}
                onChange={(e) => { setCreateForm((f) => ({ ...f, requester_name: e.target.value })); }}
              />
            </div>
            <div className="space-y-1.5">
              <Label>E-Mail *</Label>
              <Input
                type="email"
                placeholder="max@example.com"
                value={createForm.requester_email}
                onChange={(e) => { setCreateForm((f) => ({ ...f, requester_email: e.target.value })); }}
              />
            </div>
            <div className="grid grid-cols-2 gap-3">
              <div className="space-y-1.5">
                <Label>Art der Anfrage *</Label>
                <Select
                  value={createForm.type}
                  onValueChange={(v) => { setCreateForm((f) => ({ ...f, type: v as DSRType })); }}
                >
                  <SelectTrigger><SelectValue /></SelectTrigger>
                  <SelectContent>
                    {(Object.entries(TYPE_LABELS) as [DSRType, string][]).map(([v, l]) => (
                      <SelectItem key={v} value={v}>{l}</SelectItem>
                    ))}
                  </SelectContent>
                </Select>
              </div>
              <div className="space-y-1.5">
                <Label>Eingangskanal</Label>
                <Input
                  placeholder="z.B. E-Mail, Portal"
                  value={createForm.channel}
                  onChange={(e) => { setCreateForm((f) => ({ ...f, channel: e.target.value })); }}
                />
              </div>
            </div>
            <div className="space-y-1.5">
              <Label>Referenz-ID</Label>
              <Input
                placeholder="Ticket-Nr., Fallnummer …"
                value={createForm.reference_id}
                onChange={(e) => { setCreateForm((f) => ({ ...f, reference_id: e.target.value })); }}
              />
            </div>
            <div className="space-y-1.5">
              <Label>Beschreibung</Label>
              <Textarea
                placeholder="Inhalt der Anfrage …"
                rows={3}
                value={createForm.description}
                onChange={(e) => { setCreateForm((f) => ({ ...f, description: e.target.value })); }}
              />
            </div>
            <div className="space-y-1.5">
              <Label>Interne Notizen</Label>
              <Textarea
                placeholder="Interne Anmerkungen …"
                rows={2}
                value={createForm.notes}
                onChange={(e) => { setCreateForm((f) => ({ ...f, notes: e.target.value })); }}
              />
            </div>
          </div>
          <DialogFooter>
            <Button variant="outline" onClick={() => { setDialogMode(null); }}>Abbrechen</Button>
            <Button onClick={handleSubmit} disabled={!canSubmitCreate}>
              {isPending ? 'Speichern …' : 'DSR anlegen'}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      {/* Edit Dialog */}
      <Dialog open={dialogMode === 'edit'} onOpenChange={(open) => { if (!open) { setDialogMode(null); } }}>
        <DialogContent className="max-w-lg">
          <DialogHeader>
            <DialogTitle>Status aktualisieren</DialogTitle>
          </DialogHeader>
          <div className="space-y-4 py-2">
            <div className="space-y-1.5">
              <Label>Status *</Label>
              <Select
                value={editForm.status}
                onValueChange={(v) => { setEditForm((f) => ({ ...f, status: v as DSRStatus })); }}
              >
                <SelectTrigger><SelectValue /></SelectTrigger>
                <SelectContent>
                  {(Object.entries(STATUS_LABELS) as [DSRStatus, string][]).map(([v, l]) => (
                    <SelectItem key={v} value={v}>{l}</SelectItem>
                  ))}
                </SelectContent>
              </Select>
            </div>
            <div className="space-y-1.5">
              <Label>Interne Notizen</Label>
              <Textarea
                placeholder="Begründung, Maßnahmen, Kommentare …"
                rows={3}
                value={editForm.notes}
                onChange={(e) => { setEditForm((f) => ({ ...f, notes: e.target.value })); }}
              />
            </div>
          </div>
          <DialogFooter>
            <Button variant="outline" onClick={() => { setDialogMode(null); }}>Abbrechen</Button>
            <Button onClick={handleSubmit} disabled={isPending}>
              {isPending ? 'Speichern …' : 'Speichern'}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </div>
  )
}
