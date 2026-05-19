import { useState } from 'react'
import {
  ClipboardCheck, Plus, ChevronDown, ChevronUp, Trash2, AlertTriangle, CheckSquare,
} from 'lucide-react'
import { PageHeader } from '../../../shared/components/PageHeader'
import { EmptyState } from '../../../shared/components/EmptyState'
import { Pagination } from '../../../shared/components/Pagination'
import { BulkActionBar } from '../../../shared/components/BulkActionBar'
import { FieldError } from '../../../shared/components/FieldError'
import { Button } from '../../../components/ui/button'
import { Badge } from '../../../components/ui/badge'
import { Card } from '../../../components/ui/card'
import { Dialog, DialogContent, DialogHeader, DialogTitle, DialogFooter } from '../../../components/ui/dialog'
import { Input } from '../../../components/ui/input'
import { Label } from '../../../components/ui/label'
import { Textarea } from '../../../components/ui/textarea'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '../../../components/ui/select'
import { useFormValidation } from '../../../shared/hooks/useFormValidation'
import { toast } from '../../../shared/hooks/useToast'
import {
  useCAPAs, useCreateCAPA, useUpdateCAPA, useDeleteCAPA, useBulkUpdateCAPAs,
  type CAPA, type CreateCAPAInput, type UpdateCAPAInput,
} from '../hooks/useCAPAs'

// ---- constants ----

const PRIORITY_CLASS: Record<CAPA['priority'], string> = {
  critical: 'bg-red-500/20 text-red-400 border-red-500/30',
  high:     'bg-orange-500/20 text-orange-400 border-orange-500/30',
  medium:   'bg-amber-500/20 text-amber-400 border-amber-500/30',
  low:      'bg-slate-500/20 text-slate-400 border-slate-500/30',
}
const PRIORITY_LABEL: Record<CAPA['priority'], string> = {
  critical: 'Kritisch', high: 'Hoch', medium: 'Mittel', low: 'Niedrig',
}
const STATUS_CLASS: Record<CAPA['status'], string> = {
  open:          'bg-red-500/20 text-red-400 border-red-500/30',
  in_progress:   'bg-amber-500/20 text-amber-400 border-amber-500/30',
  implemented:   'bg-blue-500/20 text-blue-400 border-blue-500/30',
  verified:      'bg-green-500/20 text-green-400 border-green-500/30',
  closed:        'bg-secondary text-secondary-foreground',
}
const STATUS_LABEL: Record<CAPA['status'], string> = {
  open:          'Offen',
  in_progress:   'In Bearbeitung',
  implemented:   'Umgesetzt',
  verified:      'Verifiziert',
  closed:        'Geschlossen',
}
const SOURCE_LABEL: Record<CAPA['source_type'], string> = {
  audit:    'Audit', incident: 'Vorfall', risk: 'Risiko', manual: 'Manuell',
}

const STATUS_FLOW: CAPA['status'][] = ['open', 'in_progress', 'implemented', 'verified', 'closed']
const NEXT_STATUS_LABEL: Partial<Record<CAPA['status'], string>> = {
  open:        'Als "In Bearbeitung" markieren',
  in_progress: 'Als "Umgesetzt" markieren',
  implemented: 'Als "Verifiziert" markieren',
  verified:    'Als geschlossen markieren',
}

type FilterTab = 'all' | CAPA['status']
const TABS: { key: FilterTab; label: string }[] = [
  { key: 'all',         label: 'Alle' },
  { key: 'open',        label: 'Offen' },
  { key: 'in_progress', label: 'In Bearbeitung' },
  { key: 'implemented', label: 'Umgesetzt' },
  { key: 'verified',    label: 'Verifiziert' },
  { key: 'closed',      label: 'Geschlossen' },
]

// ---- create dialog ----

interface CreateDialogProps {
  open: boolean
  onClose: () => void
  prefillSourceType?: CAPA['source_type']
  prefillSourceId?: string
}

function CreateDialog({ open, onClose, prefillSourceType, prefillSourceId }: CreateDialogProps) {
  const [form, setForm] = useState<CreateCAPAInput>({
    source_type: prefillSourceType ?? 'manual',
    source_id:   prefillSourceId ?? '',
    title:       '',
    description: '',
    assignee_email: '',
    due_date:    null,
    priority:    'medium',
  })
  const create = useCreateCAPA()
  const { errors: capaErrors, validate: validateCapa, clearError: clearCapaError, clearAll: clearCapaErrors } = useFormValidation<Record<string, unknown>>({
    title: { required: true },
  })

  function handleSubmit() {
    if (!validateCapa({ title: form.title })) return
    create.mutate(form, {
      onSuccess: () => {
        setForm({ source_type: 'manual', title: '', description: '', assignee_email: '', priority: 'medium' })
        clearCapaErrors()
        toast('Korrekturmaßnahme erstellt', 'success')
        onClose()
      },
    })
  }

  return (
    <Dialog open={open} onOpenChange={(v) => { if (!v) { clearCapaErrors(); onClose() } }}>
      <DialogContent className="sm:max-w-lg">
        <DialogHeader>
          <DialogTitle>Neue Korrekturmaßnahme</DialogTitle>
        </DialogHeader>
        <div className="space-y-4 py-2">
          <div className="space-y-1.5">
            <Label>Titel <span className="text-red-400 text-xs">*</span></Label>
            <Input value={form.title} onChange={(e) => { setForm((f) => ({ ...f, title: e.target.value })); clearCapaError('title') }} placeholder="Kurzbeschreibung der Maßnahme" />
            <FieldError error={capaErrors.title ?? null} />
          </div>
          <div className="grid grid-cols-2 gap-3">
            <div className="space-y-1.5">
              <Label>Quelltyp</Label>
              <Select value={form.source_type} onValueChange={(v) => setForm((f) => ({ ...f, source_type: v as CAPA['source_type'] }))}>
                <SelectTrigger><SelectValue /></SelectTrigger>
                <SelectContent>
                  <SelectItem value="audit">Audit</SelectItem>
                  <SelectItem value="incident">Vorfall</SelectItem>
                  <SelectItem value="risk">Risiko</SelectItem>
                  <SelectItem value="manual">Manuell</SelectItem>
                </SelectContent>
              </Select>
            </div>
            <div className="space-y-1.5">
              <Label>Priorität</Label>
              <Select value={form.priority ?? 'medium'} onValueChange={(v) => setForm((f) => ({ ...f, priority: v as CAPA['priority'] }))}>
                <SelectTrigger><SelectValue /></SelectTrigger>
                <SelectContent>
                  <SelectItem value="low">Niedrig</SelectItem>
                  <SelectItem value="medium">Mittel</SelectItem>
                  <SelectItem value="high">Hoch</SelectItem>
                  <SelectItem value="critical">Kritisch</SelectItem>
                </SelectContent>
              </Select>
            </div>
          </div>
          <div className="space-y-1.5">
            <Label>Beschreibung</Label>
            <Textarea rows={3} value={form.description ?? ''} onChange={(e) => setForm((f) => ({ ...f, description: e.target.value }))} placeholder="Optionale Beschreibung …" />
          </div>
          <div className="grid grid-cols-2 gap-3">
            <div className="space-y-1.5">
              <Label>Verantwortlicher (E-Mail)</Label>
              <Input type="email" value={form.assignee_email ?? ''} onChange={(e) => setForm((f) => ({ ...f, assignee_email: e.target.value }))} placeholder="max@example.com" />
            </div>
            <div className="space-y-1.5">
              <Label>Fälligkeitsdatum</Label>
              <Input type="date" value={form.due_date ?? ''} onChange={(e) => setForm((f) => ({ ...f, due_date: e.target.value || null }))} />
            </div>
          </div>
        </div>
        <DialogFooter>
          <Button variant="outline" onClick={() => { clearCapaErrors(); onClose() }}>Abbrechen</Button>
          <Button onClick={handleSubmit} disabled={create.isPending}>
            {create.isPending ? 'Erstellen …' : 'Erstellen'}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}

// ---- inline detail panel ----

function CAPADetail({ capa, onClose }: { capa: CAPA; onClose: () => void }) {
  const update = useUpdateCAPA()
  const [rootCause, setRootCause] = useState(capa.root_cause)
  const [actionPlan, setActionPlan] = useState(capa.action_plan)
  const [verificationNote, setVerificationNote] = useState(capa.verification_note)

  const nextStatusIdx = STATUS_FLOW.indexOf(capa.status) + 1
  const nextStatus = nextStatusIdx < STATUS_FLOW.length ? STATUS_FLOW[nextStatusIdx] : null

  function save(patch: UpdateCAPAInput) {
    update.mutate({ id: capa.id, input: patch })
  }

  function advanceStatus() {
    if (!nextStatus) return
    const patch: UpdateCAPAInput = { status: nextStatus }
    if (nextStatus === 'verified' && verificationNote) patch.verification_note = verificationNote
    save(patch)
  }

  function saveText() {
    save({ root_cause: rootCause, action_plan: actionPlan })
  }

  return (
    <div className="border-t border-border bg-muted/20 px-5 py-4 space-y-4">
      <div className="space-y-1.5">
        <Label className="text-xs">Ursachenanalyse</Label>
        <Textarea rows={3} value={rootCause} onChange={(e) => setRootCause(e.target.value)} placeholder="Beschreiben Sie die Grundursache …" />
      </div>
      <div className="space-y-1.5">
        <Label className="text-xs">Maßnahmenplan</Label>
        <Textarea rows={4} value={actionPlan} onChange={(e) => setActionPlan(e.target.value)} placeholder="Beschreiben Sie die geplanten Schritte …" />
      </div>
      {capa.status === 'implemented' && (
        <div className="space-y-1.5">
          <Label className="text-xs">Verifikationsnotiz</Label>
          <Textarea rows={2} value={verificationNote} onChange={(e) => setVerificationNote(e.target.value)} placeholder="Wie wurde die Umsetzung verifiziert?" />
        </div>
      )}
      <div className="flex items-center gap-2">
        <Button size="sm" variant="outline" onClick={saveText} disabled={update.isPending}>Speichern</Button>
        {nextStatus && (
          <Button size="sm" onClick={advanceStatus} disabled={update.isPending}>
            {NEXT_STATUS_LABEL[capa.status]}
          </Button>
        )}
        <Button size="sm" variant="ghost" onClick={onClose} className="ml-auto">Schließen</Button>
      </div>
    </div>
  )
}

// ---- CAPA card ----

function CAPACard({
  capa,
  selected,
  onToggleSelect,
}: {
  capa: CAPA
  selected: boolean
  onToggleSelect: (id: string) => void
}) {
  const [expanded, setExpanded] = useState(false)
  const del = useDeleteCAPA()

  function handleDelete(e: React.MouseEvent) {
    e.stopPropagation()
    if (confirm('Korrekturmaßnahme wirklich löschen?')) del.mutate(capa.id)
  }

  return (
    <Card className={`overflow-hidden${selected ? ' ring-1 ring-brand' : ''}`}>
      {/* WCAG 2.1.1 + 4.1.2: interactive div replaced with button for keyboard + screen-reader support */}
      <div className="flex items-start gap-2 px-4 py-3">
        {/* Checkbox — stops propagation so it doesn't toggle the expand panel */}
        <div className="pt-0.5 shrink-0" onClick={(e) => e.stopPropagation()}>
          <input
            type="checkbox"
            checked={selected}
            onChange={() => onToggleSelect(capa.id)}
            aria-label={`CAPA "${capa.title}" auswählen`}
            className="rounded"
          />
        </div>
      <button
        type="button"
        className="flex-1 min-w-0 text-left flex items-start gap-3 cursor-pointer hover:bg-muted/30 transition-colors focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-brand focus-visible:ring-inset rounded"
        onClick={() => setExpanded((v) => !v)}
        aria-expanded={expanded}
        aria-controls={`capa-detail-${capa.id}`}
      >
        <div className="flex-1 min-w-0">
          <div className="flex flex-wrap items-center gap-1.5 mb-1">
            <Badge variant="outline" className="text-xs">{SOURCE_LABEL[capa.source_type]}</Badge>
            <Badge className={`text-xs ${PRIORITY_CLASS[capa.priority]}`}>{PRIORITY_LABEL[capa.priority]}</Badge>
            <Badge className={`text-xs ${STATUS_CLASS[capa.status]}`}>{STATUS_LABEL[capa.status]}</Badge>
            {capa.due_date && !['closed', 'verified'].includes(capa.status) && new Date(capa.due_date) < new Date() && (
              <Badge variant="destructive" className="text-xs gap-1">
                <AlertTriangle className="w-3 h-3" />
                Überfällig
              </Badge>
            )}
          </div>
          <p className="font-medium text-sm truncate">{capa.title}</p>
          <div className="flex items-center gap-3 mt-1 text-xs text-muted-foreground">
            {capa.assignee_email && <span>{capa.assignee_email}</span>}
            {capa.due_date && (
              <span className={
                !['closed', 'verified'].includes(capa.status) && new Date(capa.due_date) < new Date()
                  ? 'text-red-400 font-medium'
                  : ''
              }>
                Fällig: {new Date(capa.due_date).toLocaleDateString('de-DE')}
              </span>
            )}
          </div>
        </div>
        <div className="flex items-center gap-1 shrink-0">
          <Button size="icon" variant="ghost" className="h-7 w-7 text-destructive hover:text-destructive" onClick={handleDelete} aria-label="CAPA löschen">
            <Trash2 className="w-3.5 h-3.5" aria-hidden="true" />
          </Button>
          {expanded
            ? <ChevronUp className="w-4 h-4 text-muted-foreground" aria-hidden="true" />
            : <ChevronDown className="w-4 h-4 text-muted-foreground" aria-hidden="true" />
          }
        </div>
      </button>
      </div>
      {expanded && (
        <div id={`capa-detail-${capa.id}`}>
          <CAPADetail capa={capa} onClose={() => setExpanded(false)} />
        </div>
      )}
    </Card>
  )
}

// ---- status stepper ----

function StatusStepper({ status }: { status: CAPA['status'] }) {
  const idx = STATUS_FLOW.indexOf(status)
  return (
    <div className="flex items-center gap-0 mb-4">
      {STATUS_FLOW.map((s, i) => (
        <div key={s} className="flex items-center">
          <div className={`px-2 py-0.5 rounded text-xs font-medium ${i <= idx ? STATUS_CLASS[s] : 'bg-muted text-muted-foreground'}`}>
            {STATUS_LABEL[s]}
          </div>
          {i < STATUS_FLOW.length - 1 && (
            <div className={`h-px w-6 ${i < idx ? 'bg-green-500' : 'bg-border'}`} />
          )}
        </div>
      ))}
    </div>
  )
}

// ---- main page ----

export default function CAPAsPage() {
  const [activeTab, setActiveTab] = useState<FilterTab>('all')
  const [createOpen, setCreateOpen] = useState(false)
  const [page, setPage] = useState(1)
  const [selected, setSelected] = useState<Set<string>>(new Set())

  const { data: capas, isLoading, pagination } = useCAPAs(activeTab === 'all' ? undefined : activeTab, page)
  const bulkUpdateCAPAs = useBulkUpdateCAPAs()

  const today = new Date()
  const overdueCAPAs = capas?.filter(
    (c) => c.due_date && !['closed', 'verified'].includes(c.status) && new Date(c.due_date) < today,
  ) ?? []

  function toggleSelect(id: string) {
    setSelected((prev) => {
      const next = new Set(prev)
      if (next.has(id)) next.delete(id)
      else next.add(id)
      return next
    })
  }

  async function handleBulkStatusChange(status: CAPA['status']) {
    if (selected.size === 0) return
    try {
      await bulkUpdateCAPAs.mutateAsync({ ids: Array.from(selected), status })
      setSelected(new Set())
      toast('Status aktualisiert', 'success')
    } catch {
      toast('Bulk-Update fehlgeschlagen', 'error')
    }
  }

  return (
    <div className="flex flex-col h-full">
      <PageHeader
        title="Korrekturmaßnahmen (CAPA)"
        description="Korrektur- und Vorbeugemaßnahmen — ISO 27001 PDCA-Regelkreis"
        actions={
          <div className="flex items-center gap-3">
            {overdueCAPAs.length > 0 && (
              <span className="flex items-center gap-1 text-sm text-red-400 font-medium">
                <AlertTriangle className="w-4 h-4" />
                {overdueCAPAs.length} überfällig
              </span>
            )}
            <Button onClick={() => setCreateOpen(true)}>
              <Plus className="w-4 h-4 mr-1" />
              Neue Korrekturmaßnahme
            </Button>
          </div>
        }
      />

      <div className="flex-1 p-6 space-y-4">
        {/* Filter tabs */}
        <div className="flex gap-1 flex-wrap">
          {TABS.map((tab) => (
            <Button
              key={tab.key}
              variant={activeTab === tab.key ? 'default' : 'outline'}
              size="sm"
              onClick={() => setActiveTab(tab.key)}
            >
              {tab.label}
            </Button>
          ))}
        </div>

        {/* Status stepper — shown when filter active */}
        {activeTab !== 'all' && (
          <StatusStepper status={activeTab as CAPA['status']} />
        )}

        {/* List */}
        {isLoading ? (
          <div className="flex items-center justify-center h-48">
            <div className="w-6 h-6 border-2 border-primary border-t-transparent rounded-full animate-spin" />
          </div>
        ) : !capas || capas.length === 0 ? (
          <EmptyState
            icon={ClipboardCheck}
            title="Keine Korrekturmaßnahmen"
            description="Erstellen Sie eine CAPA aus einem Audit-Befund, einem Vorfall oder manuell."
            action={
              <Button onClick={() => setCreateOpen(true)}>
                <Plus className="w-4 h-4 mr-1" />
                Neue Korrekturmaßnahme
              </Button>
            }
          />
        ) : (
          <div className="space-y-2">
            {capas.map((capa) => (
              <CAPACard
                key={capa.id}
                capa={capa}
                selected={selected.has(capa.id)}
                onToggleSelect={toggleSelect}
              />
            ))}
          </div>
        )}
        <Pagination
          page={page}
          totalPages={pagination?.total_pages ?? 1}
          onPageChange={setPage}
        />
      </div>

      <CreateDialog open={createOpen} onClose={() => setCreateOpen(false)} />

      <BulkActionBar
        selectedCount={selected.size}
        onClearSelection={() => setSelected(new Set())}
        actions={[
          {
            label: 'Abschließen',
            icon: CheckSquare,
            onClick: () => { void handleBulkStatusChange('closed') },
            disabled: bulkUpdateCAPAs.isPending,
          },
          {
            label: 'In Bearbeitung',
            onClick: () => { void handleBulkStatusChange('in_progress') },
            disabled: bulkUpdateCAPAs.isPending,
          },
          {
            label: 'Abbrechen',
            variant: 'destructive' as const,
            onClick: () => { void handleBulkStatusChange('open') },
            disabled: bulkUpdateCAPAs.isPending,
          },
        ]}
      />
    </div>
  )
}
