import { useState, useRef } from 'react'
import { Building2, Plus, Pencil, Trash2, Download, Upload } from 'lucide-react'
import { PageHeader } from '../../../shared/components/PageHeader'
import { EmptyState } from '../../../shared/components/EmptyState'
import { Button } from '../../../components/ui/button'
import { Badge } from '../../../components/ui/badge'
import { Card, CardContent } from '../../../components/ui/card'
import { Dialog, DialogContent, DialogHeader, DialogTitle, DialogFooter } from '../../../components/ui/dialog'
import { Input } from '../../../components/ui/input'
import { Label } from '../../../components/ui/label'
import { Textarea } from '../../../components/ui/textarea'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '../../../components/ui/select'
import { useSuppliers, useCreateSupplier, useUpdateSupplier, useDeleteSupplier, useImportSuppliersCSV } from '../hooks/useSuppliers'
import { ProGate } from '../../../shared/components/ProGate'
import { useSupplierStatus, statusToVariant } from '../hooks/useAssessments'
import type { Supplier, CreateSupplierInput } from '../types'
import { SkeletonCardGrid } from '../../../shared/components/SkeletonLoaders'
import { FieldError } from '../../../shared/components/FieldError'
import { useFormValidation } from '../../../shared/hooks/useFormValidation'
import { toast } from '../../../shared/hooks/useToast'
import { useFormatDate } from '../../../shared/hooks/useFormatDate'

// ─── Toast (minimal inline) ───────────────────────────────────────────────────

const CRITICALITY_CLASS: Record<Supplier['criticality'], string> = {
  standard: 'bg-secondary text-secondary-foreground',
  important: 'bg-amber-500/20 text-amber-400 border-amber-500/30',
  critical: 'bg-red-500/20 text-red-400 border-red-500/30',
}
const CRITICALITY_LABELS: Record<Supplier['criticality'], string> = {
  standard: 'Standard', important: 'Wichtig', critical: 'Kritisch',
}

const CONTRACT_STATUS_CLASS: Record<string, string> = {
  active: 'bg-green-500/20 text-green-400 border-green-500/30',
  expiring_soon: 'bg-amber-500/20 text-amber-400 border-amber-500/30',
  expired: 'bg-red-500/20 text-red-400 border-red-500/30',
}
const CONTRACT_STATUS_LABELS: Record<string, string> = {
  active: 'Aktiv',
  expiring_soon: 'Läuft ab',
  expired: 'Abgelaufen',
}

const ASSESSMENT_STATUS_CLASS: Record<string, string> = {
  none: 'bg-secondary text-secondary-foreground',
  pending: 'bg-amber-500/20 text-amber-400 border-amber-500/30',
  completed: 'bg-green-500/20 text-green-400 border-green-500/30',
}
const ASSESSMENT_STATUS_LABELS: Record<string, string> = {
  none: 'Nicht bewertet',
  pending: 'Ausstehend',
  completed: 'Abgeschlossen',
}

function ContractStatusBadge({ status }: { status?: string }) {
  if (!status) return null
  return (
    <Badge
      className={CONTRACT_STATUS_CLASS[status] ?? 'bg-secondary text-secondary-foreground'}
      data-testid="contract-status-badge"
    >
      {CONTRACT_STATUS_LABELS[status] ?? status}
    </Badge>
  )
}

function AssessmentStatusBadge({ status }: { status?: string }) {
  if (!status) return null
  return (
    <Badge
      className={ASSESSMENT_STATUS_CLASS[status] ?? 'bg-secondary text-secondary-foreground'}
      data-testid="assessment-status-badge"
    >
      {ASSESSMENT_STATUS_LABELS[status] ?? status}
    </Badge>
  )
}

function emptyForm(): CreateSupplierInput {
  return {
    name: '',
    contact_name: '',
    contact_email: '',
    service_type: '',
    criticality: 'standard',
    nis2_relevant: false,
    dora_relevant: false,
    contract_end: '',
    notes: '',
    sub_suppliers: [],
    data_location: '',
    exit_strategy_exists: false,
    assessment_status: 'none',
  }
}

function supplierToForm(s: Supplier): CreateSupplierInput {
  return {
    name: s.name,
    contact_name: s.contact_name ?? '',
    contact_email: s.contact_email ?? '',
    service_type: s.service_type ?? '',
    criticality: s.criticality,
    nis2_relevant: s.nis2_relevant,
    dora_relevant: s.dora_relevant,
    contract_end: s.contract_end ? s.contract_end.slice(0, 10) : '',
    notes: s.notes ?? '',
    sub_suppliers: s.sub_suppliers ?? [],
    data_location: s.data_location ?? '',
    exit_strategy_exists: s.exit_strategy_exists ?? false,
    assessment_status: s.assessment_status ?? 'none',
  }
}

function SupplierStatusBadge({ supplierId }: { supplierId: string }) {
  const { data: status } = useSupplierStatus(supplierId)
  if (!status) return null
  const variant = statusToVariant(status.status)
  const labels = { green: 'Grün', yellow: 'Gelb', red: 'Rot' }
  return (
    <Badge
      variant={variant}
      data-testid="supplier-status-badge"
      data-status={status.status}
      className={
        status.status === 'green'
          ? 'bg-green-100 text-green-800 border-green-300'
          : status.status === 'yellow'
            ? 'bg-yellow-100 text-yellow-800 border-yellow-300'
            : 'bg-red-100 text-red-800 border-red-300'
      }
    >
      {labels[status.status] ?? status.status}
    </Badge>
  )
}

function SupplierCard({ supplier, onEdit, onDelete }: { supplier: Supplier; onEdit: () => void; onDelete: () => void }) {
  const { formatDate } = useFormatDate()
  return (
    <Card>
      <CardContent className="pt-5 space-y-2">
        <div className="flex items-start justify-between gap-2">
          <p className="font-medium text-sm">{supplier.name}</p>
          <div className="flex items-center gap-1.5 shrink-0">
            <Badge className={CRITICALITY_CLASS[supplier.criticality]}>{CRITICALITY_LABELS[supplier.criticality]}</Badge>
            <ContractStatusBadge status={supplier.contract_status} />
            <AssessmentStatusBadge status={supplier.assessment_status} />
            <SupplierStatusBadge supplierId={supplier.id} />
            <Button variant="ghost" size="icon" className="h-7 w-7" onClick={onEdit}><Pencil className="w-3.5 h-3.5" /></Button>
            <Button variant="ghost" size="icon" className="h-7 w-7 text-red-400 hover:text-red-300" onClick={onDelete}><Trash2 className="w-3.5 h-3.5" /></Button>
          </div>
        </div>
        {supplier.service_type && (
          <p className="text-xs text-muted-foreground">{supplier.service_type}</p>
        )}
        <div className="flex flex-wrap gap-1.5">
          {supplier.nis2_relevant && <Badge variant="outline" className="text-xs">NIS2</Badge>}
          {supplier.dora_relevant && <Badge variant="outline" className="text-xs">DORA</Badge>}
          {supplier.data_location && <Badge variant="outline" className="text-xs">{supplier.data_location}</Badge>}
        </div>
        <div className="text-xs text-muted-foreground space-y-0.5">
          {supplier.contact_name && <p>Kontakt: {supplier.contact_name}{supplier.contact_email ? ` · ${supplier.contact_email}` : ''}</p>}
          {supplier.contract_end && <p>Vertragsende: {formatDate(supplier.contract_end)}</p>}
        </div>
      </CardContent>
    </Card>
  )
}

export default function SuppliersPage() {
  const [dialogOpen, setDialogOpen] = useState(false)
  const [editId, setEditId] = useState<string | null>(null)
  const [form, setForm] = useState<CreateSupplierInput>(emptyForm())
  const [subSuppliersRaw, setSubSuppliersRaw] = useState('')
  const [filterCriticality, setFilterCriticality] = useState('')
  const [filterAssessmentStatus, setFilterAssessmentStatus] = useState('')
  const fileInputRef = useRef<HTMLInputElement>(null)

  const filters = {
    criticality: filterCriticality || undefined,
    assessmentStatus: filterAssessmentStatus || undefined,
  }
  const { data: suppliers, isLoading, isError, error } = useSuppliers(filters)
  const createSupplier = useCreateSupplier()
  const updateSupplier = useUpdateSupplier(editId ?? '')
  const deleteSupplier = useDeleteSupplier()
  const importCSV = useImportSuppliersCSV()
  const { errors: supErrors, validate: validateSup, clearError: clearSupError, clearAll: clearSupErrors } = useFormValidation<Record<string, unknown>>({
    name: { required: true, maxLength: 200 },
    contact_email: { pattern: /^$|^[^\s@]+@[^\s@]+\.[^\s@]+$/, patternMessage: 'Bitte eine gültige E-Mail-Adresse eingeben.' },
  })

  function openCreate() {
    setEditId(null)
    setForm(emptyForm())
    setSubSuppliersRaw('')
    clearSupErrors()
    setDialogOpen(true)
  }

  function openEdit(s: Supplier) {
    setEditId(s.id)
    setForm(supplierToForm(s))
    setSubSuppliersRaw((s.sub_suppliers ?? []).join(', '))
    clearSupErrors()
    setDialogOpen(true)
  }

  function handleDelete(id: string) {
    if (confirm('Lieferanten wirklich löschen?')) {
      deleteSupplier.mutate(id)
    }
  }

  function handleSubmit() {
    if (!validateSup({ name: form.name, contact_email: form.contact_email ?? '' })) return
    const sub = subSuppliersRaw
      .split(',')
      .map((v) => v.trim())
      .filter(Boolean)
    const payload = {
      ...form,
      sub_suppliers: sub,
      contract_end: form.contract_end ? new Date(form.contract_end).toISOString() : undefined,
    }
    if (editId) {
      updateSupplier.mutate(payload, { onSuccess: () => { setDialogOpen(false); } })
    } else {
      createSupplier.mutate(payload, {
        onSuccess: () => {
          setDialogOpen(false)
          toast(`Lieferant hinzugefügt: ${form.name} wurde zur Lieferantenliste hinzugefügt.`, 'success')
        },
      })
    }
  }

  async function handleExportCSV() {
    const res = await fetch('/api/v1/secvitals/suppliers/export', {
      credentials: 'include',
    })
    if (!res.ok) {
      toast('CSV-Export fehlgeschlagen. Bitte versuchen Sie es erneut.', 'error')
      return
    }
    const blob = await res.blob()
    const url = URL.createObjectURL(blob)
    const a = document.createElement('a')
    a.href = url
    a.download = 'suppliers-export.csv'
    document.body.appendChild(a)
    a.click()
    document.body.removeChild(a)
    URL.revokeObjectURL(url)
  }

  function handleImportCSVClick() {
    fileInputRef.current?.click()
  }

  function handleImportFileChange(e: React.ChangeEvent<HTMLInputElement>) {
    const file = e.target.files?.[0]
    if (!file) return
    const formData = new FormData()
    formData.append('file', file)
    importCSV.mutate(formData, {
      onSuccess: (result) => {
        toast(`Import abgeschlossen: ${result.imported} importiert, ${result.skipped} übersprungen.`, 'success')
      },
      onError: (err) => {
        toast(`Import fehlgeschlagen: ${err.message}`, 'error')
      },
    })
    // Reset file input so same file can be re-uploaded
    if (fileInputRef.current) fileInputRef.current.value = ''
  }

  const isPending = createSupplier.isPending || updateSupplier.isPending

  return (
    <ProGate error={isError ? error : null}>
    <div className="flex flex-col h-full">
      <PageHeader
        title="Lieferanten-Register"
        description="Drittanbieter und Dienstleister verwalten — NIS2 Art. 21 / DORA Art. 28."
        actions={
          <div className="flex gap-2">
            <Button variant="outline" onClick={() => { void handleExportCSV(); }}>
              <Download className="w-4 h-4 mr-1" />
              CSV exportieren
            </Button>
            <Button variant="outline" onClick={handleImportCSVClick} disabled={importCSV.isPending} data-testid="import-csv-button">
              <Upload className="w-4 h-4 mr-1" />
              {importCSV.isPending ? 'Importieren …' : 'CSV importieren'}
            </Button>
            <input
              ref={fileInputRef}
              type="file"
              accept=".csv,text/csv"
              className="hidden"
              onChange={handleImportFileChange}
              data-testid="csv-file-input"
            />
            <Button onClick={openCreate}>
              <Plus className="w-4 h-4 mr-1" />
              Lieferant hinzufügen
            </Button>
          </div>
        }
      />

      {/* Filter toolbar */}
      <div className="px-6 pt-2 pb-0 flex flex-wrap gap-3 items-center" data-testid="filter-toolbar">
        <div className="flex items-center gap-2">
          <span className="text-xs text-muted-foreground">Kritikalität:</span>
          <Select
            value={filterCriticality || '_all_'}
            onValueChange={(v) => { setFilterCriticality(v === '_all_' ? '' : v); }}
            data-testid="criticality-filter"
          >
            <SelectTrigger className="h-8 w-36 text-xs">
              <SelectValue placeholder="Alle" />
            </SelectTrigger>
            <SelectContent>
              <SelectItem value="_all_">Alle</SelectItem>
              <SelectItem value="standard">Standard</SelectItem>
              <SelectItem value="important">Wichtig</SelectItem>
              <SelectItem value="critical">Kritisch</SelectItem>
            </SelectContent>
          </Select>
        </div>
        <div className="flex items-center gap-2">
          <span className="text-xs text-muted-foreground">Bewertungsstatus:</span>
          <Select
            value={filterAssessmentStatus || '_all_'}
            onValueChange={(v) => { setFilterAssessmentStatus(v === '_all_' ? '' : v); }}
            data-testid="assessment-status-filter"
          >
            <SelectTrigger className="h-8 w-44 text-xs">
              <SelectValue placeholder="Alle" />
            </SelectTrigger>
            <SelectContent>
              <SelectItem value="_all_">Alle</SelectItem>
              <SelectItem value="none">Nicht bewertet</SelectItem>
              <SelectItem value="pending">Ausstehend</SelectItem>
              <SelectItem value="completed">Abgeschlossen</SelectItem>
            </SelectContent>
          </Select>
        </div>
        {(filterCriticality || filterAssessmentStatus) && (
          <Button
            variant="ghost"
            size="sm"
            className="h-8 text-xs"
            onClick={() => { setFilterCriticality(''); setFilterAssessmentStatus('') }}
          >
            Filter zurücksetzen
          </Button>
        )}
      </div>

      <div className="flex-1 p-6">
        {isLoading && <SkeletonCardGrid count={6} />}
        {isError && (
          <div className="text-sm text-red-400 p-4 bg-red-500/10 rounded-lg">Fehler beim Laden des Lieferanten-Registers.</div>
        )}
        {!isLoading && !isError && suppliers?.length === 0 && (
          <EmptyState
            icon={Building2}
            title="Noch keine Lieferanten"
            description="Dokumentiere Drittanbieter und Dienstleister. Die Lieferantenverwaltung hilft dir, Abhängigkeiten und Risiken im Blick zu behalten."
            action={<Button onClick={openCreate}><Plus className="w-4 h-4 mr-1" />Lieferant hinzufügen</Button>}
          />
        )}
        {!isLoading && !isError && suppliers && suppliers.length > 0 && (
          <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 gap-4">
            {suppliers.map((s) => (
              <SupplierCard key={s.id} supplier={s} onEdit={() => { openEdit(s); }} onDelete={() => { handleDelete(s.id); }} />
            ))}
          </div>
        )}
      </div>

      <Dialog open={dialogOpen} onOpenChange={setDialogOpen}>
        <DialogContent className="max-w-lg max-h-[90vh] overflow-y-auto">
          <DialogHeader>
            <DialogTitle>{editId ? 'Lieferant bearbeiten' : 'Lieferant hinzufügen'}</DialogTitle>
          </DialogHeader>
          <div className="space-y-4 py-2">
            <div className="space-y-1.5">
              <Label>Name <span className="text-red-400 text-xs">*</span></Label>
              <Input placeholder="z.B. Cloudflare Inc." value={form.name}
                onChange={(e) => { setForm((f) => ({ ...f, name: e.target.value })); clearSupError('name') }} />
              <FieldError error={supErrors.name ?? null} />
            </div>
            <div className="space-y-1.5">
              <Label>Dienstleistungstyp</Label>
              <Input placeholder="z.B. CDN, Cloud-Speicher, IT-Security" value={form.service_type ?? ''}
                onChange={(e) => { setForm((f) => ({ ...f, service_type: e.target.value })); }} />
            </div>
            <div className="space-y-1.5">
              <Label>Kritikalität</Label>
              <Select value={form.criticality ?? 'standard'} onValueChange={(v) => { setForm((f) => ({ ...f, criticality: v as Supplier['criticality'] })); }}>
                <SelectTrigger><SelectValue /></SelectTrigger>
                <SelectContent>
                  <SelectItem value="standard">Standard</SelectItem>
                  <SelectItem value="important">Wichtig</SelectItem>
                  <SelectItem value="critical">Kritisch</SelectItem>
                </SelectContent>
              </Select>
            </div>
            <div className="space-y-1.5">
              <Label>Bewertungsstatus</Label>
              <Select value={form.assessment_status ?? 'none'} onValueChange={(v) => { setForm((f) => ({ ...f, assessment_status: v as 'none' | 'pending' | 'completed' })); }}>
                <SelectTrigger><SelectValue /></SelectTrigger>
                <SelectContent>
                  <SelectItem value="none">Nicht bewertet</SelectItem>
                  <SelectItem value="pending">Ausstehend</SelectItem>
                  <SelectItem value="completed">Abgeschlossen</SelectItem>
                </SelectContent>
              </Select>
            </div>
            <div className="grid grid-cols-2 gap-3">
              <label className="flex items-center gap-2 text-sm cursor-pointer">
                <input type="checkbox" checked={form.nis2_relevant ?? false}
                  onChange={(e) => { setForm((f) => ({ ...f, nis2_relevant: e.target.checked })); }} />
                NIS2-relevant
              </label>
              <label className="flex items-center gap-2 text-sm cursor-pointer">
                <input type="checkbox" checked={form.dora_relevant ?? false}
                  onChange={(e) => { setForm((f) => ({ ...f, dora_relevant: e.target.checked })); }} />
                DORA-relevant
              </label>
            </div>
            <div className="grid grid-cols-2 gap-3">
              <div className="space-y-1.5">
                <Label>Ansprechpartner</Label>
                <Input placeholder="Name" value={form.contact_name ?? ''}
                  onChange={(e) => { setForm((f) => ({ ...f, contact_name: e.target.value })); }} />
              </div>
              <div className="space-y-1.5">
                <Label>E-Mail</Label>
                <Input placeholder="E-Mail" type="email" value={form.contact_email ?? ''}
                  onChange={(e) => { setForm((f) => ({ ...f, contact_email: e.target.value })); clearSupError('contact_email') }} />
                <FieldError error={supErrors.contact_email ?? null} />
              </div>
            </div>
            <div className="space-y-1.5">
              <Label>Vertragsende</Label>
              <Input type="date" value={form.contract_end ?? ''}
                onChange={(e) => { setForm((f) => ({ ...f, contract_end: e.target.value })); }} />
            </div>
            <div className="space-y-1.5">
              <Label>Notizen</Label>
              <Textarea rows={3} placeholder="Weitere Informationen" value={form.notes ?? ''}
                onChange={(e) => { setForm((f) => ({ ...f, notes: e.target.value })); }} />
            </div>
            <div className="space-y-1.5">
              <Label>Datenspeicherort (DORA)</Label>
              <Select value={form.data_location ?? ''} onValueChange={(v) => { setForm((f) => ({ ...f, data_location: v })); }}>
                <SelectTrigger><SelectValue placeholder="Auswählen …" /></SelectTrigger>
                <SelectContent>
                  <SelectItem value="EU">EU</SelectItem>
                  <SelectItem value="NonEU">NonEU</SelectItem>
                  <SelectItem value="Hybrid">Hybrid</SelectItem>
                </SelectContent>
              </Select>
            </div>
            <div className="space-y-1.5">
              <Label>Unterdienstleister (kommagetrennt)</Label>
              <Textarea rows={2} placeholder="Komma-separiert eingeben" value={subSuppliersRaw}
                onChange={(e) => { setSubSuppliersRaw(e.target.value); }} />
            </div>
            <label className="flex items-center gap-2 text-sm cursor-pointer">
              <input type="checkbox" checked={form.exit_strategy_exists ?? false}
                onChange={(e) => { setForm((f) => ({ ...f, exit_strategy_exists: e.target.checked })); }} />
              Exit-Strategie vorhanden (DORA)
            </label>
          </div>
          <DialogFooter>
            <Button variant="outline" onClick={() => { setDialogOpen(false); }}>Abbrechen</Button>
            <Button onClick={handleSubmit} disabled={isPending}>
              {isPending ? 'Speichern …' : editId ? 'Speichern' : 'Hinzufügen'}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </div>
    </ProGate>
  )
}
