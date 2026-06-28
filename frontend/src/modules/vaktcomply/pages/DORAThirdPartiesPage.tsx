import { useState } from 'react'
import { Building2, Plus, Pencil, Trash2, AlertTriangle, CheckCircle, Globe } from 'lucide-react'
import { Spinner } from '../../../components/Spinner'
import { PageHeader } from '../../../shared/components/PageHeader'
import { EmptyState } from '../../../shared/components/EmptyState'
import { ProGate } from '../../../shared/components/ProGate'
import { Button } from '../../../components/ui/button'
import { Badge } from '../../../components/ui/badge'
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
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
  DialogFooter,
} from '../../../components/ui/dialog'
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
} from '../../../components/ui/alert-dialog'
import {
  useDORAThirdParties,
  useCreateDORAThirdParty,
  useUpdateDORAThirdParty,
  useDeleteDORAThirdParty,
} from '../hooks/useDORAThirdParties'
import type {
  DORAThirdParty,
  DORAThirdPartyCriticality,
  CreateDORAThirdPartyInput,
} from '../types'
import { useTranslation } from 'react-i18next'

// ── Helpers ──────────────────────────────────────────────────────────────────

function criticalityVariant(c: DORAThirdPartyCriticality): React.ComponentProps<typeof Badge>['variant'] {
  if (c === 'kritisch') return 'destructive'
  if (c === 'wichtig') return 'warning'
  return 'secondary'
}

const SERVICE_TYPES = ['IT-Outsourcing', 'Cloud', 'SaaS', 'Netzwerk', 'Sonstiges'] as const
const CRITICALITIES = ['kritisch', 'wichtig', 'unkritisch'] as const
const DATA_LOCATIONS = ['EU', 'Non-EU', 'Mixed'] as const

const EMPTY_FORM: CreateDORAThirdPartyInput = {
  name: '',
  service_type: 'Cloud',
  criticality: 'wichtig',
  data_location: 'EU',
  has_subcontractors: false,
  exit_strategy: false,
}

// ── Form Dialog ──────────────────────────────────────────────────────────────

function ThirdPartyDialog({
  open,
  onClose,
  initial,
  onSave,
  saving,
}: {
  open: boolean
  onClose: () => void
  initial?: DORAThirdParty | null
  onSave: (input: CreateDORAThirdPartyInput) => void
  saving: boolean
}) {
  const { t } = useTranslation()
  const [form, setForm] = useState<CreateDORAThirdPartyInput>(
    initial
      ? {
          name: initial.name,
          service_type: initial.service_type,
          criticality: initial.criticality,
          contract_start: initial.contract_start ?? null,
          contract_end: initial.contract_end ?? null,
          sla_rto_hours: initial.sla_rto_hours ?? null,
          sla_availability: initial.sla_availability ?? null,
          has_subcontractors: initial.has_subcontractors,
          subcontractor_names: initial.subcontractor_names ?? '',
          data_location: initial.data_location,
          exit_strategy: initial.exit_strategy,
          exit_notes: initial.exit_notes ?? '',
          notes: initial.notes ?? '',
        }
      : EMPTY_FORM,
  )

  const set = <K extends keyof CreateDORAThirdPartyInput>(key: K, val: CreateDORAThirdPartyInput[K]) =>
    { setForm((f) => ({ ...f, [key]: val })); }

  return (
    <Dialog open={open} onOpenChange={(v) => { if (!v) { onClose(); } }}>
      <DialogContent className="max-w-2xl max-h-[90vh] overflow-y-auto">
        <DialogHeader>
          <DialogTitle>{initial ? t('vaktcomply.doraThirdParties.dialogEditTitle') : t('vaktcomply.doraThirdParties.dialogAddTitle')}</DialogTitle>
        </DialogHeader>

        <div className="space-y-4 py-2">
          <div className="grid grid-cols-2 gap-4">
            <div className="col-span-2 space-y-1">
              <Label>{t('vaktcomply.doraThirdParties.labelName')}</Label>
              <Input
                value={form.name}
                onChange={(e) => { set('name', e.target.value); }}
                placeholder={t('vaktcomply.doraThirdParties.placeholderName')}
              />
            </div>

            <div className="space-y-1">
              <Label>{t('vaktcomply.doraThirdParties.labelServiceType')}</Label>
              <Select value={form.service_type} onValueChange={(v) => { set('service_type', v as typeof form.service_type); }}>
                <SelectTrigger><SelectValue /></SelectTrigger>
                <SelectContent>
                  {SERVICE_TYPES.map((t) => <SelectItem key={t} value={t}>{t}</SelectItem>)}
                </SelectContent>
              </Select>
            </div>

            <div className="space-y-1">
              <Label>{t('vaktcomply.doraThirdParties.labelCriticality')}</Label>
              <Select value={form.criticality} onValueChange={(v) => { set('criticality', v as typeof form.criticality); }}>
                <SelectTrigger><SelectValue /></SelectTrigger>
                <SelectContent>
                  {CRITICALITIES.map((c) => <SelectItem key={c} value={c}>{c}</SelectItem>)}
                </SelectContent>
              </Select>
            </div>

            <div className="space-y-1">
              <Label>{t('vaktcomply.doraThirdParties.labelContractStart')}</Label>
              <Input type="date" value={form.contract_start ?? ''} onChange={(e) => { set('contract_start', e.target.value || null); }} />
            </div>

            <div className="space-y-1">
              <Label>{t('vaktcomply.doraThirdParties.labelContractEnd')}</Label>
              <Input type="date" value={form.contract_end ?? ''} onChange={(e) => { set('contract_end', e.target.value || null); }} />
            </div>

            <div className="space-y-1">
              <Label>{t('vaktcomply.doraThirdParties.labelSlaRto')}</Label>
              <Input
                type="number"
                min={0}
                value={form.sla_rto_hours ?? ''}
                onChange={(e) => { set('sla_rto_hours', e.target.value ? Number(e.target.value) : null); }}
                placeholder={t('vaktcomply.doraThirdParties.placeholderSlaRto')}
              />
            </div>

            <div className="space-y-1">
              <Label>{t('vaktcomply.doraThirdParties.labelSlaAvailability')}</Label>
              <Input
                type="number"
                min={0}
                max={100}
                step={0.01}
                value={form.sla_availability ?? ''}
                onChange={(e) => { set('sla_availability', e.target.value ? Number(e.target.value) : null); }}
                placeholder={t('vaktcomply.doraThirdParties.placeholderSlaAvailability')}
              />
            </div>

            <div className="space-y-1">
              <Label>{t('vaktcomply.doraThirdParties.labelDataLocation')}</Label>
              <Select value={form.data_location} onValueChange={(v) => { set('data_location', v as typeof form.data_location); }}>
                <SelectTrigger><SelectValue /></SelectTrigger>
                <SelectContent>
                  {DATA_LOCATIONS.map((l) => <SelectItem key={l} value={l}>{l}</SelectItem>)}
                </SelectContent>
              </Select>
            </div>

            <div className="space-y-1 flex items-center gap-3 pt-6">
              <input
                type="checkbox"
                id="has_sub"
                checked={form.has_subcontractors}
                onChange={(e) => { set('has_subcontractors', e.target.checked); }}
                className="h-4 w-4"
              />
              <Label htmlFor="has_sub">{t('vaktcomply.doraThirdParties.labelHasSubcontractors')}</Label>
            </div>

            {form.has_subcontractors && (
              <div className="col-span-2 space-y-1">
                <Label>{t('vaktcomply.doraThirdParties.labelSubcontractorNames')}</Label>
                <Input
                  value={form.subcontractor_names ?? ''}
                  onChange={(e) => { set('subcontractor_names', e.target.value); }}
                  placeholder={t('vaktcomply.doraThirdParties.placeholderSubcontractorNames')}
                />
              </div>
            )}

            <div className="col-span-2 space-y-1 flex items-center gap-3">
              <input
                type="checkbox"
                id="exit_strat"
                checked={form.exit_strategy}
                onChange={(e) => { set('exit_strategy', e.target.checked); }}
                className="h-4 w-4"
              />
              <Label htmlFor="exit_strat">{t('vaktcomply.doraThirdParties.labelExitStrategy')}</Label>
            </div>

            {form.exit_strategy && (
              <div className="col-span-2 space-y-1">
                <Label>{t('vaktcomply.doraThirdParties.labelExitNotes')}</Label>
                <Textarea
                  value={form.exit_notes ?? ''}
                  onChange={(e) => { set('exit_notes', e.target.value); }}
                  rows={2}
                  placeholder={t('vaktcomply.doraThirdParties.placeholderExitNotes')}
                />
              </div>
            )}

            <div className="col-span-2 space-y-1">
              <Label>{t('vaktcomply.doraThirdParties.labelNotes')}</Label>
              <Textarea
                value={form.notes ?? ''}
                onChange={(e) => { set('notes', e.target.value); }}
                rows={3}
                placeholder={t('vaktcomply.doraThirdParties.placeholderNotes')}
              />
            </div>
          </div>
        </div>

        <DialogFooter>
          <Button variant="outline" onClick={onClose} disabled={saving}>{t('vaktcomply.doraThirdParties.btnCancel')}</Button>
          <Button onClick={() => { onSave(form); }} disabled={saving || !form.name}>
            {saving ? <Spinner size="sm" className="mr-2" /> : null}
            {initial ? t('vaktcomply.doraThirdParties.btnSave') : t('vaktcomply.doraThirdParties.btnAdd')}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}

// ── Row ──────────────────────────────────────────────────────────────────────

function ThirdPartyRow({
  tp,
  onEdit,
  onDelete,
}: {
  tp: DORAThirdParty
  onEdit: () => void
  onDelete: () => void
}) {
  const { t } = useTranslation()
  return (
    <div className="flex items-start justify-between px-4 py-3 hover:bg-surface2 border-b border-border last:border-0">
      <div className="flex items-start gap-3 min-w-0">
        <Building2 className="w-4 h-4 mt-0.5 text-secondary shrink-0" />
        <div className="min-w-0">
          <div className="flex items-center gap-2 flex-wrap">
            <span className="font-medium text-sm truncate">{tp.name}</span>
            <Badge variant={criticalityVariant(tp.criticality)} className="text-xs">{tp.criticality}</Badge>
            <Badge variant="secondary" className="text-xs">{tp.service_type}</Badge>
            <span className="flex items-center gap-1 text-xs text-secondary">
              <Globe className="w-3 h-3" />{tp.data_location}
            </span>
          </div>
          <div className="flex items-center gap-3 mt-1 text-xs text-secondary flex-wrap">
            {tp.contract_end && (
              <span>{t('vaktcomply.doraThirdParties.contractEnd')}: {tp.contract_end}</span>
            )}
            {tp.sla_rto_hours != null && (
              <span>RTO: {tp.sla_rto_hours}h</span>
            )}
            {tp.sla_availability != null && (
              <span>SLA: {tp.sla_availability}%</span>
            )}
            {tp.exit_strategy ? (
              <span className="flex items-center gap-1 text-green-600">
                <CheckCircle className="w-3 h-3" /> {t('vaktcomply.doraThirdParties.exitPlanOk')}
              </span>
            ) : tp.criticality === 'kritisch' ? (
              <span className="flex items-center gap-1 text-amber-600">
                <AlertTriangle className="w-3 h-3" /> {t('vaktcomply.doraThirdParties.exitPlanMissing')}
              </span>
            ) : null}
          </div>
        </div>
      </div>
      <div className="flex items-center gap-1 shrink-0 ml-2">
        <Button variant="ghost" size="sm" onClick={onEdit} title={t('vaktcomply.doraThirdParties.titleEdit')}>
          <Pencil className="w-4 h-4" />
        </Button>
        <Button variant="ghost" size="sm" onClick={onDelete} title={t('vaktcomply.doraThirdParties.titleDelete')} className="text-destructive hover:text-destructive">
          <Trash2 className="w-4 h-4" />
        </Button>
      </div>
    </div>
  )
}

// ── Page ─────────────────────────────────────────────────────────────────────

export default function DORAThirdPartiesPage() {
  const { t } = useTranslation()
  const [criticality, setCriticality] = useState<string>('')
  const { data: list, isLoading, isError, error } = useDORAThirdParties(criticality || undefined)
  const createMut = useCreateDORAThirdParty()
  const deleteMut = useDeleteDORAThirdParty()

  const [dialogOpen, setDialogOpen] = useState(false)
  const [editing, setEditing] = useState<DORAThirdParty | null>(null)
  const [deleteTarget, setDeleteTarget] = useState<DORAThirdParty | null>(null)

  const updateMut = useUpdateDORAThirdParty(editing?.id ?? '')

  function handleSave(input: CreateDORAThirdPartyInput) {
    if (editing) {
      updateMut.mutate(input, {
        onSuccess: () => { setDialogOpen(false); setEditing(null) },
      })
    } else {
      createMut.mutate(input, {
        onSuccess: () => { setDialogOpen(false); },
      })
    }
  }

  const kritisch = list?.filter((tp) => tp.criticality === 'kritisch').length ?? 0
  const noExit = list?.filter((tp) => tp.criticality === 'kritisch' && !tp.exit_strategy).length ?? 0

  return (
    <ProGate error={isError ? error : null}>
      <div className="flex flex-col h-full">
        <PageHeader
          title={t('vaktcomply.doraThirdParties.pageTitle')}
          description={t('vaktcomply.doraThirdParties.pageDescription')}
          actions={
            <Button size="sm" onClick={() => { setEditing(null); setDialogOpen(true) }}>
              <Plus className="w-4 h-4 mr-1" />
              {t('vaktcomply.doraThirdParties.addProvider')}
            </Button>
          }
        />

        <div className="flex-1 p-6 space-y-4">
          {/* Summary badges */}
          {list && list.length > 0 && (
            <div className="flex items-center gap-3 flex-wrap">
              <span className="text-sm text-secondary">{t('vaktcomply.doraThirdParties.entries', { count: list.length })}</span>
              {kritisch > 0 && <Badge variant="destructive">{kritisch} {t('vaktcomply.doraThirdParties.critical')}</Badge>}
              {noExit > 0 && (
                <Badge variant="warning" className="flex items-center gap-1">
                  <AlertTriangle className="w-3 h-3" /> {noExit} {t('vaktcomply.doraThirdParties.criticalNoExit')}
                </Badge>
              )}
            </div>
          )}

          {/* Filter */}
          <div className="flex items-center gap-2">
            <Label className="text-sm shrink-0">{t('vaktcomply.doraThirdParties.filterLabel')}</Label>
            <Select value={criticality || 'all'} onValueChange={(v) => { setCriticality(v === 'all' ? '' : v); }}>
              <SelectTrigger className="w-40">
                <SelectValue placeholder={t('vaktcomply.doraThirdParties.filterAll')} />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value="all">{t('vaktcomply.doraThirdParties.filterAll')}</SelectItem>
                {CRITICALITIES.map((c) => <SelectItem key={c} value={c}>{c}</SelectItem>)}
              </SelectContent>
            </Select>
          </div>

          {isLoading ? (
            <div className="flex items-center justify-center h-32">
              <Spinner size="md" />
            </div>
          ) : !list || list.length === 0 ? (
            <EmptyState
              icon={Building2}
              title={t('vaktcomply.doraThirdParties.emptyTitle')}
              description={t('vaktcomply.doraThirdParties.emptyDescription')}
              action={
                <Button size="sm" onClick={() => { setEditing(null); setDialogOpen(true) }}>
                  <Plus className="w-4 h-4 mr-1" />
                  {t('vaktcomply.doraThirdParties.addProvider')}
                </Button>
              }
            />
          ) : (
            <div className="border border-border rounded-lg overflow-hidden">
              {list.map((tp) => (
                <ThirdPartyRow
                  key={tp.id}
                  tp={tp}
                  onEdit={() => { setEditing(tp); setDialogOpen(true) }}
                  onDelete={() => { setDeleteTarget(tp); }}
                />
              ))}
            </div>
          )}
        </div>
      </div>

      <ThirdPartyDialog
        open={dialogOpen}
        onClose={() => { setDialogOpen(false); setEditing(null) }}
        initial={editing}
        onSave={handleSave}
        saving={createMut.isPending || updateMut.isPending}
      />

      <AlertDialog open={!!deleteTarget} onOpenChange={(v) => { if (!v) { setDeleteTarget(null); } }}>
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>{t('vaktcomply.doraThirdParties.deleteDialogTitle')}</AlertDialogTitle>
            <AlertDialogDescription>
              {t('vaktcomply.doraThirdParties.deleteDialogDescription', { name: deleteTarget?.name ?? '' })}
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel>{t('vaktcomply.doraThirdParties.deleteBtnCancel')}</AlertDialogCancel>
            <AlertDialogAction
              className="bg-destructive hover:bg-destructive/90"
              onClick={() => {
                if (deleteTarget) {
                  deleteMut.mutate(deleteTarget.id, { onSuccess: () => { setDeleteTarget(null); } })
                }
              }}
            >
              {t('vaktcomply.doraThirdParties.deleteBtnConfirm')}
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </ProGate>
  )
}
