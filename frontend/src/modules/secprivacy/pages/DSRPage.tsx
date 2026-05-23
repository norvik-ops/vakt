import { useState } from 'react'
import { Users, Plus, Pencil, Trash2, AlertTriangle, Download, ShieldCheck } from 'lucide-react'
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
import { useDSRs, useCreateDSR, useUpdateDSR, useDeleteDSR } from '../hooks/useDSRs'
import { ComplianceTooltip } from '../../../shared/components/ComplianceTooltip'
import type { DSR, DSRType, DSRStatus, CreateDSRInput, UpdateDSRInput } from '../types'
import { useFormatDate } from '../../../shared/hooks/useFormatDate'

/**
 * Human-readable German labels for each DSR type, including the governing DSGVO article.
 * Keys map to DSRType union values; values are displayed in the UI and form selects.
 */
const TYPE_LABELS: Record<DSRType, string> = {
  access: 'Auskunft (Art. 15)',
  erasure: 'Löschung (Art. 17)',
  portability: 'Datenübertragbarkeit (Art. 20)',
  objection: 'Widerspruch (Art. 21)',
  rectification: 'Berichtigung (Art. 16)',
}

/** German display labels for each DSR lifecycle status. */
const STATUS_LABELS: Record<DSRStatus, string> = {
  open: 'Offen',
  in_progress: 'In Bearbeitung',
  completed: 'Abgeschlossen',
  rejected: 'Abgelehnt',
}

/** Tailwind badge colour classes keyed by DSR status, matching the design system. */
const STATUS_CLASS: Record<DSRStatus, string> = {
  open: 'bg-yellow-500/20 text-yellow-400 border-yellow-500/30',
  in_progress: 'bg-blue-500/20 text-blue-400 border-blue-500/30',
  completed: 'bg-green-500/20 text-green-400 border-green-500/30',
  rejected: 'bg-secondary text-secondary-foreground',
}

/**
 * Returns true when the given ISO date string is in the past.
 * Used to highlight DSRs that have breached the 30-day response deadline (Art. 12 DSGVO).
 */
function isOverdue(dueDate?: string): boolean {
  if (!dueDate) return false
  return new Date(dueDate) < new Date()
}

/** Local form state for the "create DSR" dialog. */
interface CreateFormState {
  requester_name: string
  requester_email: string
  type: DSRType
  description: string
  notes: string
}

/** Local form state for the "edit DSR status" dialog — only mutable fields. */
interface EditFormState {
  status: DSRStatus
  notes: string
}

/** Returns a blank create-form state with the default type set to `access`. */
function emptyCreateForm(): CreateFormState {
  return {
    requester_name: '',
    requester_email: '',
    type: 'access',
    description: '',
    notes: '',
  }
}

/** Seeds the edit-form from an existing DSR so the dialog opens with current values. */
function emptyEditForm(dsr: DSR): EditFormState {
  return {
    status: dsr.status,
    notes: dsr.notes ?? '',
  }
}

/**
 * Card representation of a single DSR, showing requester, type badge, status,
 * the 30-day due-date countdown, and action buttons for edit and delete.
 * Renders a red border and an overdue warning when the deadline has passed and the
 * request is still open.
 */
function DSRCard({
  dsr,
  onEdit,
  onDelete,
  onErasure,
}: {
  dsr: DSR
  onEdit: (d: DSR) => void
  onDelete: (id: string) => void
  onErasure?: (id: string) => void
}) {
  const { formatDate } = useFormatDate()
  const overdue = isOverdue(dsr.due_date)
  const receivedDate = formatDate(dsr.received_at, { year: 'numeric', month: 'short', day: 'numeric' })

  return (
    <Card className={overdue && dsr.status === 'open' ? 'border-red-500/30' : ''}>
      <CardContent className="pt-5 space-y-2">
        <div className="flex items-start justify-between gap-2">
          <div className="min-w-0">
            <p className="font-medium text-sm truncate">{dsr.requester_name}</p>
            <p className="text-xs text-muted-foreground truncate">{dsr.requester_email}</p>
          </div>
          <Badge className={STATUS_CLASS[dsr.status]}>{STATUS_LABELS[dsr.status]}</Badge>
        </div>

        <Badge variant="outline" className="text-xs font-normal">
          {TYPE_LABELS[dsr.type]}
        </Badge>

        {dsr.description && (
          <p className="text-xs text-muted-foreground line-clamp-2">{dsr.description}</p>
        )}

        <div className="flex flex-wrap gap-3 text-xs text-muted-foreground">
          <span>Eingegangen: {receivedDate}</span>
          {dsr.due_date && (
            <span className={overdue && dsr.status !== 'completed' && dsr.status !== 'rejected' ? 'text-red-400 font-medium' : ''}>
              {overdue && dsr.status !== 'completed' && dsr.status !== 'rejected' ? (
                <><AlertTriangle className="w-3 h-3 inline mr-0.5" />Frist abgelaufen: {formatDate(dsr.due_date)}</>
              ) : (
                <>Frist: {formatDate(dsr.due_date)}</>
              )}
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
              Löschung durchgeführt
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

/**
 * Full-page view for managing Data Subject Requests (Betroffenenanfragen).
 *
 * Lists open and closed DSRs in separate sections. Shows a red alert banner when
 * any open request has exceeded the 30-day statutory response deadline (Art. 12 DSGVO).
 * Provides inline create and status-update dialogs without navigating away.
 */
export default function DSRPage() {
  const [dialogMode, setDialogMode] = useState<'create' | 'edit' | null>(null)
  const [editId, setEditId] = useState<string | null>(null)
  const [createForm, setCreateForm] = useState<CreateFormState>(emptyCreateForm())
  const [editForm, setEditForm] = useState<EditFormState>({ status: 'open', notes: '' })
  const [deleteId, setDeleteId] = useState<string | null>(null)
  const [erasureId, setErasureId] = useState<string | null>(null)
  const [erasureNote, setErasureNote] = useState('')

  const { data: dsrs, isLoading, isError } = useDSRs()
  const createDSR = useCreateDSR()
  const updateDSR = useUpdateDSR()
  const deleteDSR = useDeleteDSR()

  function openCreate() {
    setCreateForm(emptyCreateForm())
    setEditId(null)
    setDialogMode('create')
  }

  function openEdit(dsr: DSR) {
    setEditForm(emptyEditForm(dsr))
    setEditId(dsr.id)
    setDialogMode('edit')
  }

  function handleDelete(id: string) {
    setDeleteId(id)
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
      input: {
        status: 'completed',
        notes: erasureNote || 'Löschung ausgeführt (Art. 17 DSGVO). Datensätze wurden gelöscht.',
      },
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

  const isPending = createDSR.isPending || updateDSR.isPending
  const canSubmitCreate =
    createForm.requester_name && createForm.requester_email && !isPending
  const canSubmitEdit = !isPending

  const openDSRs = dsrs?.filter((d) => d.status === 'open' || d.status === 'in_progress') ?? []
  const closedDSRs = dsrs?.filter((d) => d.status === 'completed' || d.status === 'rejected') ?? []
  const overdueDSRs = openDSRs.filter((d) => isOverdue(d.due_date))

  return (
    <div className="flex flex-col h-full">
      <PageHeader
        title="Datenschutzanfragen (DSR)"
        description="Art. 17, 20, 21 DSGVO — Verwaltung von Betroffenenrechten."
        actions={
          <div className="flex items-center gap-2">
            <Button variant="outline" onClick={() => {
              void fetch('/api/v1/secprivacy/dsrs/export.csv', { credentials: 'include' })
                .then(res => res.blob())
                .then(blob => {
                  const url = URL.createObjectURL(blob)
                  const a = document.createElement('a')
                  a.href = url
                  a.download = `dsr-export-${new Date().toISOString().slice(0,10)}.csv`
                  document.body.appendChild(a)
                  a.click()
                  a.remove()
                  URL.revokeObjectURL(url)
                })
            }}>
              <Download className="w-4 h-4 mr-1" />
              Exportieren
            </Button>
            <Button onClick={openCreate}>
              <Plus className="w-4 h-4 mr-1" />
              DSR anlegen
            </Button>
          </div>
        }
      />

      <InfoBanner icon={Users} title="Betroffenenrechte nach DSGVO (Art. 12 DSGVO)">
        <p>Betroffene Personen haben das Recht auf Auskunft, Löschung, Datenübertragbarkeit, Widerspruch und Berichtigung. Anfragen müssen innerhalb von <strong>30 Tagen</strong> beantwortet werden — bei komplexen Anfragen maximal 60 Tage (mit Begründung).</p>
        <p className="mt-1">Jede Anfrage muss dokumentiert werden — auch wenn sie abgelehnt wird.</p>
      </InfoBanner>

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
            description="Dokumentieren Sie Betroffenenanfragen gemäß Art. 12-21 DSGVO und verfolgen Sie die 30-Tage-Frist."
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
            {overdueDSRs.length > 0 && (
              <div className="flex items-start gap-3 p-4 bg-red-500/10 border border-red-500/30 rounded-lg">
                <AlertTriangle className="w-5 h-5 text-red-500 shrink-0 mt-0.5" />
                <div>
                  <p className="text-sm font-semibold text-red-500">
                    {overdueDSRs.length} Anfrage{overdueDSRs.length > 1 ? 'n' : ''} — 30-Tage-Frist abgelaufen
                  </p>
                  <p className="text-xs text-secondary mt-0.5">
                    Anfragen müssen innerhalb von 30 Tagen beantwortet werden (Art. 12 DSGVO).
                  </p>
                </div>
              </div>
            )}

            {openDSRs.length > 0 && (
              <div className="space-y-3">
                <h2 className="text-sm font-semibold text-secondary">
                  Offene Anfragen ({openDSRs.length})
                </h2>
                <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 gap-4">
                  {openDSRs.map((d) => (
                    <DSRCard key={d.id} dsr={d} onEdit={openEdit} onDelete={handleDelete} onErasure={handleErasureOpen} />
                  ))}
                </div>
              </div>
            )}

            {closedDSRs.length > 0 && (
              <div className="space-y-3">
                <h2 className="text-sm font-semibold text-muted-foreground">
                  Abgeschlossene Anfragen
                </h2>
                <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 gap-4">
                  {closedDSRs.map((d) => (
                    <DSRCard key={d.id} dsr={d} onEdit={openEdit} onDelete={handleDelete} onErasure={handleErasureOpen} />
                  ))}
                </div>
              </div>
            )}
          </>
        )}
      </div>

      {/* Art. 17 Erasure execution dialog */}
      <Dialog open={erasureId !== null} onOpenChange={(open) => { if (!open) { setErasureId(null); } }}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>Löschung bestätigen (Art. 17 DSGVO)</DialogTitle>
          </DialogHeader>
          <p className="text-sm text-muted-foreground">
            Bestätigen Sie, dass die Daten der betroffenen Person gelöscht wurden. Diese Bestätigung wird als Compliance-Nachweis gespeichert.
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

      <AlertDialog open={deleteId !== null} onOpenChange={(open) => { if (!open) { setDeleteId(null); } }}>
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>Datenschutzanfrage löschen?</AlertDialogTitle>
            <AlertDialogDescription>
              Diese Aktion kann nicht rückgängig gemacht werden.
            </AlertDialogDescription>
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
            <div className="space-y-1.5">
              <Label>Art der Anfrage *</Label>
              <Select
                value={createForm.type}
                onValueChange={(v) => { setCreateForm((f) => ({ ...f, type: v as DSRType })); }}
              >
                <SelectTrigger>
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  {(Object.entries(TYPE_LABELS) as [DSRType, string][]).map(([v, l]) => (
                    <SelectItem key={v} value={v}>{l}</SelectItem>
                  ))}
                </SelectContent>
              </Select>
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
            <Button variant="outline" onClick={() => { setDialogMode(null); }}>
              Abbrechen
            </Button>
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
                <SelectTrigger>
                  <SelectValue />
                </SelectTrigger>
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
            <Button variant="outline" onClick={() => { setDialogMode(null); }}>
              Abbrechen
            </Button>
            <Button onClick={handleSubmit} disabled={!canSubmitEdit}>
              {isPending ? 'Speichern …' : 'Speichern'}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </div>
  )
}
