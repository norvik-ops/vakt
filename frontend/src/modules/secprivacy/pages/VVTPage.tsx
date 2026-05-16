import { useState } from 'react'
import { FileText, Plus, Globe2, Pencil, Trash2, Download } from 'lucide-react'
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
import { Pagination } from '../../../shared/components/Pagination'
import { useVVT, useCreateVVT, useUpdateVVT, useDeleteVVT, useExportVVT } from '../hooks/useVVT'
import type { VVTEntry, CreateVVTInput, UpdateVVTInput } from '../types'

const LEGAL_BASIS_OPTIONS = [
  { value: 'Art. 6 Abs. 1 lit. a DSGVO', label: 'Einwilligung (Art. 6 I a)' },
  { value: 'Art. 6 Abs. 1 lit. b DSGVO', label: 'Vertragserfüllung (Art. 6 I b)' },
  { value: 'Art. 6 Abs. 1 lit. c DSGVO', label: 'Rechtliche Pflicht (Art. 6 I c)' },
  { value: 'Art. 6 Abs. 1 lit. d DSGVO', label: 'Lebenswichtige Interessen (Art. 6 I d)' },
  { value: 'Art. 6 Abs. 1 lit. e DSGVO', label: 'Öffentliches Interesse (Art. 6 I e)' },
  { value: 'Art. 6 Abs. 1 lit. f DSGVO', label: 'Berechtigte Interessen (Art. 6 I f)' },
]

function StatusBadge({ status }: { status: VVTEntry['status'] }) {
  return status === 'active' ? (
    <Badge className="bg-green-500/20 text-green-400 border-green-500/30">Aktiv</Badge>
  ) : (
    <Badge variant="secondary">Archiviert</Badge>
  )
}

function tagsFromRaw(raw: string): string[] {
  return raw.split(',').map((s) => s.trim()).filter(Boolean)
}

function rawFromTags(tags: string[]): string {
  return tags.join(', ')
}

interface VVTFormState {
  name: string
  purpose: string
  legal_basis: string
  retention_period: string
  third_country_transfer: boolean
  safeguards: string
  responsible_person: string
  status: 'active' | 'archived'
  rawCategories: string
  rawSubjects: string
  rawRecipients: string
}

function emptyForm(): VVTFormState {
  return {
    name: '',
    purpose: '',
    legal_basis: '',
    retention_period: '',
    third_country_transfer: false,
    safeguards: '',
    responsible_person: '',
    status: 'active',
    rawCategories: '',
    rawSubjects: '',
    rawRecipients: '',
  }
}

function formFromEntry(e: VVTEntry): VVTFormState {
  return {
    name: e.name,
    purpose: e.purpose,
    legal_basis: e.legal_basis,
    retention_period: e.retention_period ?? '',
    third_country_transfer: e.third_country_transfer,
    safeguards: e.safeguards ?? '',
    responsible_person: e.responsible_person ?? '',
    status: e.status,
    rawCategories: rawFromTags(e.data_categories),
    rawSubjects: rawFromTags(e.data_subjects),
    rawRecipients: rawFromTags(e.recipients),
  }
}

function VVTCard({
  entry,
  onEdit,
  onDelete,
}: {
  entry: VVTEntry
  onEdit: (e: VVTEntry) => void
  onDelete: (id: string) => void
}) {
  const date = new Date(entry.created_at).toLocaleDateString('de-DE', {
    year: 'numeric',
    month: 'short',
    day: 'numeric',
  })

  return (
    <Card>
      <CardContent className="pt-5 space-y-3">
        <div className="flex items-start justify-between gap-2">
          <div>
            <p className="font-medium text-sm">{entry.name}</p>
            <p className="text-xs text-muted-foreground mt-0.5">{entry.legal_basis}</p>
          </div>
          <div className="flex items-center gap-1.5 shrink-0">
            {entry.third_country_transfer && (
              <Globe2 className="w-4 h-4 text-amber-400" aria-label="Drittlandtransfer" />
            )}
            <StatusBadge status={entry.status} />
          </div>
        </div>
        <p className="text-xs text-muted-foreground line-clamp-2">{entry.purpose}</p>
        {entry.data_categories.length > 0 && (
          <div className="flex flex-wrap gap-1">
            {entry.data_categories.map((c) => (
              <span key={c} className="text-xs px-1.5 py-0.5 rounded bg-primary/10 text-primary">
                {c}
              </span>
            ))}
          </div>
        )}
        <div className="flex items-center justify-between">
          <p className="text-xs text-muted-foreground">Erstellt {date}</p>
          <div className="flex gap-1">
            <Button size="icon" variant="ghost" className="h-7 w-7" aria-label="Bearbeiten" onClick={() => onEdit(entry)}>
              <Pencil className="w-3.5 h-3.5" />
            </Button>
            <Button
              size="icon"
              variant="ghost"
              className="h-7 w-7 text-destructive hover:text-destructive"
              aria-label="Löschen"
              onClick={() => onDelete(entry.id)}
            >
              <Trash2 className="w-3.5 h-3.5" />
            </Button>
          </div>
        </div>
      </CardContent>
    </Card>
  )
}

function VVTForm({
  form,
  onChange,
}: {
  form: VVTFormState
  onChange: (f: VVTFormState) => void
}) {
  const set = (patch: Partial<VVTFormState>) => onChange({ ...form, ...patch })

  return (
    <div className="space-y-4 py-2">
      <div className="space-y-1.5">
        <Label>Bezeichnung *</Label>
        <Input
          placeholder="z.B. Kundenverwaltung"
          value={form.name}
          onChange={(e) => set({ name: e.target.value })}
        />
      </div>
      <div className="space-y-1.5">
        <Label>Zweck der Verarbeitung *</Label>
        <Textarea
          placeholder="Beschreiben Sie den Verarbeitungszweck …"
          rows={2}
          value={form.purpose}
          onChange={(e) => set({ purpose: e.target.value })}
        />
      </div>
      <div className="space-y-1.5">
        <Label>Rechtsgrundlage *</Label>
        <Select value={form.legal_basis} onValueChange={(v) => set({ legal_basis: v })}>
          <SelectTrigger>
            <SelectValue placeholder="Rechtsgrundlage wählen …" />
          </SelectTrigger>
          <SelectContent>
            {LEGAL_BASIS_OPTIONS.map((o) => (
              <SelectItem key={o.value} value={o.value}>
                {o.label}
              </SelectItem>
            ))}
          </SelectContent>
        </Select>
      </div>
      <div className="space-y-1.5">
        <Label>Datenkategorien (kommagetrennt)</Label>
        <Input
          placeholder="z.B. Name, E-Mail, IP-Adresse"
          value={form.rawCategories}
          onChange={(e) => set({ rawCategories: e.target.value })}
        />
      </div>
      <div className="space-y-1.5">
        <Label>Betroffene Personen (kommagetrennt)</Label>
        <Input
          placeholder="z.B. Kunden, Mitarbeiter"
          value={form.rawSubjects}
          onChange={(e) => set({ rawSubjects: e.target.value })}
        />
      </div>
      <div className="space-y-1.5">
        <Label>Empfänger (kommagetrennt)</Label>
        <Input
          placeholder="z.B. Steuerberater, Hosting-Anbieter"
          value={form.rawRecipients}
          onChange={(e) => set({ rawRecipients: e.target.value })}
        />
      </div>
      <div className="space-y-1.5">
        <Label>Löschfrist</Label>
        <Input
          placeholder="z.B. 10 Jahre (§ 257 HGB)"
          value={form.retention_period}
          onChange={(e) => set({ retention_period: e.target.value })}
        />
      </div>
      <div className="space-y-1.5">
        <Label>Verantwortliche Person</Label>
        <Input
          placeholder="z.B. Max Mustermann"
          value={form.responsible_person}
          onChange={(e) => set({ responsible_person: e.target.value })}
        />
      </div>
      <div className="space-y-1.5">
        <Label>Status</Label>
        <Select value={form.status} onValueChange={(v) => set({ status: v as 'active' | 'archived' })}>
          <SelectTrigger>
            <SelectValue />
          </SelectTrigger>
          <SelectContent>
            <SelectItem value="active">Aktiv</SelectItem>
            <SelectItem value="archived">Archiviert</SelectItem>
          </SelectContent>
        </Select>
      </div>
      <div className="flex items-center gap-2">
        <input
          type="checkbox"
          id="vvt-thirdcountry"
          checked={form.third_country_transfer}
          onChange={(e) => set({ third_country_transfer: e.target.checked })}
          className="w-4 h-4"
        />
        <Label htmlFor="vvt-thirdcountry">Drittlandtransfer (außerhalb EU/EWR)</Label>
      </div>
      {form.third_country_transfer && (
        <div className="space-y-1.5">
          <Label>Schutzmaßnahmen (Art. 46 DSGVO)</Label>
          <Textarea
            placeholder="z.B. Standardvertragsklauseln (SCC)"
            rows={2}
            value={form.safeguards}
            onChange={(e) => set({ safeguards: e.target.value })}
          />
        </div>
      )}
    </div>
  )
}

export default function VVTPage() {
  const [dialogMode, setDialogMode] = useState<'create' | 'edit' | null>(null)
  const [editId, setEditId] = useState<string | null>(null)
  const [form, setForm] = useState<VVTFormState>(emptyForm())
  const [page, setPage] = useState(1)
  const [deleteId, setDeleteId] = useState<string | null>(null)

  const { data: entries, isLoading, isError, pagination } = useVVT(page)
  const createVVT = useCreateVVT()
  const updateVVT = useUpdateVVT()
  const deleteVVT = useDeleteVVT()
  const exportVVT = useExportVVT()

  function openCreate() {
    setForm(emptyForm())
    setEditId(null)
    setDialogMode('create')
  }

  function openEdit(entry: VVTEntry) {
    setForm(formFromEntry(entry))
    setEditId(entry.id)
    setDialogMode('edit')
  }

  function handleDelete(id: string) {
    setDeleteId(id)
  }

  function confirmDelete() {
    if (deleteId) deleteVVT.mutate(deleteId)
    setDeleteId(null)
  }

  function buildPayload(): CreateVVTInput {
    return {
      name: form.name,
      purpose: form.purpose,
      legal_basis: form.legal_basis,
      data_categories: tagsFromRaw(form.rawCategories),
      data_subjects: tagsFromRaw(form.rawSubjects),
      recipients: tagsFromRaw(form.rawRecipients),
      retention_period: form.retention_period || undefined,
      third_country_transfer: form.third_country_transfer,
      safeguards: form.safeguards || undefined,
      responsible_person: form.responsible_person || undefined,
    }
  }

  function handleSubmit() {
    if (dialogMode === 'create') {
      createVVT.mutate(buildPayload(), { onSuccess: () => setDialogMode(null) })
    } else if (dialogMode === 'edit' && editId) {
      const payload: UpdateVVTInput = { ...buildPayload(), status: form.status }
      updateVVT.mutate({ id: editId, input: payload }, { onSuccess: () => setDialogMode(null) })
    }
  }

  const isPending = createVVT.isPending || updateVVT.isPending
  const canSubmit = form.name && form.purpose && form.legal_basis && !isPending

  return (
    <div className="flex flex-col h-full">
      <PageHeader
        title="Verarbeitungsverzeichnis (VVT)"
        description="Art. 30 DSGVO — Dokumentation aller Verarbeitungstätigkeiten."
        actions={
          <div className="flex gap-2">
            <Button variant="outline" onClick={exportVVT} disabled={!entries || entries.length === 0}>
              <Download className="w-4 h-4 mr-1" />
              Als PDF exportieren
            </Button>
            <Button onClick={openCreate}>
              <Plus className="w-4 h-4 mr-1" />
              Eintrag anlegen
            </Button>
          </div>
        }
      />

      <InfoBanner icon={FileText} title="Verzeichnis von Verarbeitungstätigkeiten (Art. 30 DSGVO)">
        <p>
          Das VVT dokumentiert alle personenbezogenen Datenverarbeitungen deiner Organisation — mit Zweck,
          Rechtsgrundlage, Datenkategorien, Empfängern und Löschfristen.
        </p>
      </InfoBanner>

      <div className="flex-1 p-6">
        {isLoading && (
          <div className="flex items-center justify-center h-48">
            <div className="w-6 h-6 border-2 border-primary border-t-transparent rounded-full animate-spin" />
          </div>
        )}
        {isError && (
          <div className="text-sm text-red-400 p-4 bg-red-500/10 rounded-lg">
            Fehler beim Laden des Verarbeitungsverzeichnisses.
          </div>
        )}
        {!isLoading && !isError && entries?.length === 0 && (
          <EmptyState
            icon={FileText}
            title="Noch keine Einträge"
            description="Dokumentieren Sie Ihre Verarbeitungstätigkeiten gemäß Art. 30 DSGVO."
            action={
              <Button onClick={openCreate}>
                <Plus className="w-4 h-4 mr-1" />
                Eintrag anlegen
              </Button>
            }
          />
        )}
        {!isLoading && !isError && entries && entries.length > 0 && (
          <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 gap-4">
            {entries.map((e) => (
              <VVTCard key={e.id} entry={e} onEdit={openEdit} onDelete={handleDelete} />
            ))}
          </div>
        )}
        <Pagination
          page={page}
          totalPages={pagination?.total_pages ?? 1}
          onPageChange={setPage}
        />
      </div>

      <AlertDialog open={deleteId !== null} onOpenChange={(open) => !open && setDeleteId(null)}>
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>Eintrag löschen?</AlertDialogTitle>
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
              {dialogMode === 'create' ? 'VVT-Eintrag anlegen' : 'VVT-Eintrag bearbeiten'}
            </DialogTitle>
          </DialogHeader>
          <VVTForm form={form} onChange={setForm} />
          <DialogFooter>
            <Button variant="outline" onClick={() => setDialogMode(null)}>
              Abbrechen
            </Button>
            <Button onClick={handleSubmit} disabled={!canSubmit}>
              {isPending ? 'Speichern …' : dialogMode === 'create' ? 'Eintrag anlegen' : 'Speichern'}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </div>
  )
}
