import { useState } from 'react'
import { Handshake, Plus, Pencil, Trash2, FileDown, LayoutTemplate } from 'lucide-react'
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
import { ProGate } from '../../../shared/components/ProGate'
import { useAVVs, useCreateAVV, useUpdateAVV, useDeleteAVV } from '../hooks/useAVVs'
import { useDownloadAVVPDF } from '../hooks/useAVVTemplates'
import { AVVTemplatePickerDialog } from '../components/AVVTemplatePickerDialog'
import type { AVV, CreateAVVInput, UpdateAVVInput } from '../types'

const STATUS_LABELS: Record<AVV['status'], string> = {
  active: 'Aktiv',
  expired: 'Abgelaufen',
  terminated: 'Gekündigt',
}

const STATUS_CLASS: Record<AVV['status'], string> = {
  active: 'bg-green-500/20 text-green-400 border-green-500/30',
  expired: 'bg-amber-500/20 text-amber-400 border-amber-500/30',
  terminated: 'bg-red-500/20 text-red-400 border-red-500/30',
}

interface AVVFormState {
  processor_name: string
  service_description: string
  contract_date: string
  review_date: string
  status: AVV['status']
  notes: string
}

function emptyForm(): AVVFormState {
  return {
    processor_name: '',
    service_description: '',
    contract_date: '',
    review_date: '',
    status: 'active',
    notes: '',
  }
}

function formFromEntry(a: AVV): AVVFormState {
  return {
    processor_name: a.processor_name,
    service_description: a.service_description,
    contract_date: a.contract_date ?? '',
    review_date: a.review_date ?? '',
    status: a.status,
    notes: a.notes ?? '',
  }
}

function AVVCard({
  avv,
  onEdit,
  onDelete,
  onDownloadPDF,
}: {
  avv: AVV
  onEdit: (a: AVV) => void
  onDelete: (id: string) => void
  onDownloadPDF: (id: string) => void
}) {
  const contractDate = avv.contract_date
    ? new Date(avv.contract_date).toLocaleDateString('de-DE', { year: 'numeric', month: 'short', day: 'numeric' })
    : null
  const reviewDate = avv.review_date
    ? new Date(avv.review_date).toLocaleDateString('de-DE', { year: 'numeric', month: 'short', day: 'numeric' })
    : null

  return (
    <Card>
      <CardContent className="pt-5 space-y-2">
        <div className="flex items-start justify-between gap-2">
          <p className="font-medium text-sm">{avv.processor_name}</p>
          <Badge className={STATUS_CLASS[avv.status]}>{STATUS_LABELS[avv.status]}</Badge>
        </div>
        <p className="text-xs text-muted-foreground line-clamp-2">{avv.service_description}</p>
        <div className="flex flex-wrap gap-3 text-xs text-muted-foreground">
          {contractDate && <span>Abgeschlossen: {contractDate}</span>}
          {reviewDate && <span>Review: {reviewDate}</span>}
        </div>
        {avv.template_id && (
          <p className="text-xs text-primary/70">Vorlage: {avv.template_id}</p>
        )}
        {avv.notes && <p className="text-xs text-muted-foreground italic line-clamp-1">{avv.notes}</p>}
        <div className="flex justify-end gap-1 pt-1">
          {avv.body && (
            <Button
              size="icon"
              variant="ghost"
              className="h-7 w-7"
              title="PDF exportieren"
              onClick={() => onDownloadPDF(avv.id)}
            >
              <FileDown className="w-3.5 h-3.5" />
            </Button>
          )}
          <Button size="icon" variant="ghost" className="h-7 w-7" aria-label="Bearbeiten" onClick={() => onEdit(avv)}>
            <Pencil className="w-3.5 h-3.5" />
          </Button>
          <Button
            size="icon"
            variant="ghost"
            className="h-7 w-7 text-destructive hover:text-destructive"
            aria-label="Löschen"
            onClick={() => onDelete(avv.id)}
          >
            <Trash2 className="w-3.5 h-3.5" />
          </Button>
        </div>
      </CardContent>
    </Card>
  )
}

function AVVForm({
  form,
  onChange,
  showStatus,
}: {
  form: AVVFormState
  onChange: (f: AVVFormState) => void
  showStatus: boolean
}) {
  const set = (patch: Partial<AVVFormState>) => onChange({ ...form, ...patch })

  return (
    <div className="space-y-4 py-2">
      <div className="space-y-1.5">
        <Label>Auftragsverarbeiter *</Label>
        <Input
          placeholder="z.B. Amazon Web Services EMEA SARL"
          value={form.processor_name}
          onChange={(e) => set({ processor_name: e.target.value })}
        />
      </div>
      <div className="space-y-1.5">
        <Label>Leistungsbeschreibung *</Label>
        <Textarea
          placeholder="Welche Leistung erbringt der Auftragsverarbeiter?"
          rows={3}
          value={form.service_description}
          onChange={(e) => set({ service_description: e.target.value })}
        />
      </div>
      <div className="grid grid-cols-2 gap-3">
        <div className="space-y-1.5">
          <Label>Vertragsdatum</Label>
          <Input
            type="date"
            value={form.contract_date}
            onChange={(e) => set({ contract_date: e.target.value })}
          />
        </div>
        <div className="space-y-1.5">
          <Label>Review-Datum</Label>
          <Input
            type="date"
            value={form.review_date}
            onChange={(e) => set({ review_date: e.target.value })}
          />
        </div>
      </div>
      {showStatus && (
        <div className="space-y-1.5">
          <Label>Status</Label>
          <Select value={form.status} onValueChange={(v) => set({ status: v as AVV['status'] })}>
            <SelectTrigger>
              <SelectValue />
            </SelectTrigger>
            <SelectContent>
              <SelectItem value="active">Aktiv</SelectItem>
              <SelectItem value="expired">Abgelaufen</SelectItem>
              <SelectItem value="terminated">Gekündigt</SelectItem>
            </SelectContent>
          </Select>
        </div>
      )}
      <div className="space-y-1.5">
        <Label>Notizen</Label>
        <Textarea
          placeholder="Interne Notizen …"
          rows={2}
          value={form.notes}
          onChange={(e) => set({ notes: e.target.value })}
        />
      </div>
    </div>
  )
}

export default function AVVPage() {
  const [dialogMode, setDialogMode] = useState<'create' | 'edit' | null>(null)
  const [editId, setEditId] = useState<string | null>(null)
  const [form, setForm] = useState<AVVFormState>(emptyForm())
  const [templatePickerOpen, setTemplatePickerOpen] = useState(false)
  const [deleteId, setDeleteId] = useState<string | null>(null)
  const [pdfError, setPdfError] = useState<unknown>(null)

  const { data: avvs, isLoading, isError } = useAVVs()
  const createAVV = useCreateAVV()
  const updateAVV = useUpdateAVV()
  const deleteAVV = useDeleteAVV()
  const downloadAVVPDF = useDownloadAVVPDF()

  function openCreate() {
    setForm(emptyForm())
    setEditId(null)
    setDialogMode('create')
  }

  function openEdit(avv: AVV) {
    setForm(formFromEntry(avv))
    setEditId(avv.id)
    setDialogMode('edit')
  }

  function handleDelete(id: string) {
    setDeleteId(id)
  }

  function confirmDelete() {
    if (deleteId) deleteAVV.mutate(deleteId)
    setDeleteId(null)
  }

  async function handleDownloadPDF(id: string) {
    try {
      setPdfError(null)
      await downloadAVVPDF(id)
    } catch (err) {
      setPdfError(err)
    }
  }

  function handleSubmit() {
    if (dialogMode === 'create') {
      const payload: CreateAVVInput = {
        processor_name: form.processor_name,
        service_description: form.service_description,
        contract_date: form.contract_date || undefined,
        review_date: form.review_date || undefined,
        notes: form.notes || undefined,
      }
      createAVV.mutate(payload, { onSuccess: () => setDialogMode(null) })
    } else if (dialogMode === 'edit' && editId) {
      const payload: UpdateAVVInput = {
        processor_name: form.processor_name,
        service_description: form.service_description,
        contract_date: form.contract_date || undefined,
        review_date: form.review_date || undefined,
        status: form.status,
        notes: form.notes || undefined,
      }
      updateAVV.mutate({ id: editId, input: payload }, { onSuccess: () => setDialogMode(null) })
    }
  }

  const isPending = createAVV.isPending || updateAVV.isPending
  const canSubmit = form.processor_name && form.service_description && !isPending

  return (
    <div className="flex flex-col h-full">
      <PageHeader
        title="Auftragsverarbeitungsverträge (AVV)"
        description="Art. 28 DSGVO — Verträge mit Auftragsverarbeitern dokumentieren und überwachen."
        actions={
          <div className="flex gap-2">
            <Button variant="outline" onClick={() => setTemplatePickerOpen(true)}>
              <LayoutTemplate className="w-4 h-4 mr-1" />
              Aus Vorlage erstellen
            </Button>
            <Button onClick={openCreate}>
              <Plus className="w-4 h-4 mr-1" />
              AVV anlegen
            </Button>
          </div>
        }
      />

      <InfoBanner icon={Handshake} title="Auftragsverarbeitungsverträge (Art. 28 DSGVO)">
        <p>Mit jedem Dienstleister, der in deinem Auftrag personenbezogene Daten verarbeitet (z.B. Cloud-Anbieter, HR-Software, E-Mail-Dienstleister), ist ein AVV abzuschließen — <strong>ohne AVV ist die Beauftragung rechtswidrig</strong>.</p>
        <p className="mt-1">Tipp: Review-Datum setzen und rechtzeitig vor Vertragserneuerungen prüfen, ob der AVV noch aktuell ist.</p>
      </InfoBanner>

      <ProGate error={pdfError}>{null}</ProGate>

      <div className="flex-1 p-6">
        {isLoading && (
          <div className="flex items-center justify-center h-48">
            <div className="w-6 h-6 border-2 border-primary border-t-transparent rounded-full animate-spin" />
          </div>
        )}

        {isError && (
          <div className="text-sm text-red-400 p-4 bg-red-500/10 rounded-lg">
            Fehler beim Laden der Auftragsverarbeitungsverträge.
          </div>
        )}

        {!isLoading && !isError && avvs?.length === 0 && (
          <EmptyState
            icon={Handshake}
            title="Noch keine AVVs"
            description="Dokumentieren Sie Verträge mit Ihren Auftragsverarbeitern gemäß Art. 28 DSGVO."
            action={
              <div className="flex gap-2 justify-center">
                <Button variant="outline" onClick={() => setTemplatePickerOpen(true)}>
                  <LayoutTemplate className="w-4 h-4 mr-1" />
                  Aus Vorlage erstellen
                </Button>
                <Button onClick={openCreate}>
                  <Plus className="w-4 h-4 mr-1" />
                  AVV anlegen
                </Button>
              </div>
            }
          />
        )}

        {!isLoading && !isError && avvs && avvs.length > 0 && (
          <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 gap-4">
            {avvs.map((a) => (
              <AVVCard
                key={a.id}
                avv={a}
                onEdit={openEdit}
                onDelete={handleDelete}
                onDownloadPDF={handleDownloadPDF}
              />
            ))}
          </div>
        )}
      </div>

      <AlertDialog open={deleteId !== null} onOpenChange={(open) => !open && setDeleteId(null)}>
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>AVV löschen?</AlertDialogTitle>
            <AlertDialogDescription>
              Diese Aktion kann nicht rückgängig gemacht werden.
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel onClick={() => setDeleteId(null)}>Abbrechen</AlertDialogCancel>
            <AlertDialogAction onClick={confirmDelete} className="bg-destructive text-destructive-foreground hover:bg-destructive/90">Löschen</AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>

      <Dialog open={dialogMode !== null} onOpenChange={(open) => !open && setDialogMode(null)}>
        <DialogContent className="max-w-lg max-h-[90vh] overflow-y-auto">
          <DialogHeader>
            <DialogTitle>
              {dialogMode === 'create' ? 'AVV anlegen' : 'AVV bearbeiten'}
            </DialogTitle>
          </DialogHeader>
          <AVVForm form={form} onChange={setForm} showStatus={dialogMode === 'edit'} />
          <DialogFooter>
            <Button variant="outline" onClick={() => setDialogMode(null)}>
              Abbrechen
            </Button>
            <Button onClick={handleSubmit} disabled={!canSubmit}>
              {isPending ? 'Speichern …' : dialogMode === 'create' ? 'AVV anlegen' : 'Speichern'}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      <AVVTemplatePickerDialog
        open={templatePickerOpen}
        onOpenChange={setTemplatePickerOpen}
      />
    </div>
  )
}
