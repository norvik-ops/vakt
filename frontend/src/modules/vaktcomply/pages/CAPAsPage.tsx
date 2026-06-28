import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import {
  ClipboardCheck, Plus, ChevronDown, ChevronUp, Trash2, AlertTriangle, CheckSquare,
} from 'lucide-react'
import { Spinner } from '../../../components/Spinner'
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
import { useFormatDate } from '../../../shared/hooks/useFormatDate'
import {
  useCAPAs, useCreateCAPA, useUpdateCAPA, useDeleteCAPA, useBulkUpdateCAPAs,
  useUpdateCAPANCFields, useCompleteEffectivenessCheck,
  type CAPA, type CreateCAPAInput, type UpdateCAPAInput, type CAPANCFields,
} from '../hooks/useCAPAs'

// ---- S61-3: NC classification badge helpers ----

const NC_CLASS_COLOR: Record<NonNullable<CAPA['nc_classification']>, string> = {
  major_nc:    'bg-red-500/20 text-red-400 border-red-500/30',
  minor_nc:    'bg-orange-500/20 text-orange-400 border-orange-500/30',
  observation: 'bg-blue-500/20 text-blue-400 border-blue-500/30',
  ofi:         'bg-slate-500/20 text-slate-400 border-slate-500/30',
}

// ---- constants ----

const PRIORITY_CLASS: Record<CAPA['priority'], string> = {
  critical: 'bg-red-500/20 text-red-400 border-red-500/30',
  high:     'bg-orange-500/20 text-orange-400 border-orange-500/30',
  medium:   'bg-amber-500/20 text-amber-400 border-amber-500/30',
  low:      'bg-slate-500/20 text-slate-400 border-slate-500/30',
}
const STATUS_CLASS: Record<CAPA['status'], string> = {
  open:          'bg-red-500/20 text-red-400 border-red-500/30',
  in_progress:   'bg-amber-500/20 text-amber-400 border-amber-500/30',
  implemented:   'bg-blue-500/20 text-blue-400 border-blue-500/30',
  verified:      'bg-green-500/20 text-green-400 border-green-500/30',
  closed:        'bg-secondary text-secondary-foreground',
}

const STATUS_FLOW: CAPA['status'][] = ['open', 'in_progress', 'implemented', 'verified', 'closed']

type FilterTab = 'all' | CAPA['status']

// ---- create dialog ----

interface CreateDialogProps {
  open: boolean
  onClose: () => void
  prefillSourceType?: CAPA['source_type']
  prefillSourceId?: string
}

function CreateDialog({ open, onClose, prefillSourceType, prefillSourceId }: CreateDialogProps) {
  const { t } = useTranslation()
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
        toast(t('capas.toastCreated'), 'success')
        onClose()
      },
    })
  }

  return (
    <Dialog open={open} onOpenChange={(v) => { if (!v) { clearCapaErrors(); onClose() } }}>
      <DialogContent className="sm:max-w-lg">
        <DialogHeader>
          <DialogTitle>{t('capas.createDialogTitle')}</DialogTitle>
        </DialogHeader>
        <div className="space-y-4 py-2">
          <div className="space-y-1.5">
            <Label>{t('capas.labelTitle')} <span className="text-red-400 text-xs">*</span></Label>
            <Input value={form.title} onChange={(e) => { setForm((f) => ({ ...f, title: e.target.value })); clearCapaError('title') }} placeholder={t('capas.titlePlaceholder')} />
            <FieldError error={capaErrors.title ?? null} />
          </div>
          <div className="grid grid-cols-2 gap-3">
            <div className="space-y-1.5">
              <Label>{t('capas.labelSource')}</Label>
              <Select value={form.source_type} onValueChange={(v) => { setForm((f) => ({ ...f, source_type: v as CAPA['source_type'] })); }}>
                <SelectTrigger><SelectValue /></SelectTrigger>
                <SelectContent>
                  <SelectItem value="audit">{t('capas.source_audit')}</SelectItem>
                  <SelectItem value="incident">{t('capas.source_incident')}</SelectItem>
                  <SelectItem value="risk">{t('capas.source_risk')}</SelectItem>
                  <SelectItem value="manual">{t('capas.source_manual')}</SelectItem>
                </SelectContent>
              </Select>
            </div>
            <div className="space-y-1.5">
              <Label>{t('capas.labelPriority')}</Label>
              <Select value={form.priority ?? 'medium'} onValueChange={(v) => { setForm((f) => ({ ...f, priority: v as CAPA['priority'] })); }}>
                <SelectTrigger><SelectValue /></SelectTrigger>
                <SelectContent>
                  <SelectItem value="low">{t('capas.priority_low')}</SelectItem>
                  <SelectItem value="medium">{t('capas.priority_medium')}</SelectItem>
                  <SelectItem value="high">{t('capas.priority_high')}</SelectItem>
                  <SelectItem value="critical">{t('capas.priority_critical')}</SelectItem>
                </SelectContent>
              </Select>
            </div>
          </div>
          <div className="space-y-1.5">
            <Label>{t('common.description')}</Label>
            <Textarea rows={3} value={form.description ?? ''} onChange={(e) => { setForm((f) => ({ ...f, description: e.target.value })); }} placeholder={t('capas.descPlaceholder')} />
          </div>
          <div className="grid grid-cols-2 gap-3">
            <div className="space-y-1.5">
              <Label>{t('capas.labelOwner')}</Label>
              <Input type="email" value={form.assignee_email ?? ''} onChange={(e) => { setForm((f) => ({ ...f, assignee_email: e.target.value })); }} placeholder="max@example.com" />
            </div>
            <div className="space-y-1.5">
              <Label>{t('capas.labelDueDate')}</Label>
              <Input type="date" value={form.due_date ?? ''} onChange={(e) => { setForm((f) => ({ ...f, due_date: e.target.value || null })); }} />
            </div>
          </div>
        </div>
        <DialogFooter>
          <Button variant="outline" onClick={() => { clearCapaErrors(); onClose() }}>{t('common.cancel')}</Button>
          <Button onClick={handleSubmit} disabled={create.isPending}>
            {create.isPending ? t('capas.creating') : t('common.create')}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}

// ---- inline detail panel ----

function CAPADetail({ capa, onClose }: { capa: CAPA; onClose: () => void }) {
  const { t } = useTranslation()
  const update = useUpdateCAPA()
  const updateNCFields = useUpdateCAPANCFields(capa.id)
  const completeEffectiveness = useCompleteEffectivenessCheck(capa.id)
  const [rootCause, setRootCause] = useState(capa.root_cause)
  const [actionPlan, setActionPlan] = useState(capa.action_plan)
  const [verificationNote, setVerificationNote] = useState(capa.verification_note)
  // NC fields
  const [ncClassification, setNcClassification] = useState<CAPANCFields['nc_classification']>(capa.nc_classification)
  const [immediateContainment, setImmediateContainment] = useState(capa.immediate_containment ?? '')
  const [similarNcsNotes, setSimilarNcsNotes] = useState(capa.similar_ncs_notes ?? '')
  const [effectivenessCheckDate, setEffectivenessCheckDate] = useState(capa.effectiveness_check_date ?? '')
  const [effectivenessEvidenceNote, setEffectivenessEvidenceNote] = useState('')

  const nextStatusIdx = STATUS_FLOW.indexOf(capa.status) + 1
  const nextStatus = nextStatusIdx < STATUS_FLOW.length ? STATUS_FLOW[nextStatusIdx] : null

  const NEXT_STATUS_LABEL: Partial<Record<CAPA['status'], string>> = {
    open:        t('capas.advance_open'),
    in_progress: t('capas.advance_in_progress'),
    implemented: t('capas.advance_implemented'),
    verified:    t('capas.advance_verified'),
  }

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

  function saveNCFields() {
    updateNCFields.mutate({
      nc_classification: ncClassification,
      immediate_containment: immediateContainment,
      root_cause: rootCause,
      similar_ncs_notes: similarNcsNotes,
      effectiveness_check_date: effectivenessCheckDate || undefined,
      effectiveness_evidence: '',
    }, {
      onSuccess: () => { toast(t('capas.toastNcSaved'), 'success') },
    })
  }

  function confirmEffectiveness(confirmed: boolean) {
    completeEffectiveness.mutate({ confirmed, evidence_note: effectivenessEvidenceNote }, {
      onSuccess: () => { toast(confirmed ? t('capas.toastEffectiveConfirmed') : t('capas.toastEffectiveNotConfirmed'), 'success') },
    })
  }

  return (
    <div className="border-t border-border bg-muted/20 px-5 py-4 space-y-4">
      <div className="space-y-1.5">
        <Label className="text-xs">{t('capas.rootCause')}</Label>
        <Textarea rows={3} value={rootCause} onChange={(e) => { setRootCause(e.target.value); }} placeholder={t('capas.rootCausePlaceholder')} />
      </div>
      <div className="space-y-1.5">
        <Label className="text-xs">{t('capas.actionPlan')}</Label>
        <Textarea rows={4} value={actionPlan} onChange={(e) => { setActionPlan(e.target.value); }} placeholder={t('capas.actionPlanPlaceholder')} />
      </div>
      {capa.status === 'implemented' && (
        <div className="space-y-1.5">
          <Label className="text-xs">{t('capas.verificationNote')}</Label>
          <Textarea rows={2} value={verificationNote} onChange={(e) => { setVerificationNote(e.target.value); }} placeholder={t('capas.verificationPlaceholder')} />
        </div>
      )}
      <div className="flex items-center gap-2">
        <Button size="sm" variant="outline" onClick={saveText} disabled={update.isPending}>{t('common.save')}</Button>
        {nextStatus && (
          <Button size="sm" onClick={advanceStatus} disabled={update.isPending}>
            {NEXT_STATUS_LABEL[capa.status]}
          </Button>
        )}
        <Button size="sm" variant="ghost" onClick={onClose} className="ml-auto">{t('common.close')}</Button>
      </div>

      {/* S61-3: NC root-cause + effectiveness section */}
      <div className="border-t border-border pt-3 space-y-3">
        <p className="text-xs font-semibold text-muted-foreground uppercase tracking-wide">{t('capas.ncSection')}</p>
        <div className="grid grid-cols-2 gap-3">
          <div className="space-y-1.5">
            <Label className="text-xs">{t('capas.ncClass')}</Label>
            <Select value={ncClassification ?? ''} onValueChange={(v) => { setNcClassification((v as CAPANCFields['nc_classification']) || undefined); }}>
              <SelectTrigger className="h-8 text-xs"><SelectValue placeholder={t('capas.notClassified')} /></SelectTrigger>
              <SelectContent>
                <SelectItem value="">{t('capas.notClassified')}</SelectItem>
                <SelectItem value="major_nc">{t('capas.ncClass_major')}</SelectItem>
                <SelectItem value="minor_nc">{t('capas.ncClass_minor')}</SelectItem>
                <SelectItem value="observation">{t('capas.ncClass_observation')}</SelectItem>
                <SelectItem value="ofi">{t('capas.ncClass_ofi')}</SelectItem>
              </SelectContent>
            </Select>
          </div>
          <div className="space-y-1.5">
            <Label className="text-xs">{t('capas.effectivenessDeadline')}</Label>
            <Input type="date" className="h-8 text-xs" value={effectivenessCheckDate} onChange={(e) => { setEffectivenessCheckDate(e.target.value); }} />
          </div>
        </div>
        <div className="space-y-1.5">
          <Label className="text-xs">{t('capas.immediateAction')}</Label>
          <Textarea rows={2} className="text-xs" value={immediateContainment} onChange={(e) => { setImmediateContainment(e.target.value); }} placeholder={t('capas.immediateActionPlaceholder')} />
        </div>
        <div className="space-y-1.5">
          <Label className="text-xs">{t('capas.similarNcs')}</Label>
          <Textarea rows={2} className="text-xs" value={similarNcsNotes} onChange={(e) => { setSimilarNcsNotes(e.target.value); }} placeholder={t('capas.similarNcsPlaceholder')} />
        </div>
        <Button size="sm" variant="outline" onClick={saveNCFields} disabled={updateNCFields.isPending}>{t('capas.saveNcFields')}</Button>

        {capa.effectiveness_check_date && capa.effectiveness_confirmed == null && (
          <div className="space-y-2 border border-border rounded p-3">
            <p className="text-xs font-medium">{t('capas.effectivenessCheck')}</p>
            <div className="space-y-1.5">
              <Label className="text-xs">{t('capas.evidenceNote')}</Label>
              <Textarea rows={2} className="text-xs" value={effectivenessEvidenceNote} onChange={(e) => { setEffectivenessEvidenceNote(e.target.value); }} placeholder={t('capas.evidencePlaceholder')} />
            </div>
            <div className="flex gap-2">
              <Button size="sm" onClick={() => { confirmEffectiveness(true); }} disabled={completeEffectiveness.isPending}>{t('capas.confirmEffective')}</Button>
              <Button size="sm" variant="outline" onClick={() => { confirmEffectiveness(false); }} disabled={completeEffectiveness.isPending}>{t('capas.notEffective')}</Button>
            </div>
          </div>
        )}
        {capa.effectiveness_confirmed === true && (
          <p className="text-xs text-green-400">{t('capas.effectivenessConfirmed')}</p>
        )}
        {capa.effectiveness_confirmed === false && (
          <p className="text-xs text-red-400">{t('capas.effectivenessNotConfirmed')}</p>
        )}
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
  const { t } = useTranslation()
  const [expanded, setExpanded] = useState(false)
  const del = useDeleteCAPA()
  const { formatDate } = useFormatDate()

  const SOURCE_LABEL: Record<CAPA['source_type'], string> = {
    audit:    t('capas.source_audit'),
    incident: t('capas.source_incident'),
    risk:     t('capas.source_risk'),
    manual:   t('capas.source_manual'),
  }
  const PRIORITY_LABEL: Record<CAPA['priority'], string> = {
    critical: t('capas.priority_critical'),
    high:     t('capas.priority_high'),
    medium:   t('capas.priority_medium'),
    low:      t('capas.priority_low'),
  }
  const STATUS_LABEL: Record<CAPA['status'], string> = {
    open:        t('capas.status_open'),
    in_progress: t('capas.status_in_progress'),
    implemented: t('capas.status_implemented'),
    verified:    t('capas.status_verified'),
    closed:      t('capas.status_closed'),
  }
  const NC_CLASS_LABEL: Record<NonNullable<CAPA['nc_classification']>, string> = {
    major_nc:    t('capas.ncClass_major'),
    minor_nc:    t('capas.ncClass_minor'),
    observation: t('capas.ncClass_observation'),
    ofi:         t('capas.ncClass_ofi'),
  }

  function handleDelete(e: React.MouseEvent) {
    e.stopPropagation()
    if (confirm(t('capas.deleteConfirm'))) del.mutate(capa.id)
  }

  return (
    <Card className={`overflow-hidden${selected ? ' ring-1 ring-brand' : ''}`}>
      {/* WCAG 2.1.1 + 4.1.2: interactive div replaced with button for keyboard + screen-reader support */}
      <div className="flex items-start gap-2 px-4 py-3">
        {/* Checkbox — stops propagation so it doesn't toggle the expand panel */}
        <div className="pt-0.5 shrink-0" onClick={(e) => { e.stopPropagation(); }}>
          <input
            type="checkbox"
            checked={selected}
            onChange={() => { onToggleSelect(capa.id); }}
            aria-label={t('capas.cardAriaLabel', { title: capa.title })}
            className="rounded"
          />
        </div>
      <button
        type="button"
        className="flex-1 min-w-0 text-left flex items-start gap-3 cursor-pointer hover:bg-muted/30 transition-colors focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-brand focus-visible:ring-inset rounded"
        onClick={() => { setExpanded((v) => !v); }}
        aria-expanded={expanded}
        aria-controls={`capa-detail-${capa.id}`}
      >
        <div className="flex-1 min-w-0">
          <div className="flex flex-wrap items-center gap-1.5 mb-1">
            <Badge variant="outline" className="text-xs">{SOURCE_LABEL[capa.source_type]}</Badge>
            <Badge className={`text-xs ${PRIORITY_CLASS[capa.priority]}`}>{PRIORITY_LABEL[capa.priority]}</Badge>
            <Badge className={`text-xs ${STATUS_CLASS[capa.status]}`}>{STATUS_LABEL[capa.status]}</Badge>
            {capa.nc_classification && (
              <Badge className={`text-xs ${NC_CLASS_COLOR[capa.nc_classification]}`}>
                {NC_CLASS_LABEL[capa.nc_classification]}
              </Badge>
            )}
            {capa.due_date && !['closed', 'verified'].includes(capa.status) && new Date(capa.due_date) < new Date() && (
              <Badge variant="destructive" className="text-xs gap-1">
                <AlertTriangle className="w-3 h-3" />
                {t('capas.overdue')}
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
                {t('capas.duePrefix')}{formatDate(capa.due_date)}
              </span>
            )}
          </div>
        </div>
        <div className="flex items-center gap-1 shrink-0">
          <Button size="icon" variant="ghost" className="h-7 w-7 text-destructive hover:text-destructive" onClick={handleDelete} aria-label={t('capas.deleteAria')}>
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
          <CAPADetail capa={capa} onClose={() => { setExpanded(false); }} />
        </div>
      )}
    </Card>
  )
}

// ---- status stepper ----

function StatusStepper({ status }: { status: CAPA['status'] }) {
  const { t } = useTranslation()
  const STATUS_LABEL: Record<CAPA['status'], string> = {
    open:        t('capas.status_open'),
    in_progress: t('capas.status_in_progress'),
    implemented: t('capas.status_implemented'),
    verified:    t('capas.status_verified'),
    closed:      t('capas.status_closed'),
  }
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
  const { t } = useTranslation()
  const [activeTab, setActiveTab] = useState<FilterTab>('all')
  const [createOpen, setCreateOpen] = useState(false)
  const [page, setPage] = useState(1)
  const [selected, setSelected] = useState<Set<string>>(new Set())

  const TABS: { key: FilterTab; label: string }[] = [
    { key: 'all',         label: t('common.all') },
    { key: 'open',        label: t('capas.status_open') },
    { key: 'in_progress', label: t('capas.status_in_progress') },
    { key: 'implemented', label: t('capas.status_implemented') },
    { key: 'verified',    label: t('capas.status_verified') },
    { key: 'closed',      label: t('capas.status_closed') },
  ]

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
      toast(t('capas.toastStatusUpdated'), 'success')
    } catch {
      toast(t('capas.toastBulkFailed'), 'error')
    }
  }

  return (
    <div className="flex flex-col h-full">
      <PageHeader
        title={t('capas.title')}
        description={t('capas.description')}
        actions={
          <div className="flex items-center gap-3">
            {overdueCAPAs.length > 0 && (
              <span className="flex items-center gap-1 text-sm text-red-400 font-medium">
                <AlertTriangle className="w-4 h-4" />
                {t('capas.overdueCount', { count: overdueCAPAs.length })}
              </span>
            )}
            <Button onClick={() => { setCreateOpen(true); }}>
              <Plus className="w-4 h-4 mr-1" />
              {t('capas.createBtn')}
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
              onClick={() => { setActiveTab(tab.key); }}
            >
              {tab.label}
            </Button>
          ))}
        </div>

        {/* Status stepper — shown when filter active */}
        {activeTab !== 'all' && (
          <StatusStepper status={activeTab} />
        )}

        {/* List */}
        {isLoading ? (
          <div className="flex items-center justify-center h-48">
            <Spinner size="lg" color="primary" />
          </div>
        ) : !capas || capas.length === 0 ? (
          <EmptyState
            icon={ClipboardCheck}
            title={t('capas.emptyTitle')}
            description={t('capas.emptyDesc')}
            action={
              <Button onClick={() => { setCreateOpen(true); }}>
                <Plus className="w-4 h-4 mr-1" />
                {t('capas.createBtn')}
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

      <CreateDialog open={createOpen} onClose={() => { setCreateOpen(false); }} />

      <BulkActionBar
        selectedCount={selected.size}
        onClearSelection={() => { setSelected(new Set()); }}
        actions={[
          {
            label: t('capas.bulkClose'),
            icon: CheckSquare,
            onClick: () => { void handleBulkStatusChange('closed') },
            disabled: bulkUpdateCAPAs.isPending,
          },
          {
            label: t('capas.bulkInProgress'),
            onClick: () => { void handleBulkStatusChange('in_progress') },
            disabled: bulkUpdateCAPAs.isPending,
          },
          {
            label: t('common.cancel'),
            variant: 'destructive' as const,
            onClick: () => { void handleBulkStatusChange('open') },
            disabled: bulkUpdateCAPAs.isPending,
          },
        ]}
      />
    </div>
  )
}
