import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { ListChecks, Plus, Pencil, Trash2, ChevronDown, ChevronUp } from 'lucide-react'
import { Spinner } from '../../../components/Spinner'
import { PageHeader } from '../../../shared/components/PageHeader'
import { EmptyState } from '../../../shared/components/EmptyState'
import { Button } from '../../../components/ui/button'
import { Badge } from '../../../components/ui/badge'
import { Card, CardContent, CardHeader, CardTitle } from '../../../components/ui/card'
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
  DialogFooter,
} from '../../../components/ui/dialog'
import { Input } from '../../../components/ui/input'
import { Label } from '../../../components/ui/label'
import { Textarea } from '../../../components/ui/textarea'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '../../../components/ui/select'
import {
  useRecoveryPlans,
  useCreateRecoveryPlan,
  useUpdateRecoveryPlan,
  useDeleteRecoveryPlan,
} from '../hooks/useRecoveryPlans'
import { useBIAProcesses } from '../hooks/useBIA'
import { TermTooltip } from '../../../shared/components/TermTooltip'
import type { RecoveryPlan, RecoveryStep, CreateRecoveryPlanInput } from '../types'

const STATUS_CLASS: Record<RecoveryPlan['status'], string> = {
  draft: 'bg-secondary text-secondary-foreground',
  active: 'bg-blue-500/20 text-blue-400 border-blue-500/30',
  tested: 'bg-green-500/20 text-green-400 border-green-500/30',
}

function emptyForm(): CreateRecoveryPlanInput {
  return {
    title: '',
    bia_process_id: '',
    activation_criteria: '',
    responsible: '',
    rto_hours: 4,
    status: 'draft',
    steps: [],
  }
}

function planToForm(p: RecoveryPlan): CreateRecoveryPlanInput {
  return {
    title: p.title,
    bia_process_id: p.bia_process_id ?? '',
    activation_criteria: p.activation_criteria,
    responsible: p.responsible,
    rto_hours: p.rto_hours,
    status: p.status,
    steps: p.steps,
  }
}

function StepsList({ steps }: { steps: RecoveryStep[] }) {
  const [open, setOpen] = useState(false)
  if (steps.length === 0) return null
  return (
    <div className="mt-2 border-t pt-2">
      <button
        className="flex items-center gap-1 text-xs text-muted-foreground hover:text-foreground"
        onClick={() => { setOpen((v) => !v) }}
      >
        {open ? <ChevronUp className="w-3 h-3" /> : <ChevronDown className="w-3 h-3" />}
        {steps.length} Schritte
      </button>
      {open && (
        <ol className="mt-2 space-y-1">
          {steps.map((s) => (
            <li key={s.order} className="text-xs flex gap-2">
              <span className="text-muted-foreground w-4 shrink-0">{s.order}.</span>
              <span className="flex-1">{s.action}</span>
              <span className="text-muted-foreground shrink-0">{s.responsible}</span>
              {s.duration_min > 0 && (
                <span className="text-muted-foreground shrink-0">{s.duration_min}min</span>
              )}
            </li>
          ))}
        </ol>
      )}
    </div>
  )
}

function PlanCard({
  plan,
  onEdit,
  onDelete,
}: {
  plan: RecoveryPlan
  onEdit: () => void
  onDelete: () => void
}) {
  const { t } = useTranslation()
  return (
    <Card>
      <CardHeader className="pb-2">
        <div className="flex items-start justify-between gap-2">
          <div className="space-y-1 flex-1 min-w-0">
            <CardTitle className="text-sm leading-tight">{plan.title}</CardTitle>
            <div className="flex items-center gap-1.5 flex-wrap">
              <Badge className={STATUS_CLASS[plan.status]} variant="outline">
                {t(`bcm.recoveryPlans.status.${plan.status}`)}
              </Badge>
              <span className="text-xs text-muted-foreground"><TermTooltip term="RTO" glossaryKey="RTO">RTO</TermTooltip>: {plan.rto_hours}h</span>
              {plan.bia_process_name && (
                <span className="text-xs text-muted-foreground truncate max-w-[120px]">
                  {plan.bia_process_name}
                </span>
              )}
            </div>
          </div>
          <div className="flex items-center gap-1 shrink-0">
            <Button variant="ghost" size="icon" className="h-7 w-7" onClick={onEdit}>
              <Pencil className="w-3.5 h-3.5" />
            </Button>
            <Button
              variant="ghost"
              size="icon"
              className="h-7 w-7 text-red-400 hover:text-red-300"
              onClick={onDelete}
            >
              <Trash2 className="w-3.5 h-3.5" />
            </Button>
          </div>
        </div>
      </CardHeader>
      <CardContent className="pt-0 space-y-1">
        {plan.activation_criteria && (
          <p className="text-xs text-muted-foreground line-clamp-2">{plan.activation_criteria}</p>
        )}
        {plan.responsible && (
          <p className="text-xs text-muted-foreground">{t('bcm.recoveryPlans.responsible')}: {plan.responsible}</p>
        )}
        {plan.last_tested_at && (
          <p className="text-xs text-muted-foreground">
            {t('bcm.recoveryPlans.lastTested')}: {plan.last_tested_at.slice(0, 10)}
          </p>
        )}
        <StepsList steps={plan.steps ?? []} />
      </CardContent>
    </Card>
  )
}

function StepsEditor({
  steps,
  onChange,
}: {
  steps: RecoveryStep[]
  onChange: (steps: RecoveryStep[]) => void
}) {
  const { t } = useTranslation()

  function addStep() {
    const nextOrder = steps.length > 0 ? steps[steps.length - 1].order + 1 : 1
    onChange([...steps, { order: nextOrder, action: '', responsible: '', duration_min: 0 }])
  }

  function updateStep(idx: number, field: keyof RecoveryStep, value: string | number) {
    const next = steps.map((s, i) => i === idx ? { ...s, [field]: value } : s)
    onChange(next)
  }

  function removeStep(idx: number) {
    onChange(steps.filter((_, i) => i !== idx).map((s, i) => ({ ...s, order: i + 1 })))
  }

  return (
    <div className="space-y-2">
      <div className="flex items-center justify-between">
        <Label>{t('bcm.recoveryPlans.steps')}</Label>
        <Button type="button" variant="outline" size="sm" className="h-6 text-xs" onClick={addStep}>
          <Plus className="w-3 h-3 mr-1" />
          {t('bcm.recoveryPlans.addStep')}
        </Button>
      </div>
      {steps.map((step, idx) => (
        <div key={step.order} className="flex gap-2 items-start p-2 rounded bg-muted/30">
          <span className="text-xs text-muted-foreground w-5 mt-2 shrink-0">{step.order}.</span>
          <div className="flex-1 grid grid-cols-2 gap-1.5">
            <Input
              className="h-7 text-xs col-span-2"
              placeholder={t('bcm.recoveryPlans.stepAction')}
              value={step.action}
              onChange={(e) => { updateStep(idx, 'action', e.target.value) }}
            />
            <Input
              className="h-7 text-xs"
              placeholder={t('bcm.recoveryPlans.stepResponsible')}
              value={step.responsible}
              onChange={(e) => { updateStep(idx, 'responsible', e.target.value) }}
            />
            <Input
              className="h-7 text-xs"
              type="number"
              min={0}
              placeholder="min"
              value={step.duration_min || ''}
              onChange={(e) => { updateStep(idx, 'duration_min', Number(e.target.value)) }}
            />
          </div>
          <Button
            type="button"
            variant="ghost"
            size="icon"
            className="h-7 w-7 text-red-400 hover:text-red-300 shrink-0"
            onClick={() => { removeStep(idx) }}
          >
            <Trash2 className="w-3 h-3" />
          </Button>
        </div>
      ))}
    </div>
  )
}

export default function RecoveryPlansPage() {
  const { t } = useTranslation()
  const [dialogOpen, setDialogOpen] = useState(false)
  const [editId, setEditId] = useState<string | null>(null)
  const [form, setForm] = useState<CreateRecoveryPlanInput>(emptyForm())

  const { data: plans = [], isLoading, isError } = useRecoveryPlans()
  const { data: biaProcesses = [] } = useBIAProcesses()
  const create = useCreateRecoveryPlan()
  const update = useUpdateRecoveryPlan(editId ?? '')
  const del = useDeleteRecoveryPlan()

  function openCreate() {
    setEditId(null)
    setForm(emptyForm())
    setDialogOpen(true)
  }

  function openEdit(p: RecoveryPlan) {
    setEditId(p.id)
    setForm(planToForm(p))
    setDialogOpen(true)
  }

  function handleDelete(id: string) {
    if (confirm(t('bcm.recoveryPlans.deleteConfirm'))) {
      del.mutate(id)
    }
  }

  function handleSubmit() {
    const payload = { ...form, bia_process_id: form.bia_process_id || undefined }
    if (editId) {
      update.mutate(payload, { onSuccess: () => { setDialogOpen(false) } })
    } else {
      create.mutate(payload, { onSuccess: () => { setDialogOpen(false) } })
    }
  }

  const isPending = create.isPending || update.isPending

  return (
    <div className="flex flex-col h-full">
      <PageHeader
        title={t('bcm.recoveryPlans.title')}
        description={t('bcm.recoveryPlans.description')}
        actions={
          <Button onClick={openCreate}>
            <Plus className="w-4 h-4 mr-1" />
            {t('bcm.recoveryPlans.new')}
          </Button>
        }
      />

      <div className="flex-1 p-6">
        {isLoading && (
          <div className="flex items-center justify-center h-48">
            <Spinner size="lg" color="primary" />
          </div>
        )}
        {isError && (
          <div className="text-sm text-red-400 p-4 bg-red-500/10 rounded-lg">
            {t('bcm.recoveryPlans.loadError')}
          </div>
        )}
        {!isLoading && !isError && plans.length === 0 && (
          <EmptyState
            icon={ListChecks}
            title={t('bcm.recoveryPlans.emptyTitle')}
            description={t('bcm.recoveryPlans.emptyDescription')}
            action={
              <Button onClick={openCreate}>
                <Plus className="w-4 h-4 mr-1" />
                {t('bcm.recoveryPlans.new')}
              </Button>
            }
          />
        )}
        {!isLoading && !isError && plans.length > 0 && (
          <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 gap-4">
            {plans.map((p) => (
              <PlanCard
                key={p.id}
                plan={p}
                onEdit={() => { openEdit(p) }}
                onDelete={() => { handleDelete(p.id) }}
              />
            ))}
          </div>
        )}
      </div>

      <Dialog open={dialogOpen} onOpenChange={setDialogOpen}>
        <DialogContent className="max-w-xl max-h-[90vh] overflow-y-auto">
          <DialogHeader>
            <DialogTitle>
              {editId ? t('bcm.recoveryPlans.edit') : t('bcm.recoveryPlans.new')}
            </DialogTitle>
          </DialogHeader>
          <div className="space-y-3 py-2">
            <div className="space-y-1.5">
              <Label>{t('bcm.recoveryPlans.planTitle')} *</Label>
              <Input
                value={form.title}
                placeholder={t('bcm.recoveryPlans.planTitlePlaceholder')}
                onChange={(e) => { setForm((f) => ({ ...f, title: e.target.value })) }}
              />
            </div>
            <div className="grid grid-cols-2 gap-3">
              <div className="space-y-1.5">
                <Label>{t('bcm.recoveryPlans.biaProcess')}</Label>
                <Select
                  value={form.bia_process_id ?? ''}
                  onValueChange={(v) => { setForm((f) => ({ ...f, bia_process_id: v === '__none' ? '' : v })) }}
                >
                  <SelectTrigger><SelectValue placeholder={t('bcm.recoveryPlans.noBiaProcess')} /></SelectTrigger>
                  <SelectContent>
                    <SelectItem value="__none">{t('bcm.recoveryPlans.noBiaProcess')}</SelectItem>
                    {biaProcesses.map((p) => (
                      <SelectItem key={p.id} value={p.id}>{p.name}</SelectItem>
                    ))}
                  </SelectContent>
                </Select>
              </div>
              <div className="space-y-1.5">
                <Label>{t('bcm.recoveryPlans.statusLabel')}</Label>
                <Select
                  value={form.status ?? 'draft'}
                  onValueChange={(v) => { setForm((f) => ({ ...f, status: v as RecoveryPlan['status'] })) }}
                >
                  <SelectTrigger><SelectValue /></SelectTrigger>
                  <SelectContent>
                    <SelectItem value="draft">{t('bcm.recoveryPlans.status.draft')}</SelectItem>
                    <SelectItem value="active">{t('bcm.recoveryPlans.status.active')}</SelectItem>
                    <SelectItem value="tested">{t('bcm.recoveryPlans.status.tested')}</SelectItem>
                  </SelectContent>
                </Select>
              </div>
            </div>
            <div className="grid grid-cols-2 gap-3">
              <div className="space-y-1.5">
                <Label>RTO (h)</Label>
                <Input
                  type="number"
                  min={0}
                  value={form.rto_hours ?? 4}
                  onChange={(e) => { setForm((f) => ({ ...f, rto_hours: Number(e.target.value) })) }}
                />
              </div>
              <div className="space-y-1.5">
                <Label>{t('bcm.recoveryPlans.responsible')}</Label>
                <Input
                  value={form.responsible ?? ''}
                  onChange={(e) => { setForm((f) => ({ ...f, responsible: e.target.value })) }}
                />
              </div>
            </div>
            <div className="space-y-1.5">
              <Label>{t('bcm.recoveryPlans.activationCriteria')}</Label>
              <Textarea
                rows={2}
                value={form.activation_criteria ?? ''}
                onChange={(e) => { setForm((f) => ({ ...f, activation_criteria: e.target.value })) }}
              />
            </div>
            <StepsEditor
              steps={form.steps ?? []}
              onChange={(steps) => { setForm((f) => ({ ...f, steps })) }}
            />
          </div>
          <DialogFooter>
            <Button variant="outline" onClick={() => { setDialogOpen(false) }}>
              {t('common.cancel')}
            </Button>
            <Button onClick={handleSubmit} disabled={!form.title || isPending}>
              {isPending ? t('common.saving') : editId ? t('common.save') : t('common.add')}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </div>
  )
}
