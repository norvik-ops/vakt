import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { ActivitySquare, Plus, Pencil, Trash2 } from 'lucide-react'
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
  useBIAProcesses,
  useCreateBIAProcess,
  useUpdateBIAProcess,
  useDeleteBIAProcess,
} from '../hooks/useBIA'
import { TermTooltip } from '../../../shared/components/TermTooltip'
import type { BIAProcess, BIACriticality, CreateBIAProcessInput } from '../types'

const CRIT_CLASS: Record<BIACriticality, string> = {
  critical: 'bg-red-500/20 text-red-400 border-red-500/30',
  high: 'bg-orange-500/20 text-orange-400 border-orange-500/30',
  medium: 'bg-amber-500/20 text-amber-400 border-amber-500/30',
  low: 'bg-secondary text-secondary-foreground',
}

function emptyForm(): CreateBIAProcessInput {
  return {
    name: '',
    description: '',
    process_owner: '',
    criticality: 'medium',
    schutzbedarfsklasse: 2,
    rto_hours: 24,
    rpo_hours: 4,
    mbco_percent: 50,
    dependencies: [],
  }
}

function processToForm(p: BIAProcess): CreateBIAProcessInput {
  return {
    name: p.name,
    description: p.description,
    process_owner: p.process_owner,
    criticality: p.criticality,
    schutzbedarfsklasse: p.schutzbedarfsklasse,
    rto_hours: p.rto_hours,
    rpo_hours: p.rpo_hours,
    mbco_percent: p.mbco_percent,
    dependencies: p.dependencies,
  }
}

function BIACard({
  process,
  onEdit,
  onDelete,
}: {
  process: BIAProcess
  onEdit: () => void
  onDelete: () => void
}) {
  const { t } = useTranslation()
  return (
    <Card>
      <CardHeader className="pb-2">
        <div className="flex items-start justify-between gap-2">
          <div className="space-y-1 flex-1 min-w-0">
            <CardTitle className="text-sm leading-tight">{process.name}</CardTitle>
            <div className="flex items-center gap-1.5 flex-wrap">
              <Badge className={CRIT_CLASS[process.criticality]} variant="outline">
                {t(`bcm.bia.criticality.${process.criticality}`)}
              </Badge>
              <Badge variant="outline" className="text-xs">
                {t('bcm.bia.klasse')} {process.schutzbedarfsklasse}
              </Badge>
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
      <CardContent className="pt-0 space-y-1.5">
        {process.description && (
          <p className="text-xs text-muted-foreground line-clamp-2">{process.description}</p>
        )}
        <div className="grid grid-cols-3 gap-2 text-xs">
          <div className="text-center p-1.5 rounded bg-muted/40">
            <p className="font-semibold">{process.rto_hours}h</p>
            <p className="text-muted-foreground"><TermTooltip term="RTO" glossaryKey="RTO">RTO</TermTooltip></p>
          </div>
          <div className="text-center p-1.5 rounded bg-muted/40">
            <p className="font-semibold">{process.rpo_hours}h</p>
            <p className="text-muted-foreground">RPO</p>
          </div>
          <div className="text-center p-1.5 rounded bg-muted/40">
            <p className="font-semibold">{process.mbco_percent}%</p>
            <p className="text-muted-foreground">MBCO</p>
          </div>
        </div>
        {process.process_owner && (
          <p className="text-xs text-muted-foreground">
            {t('bcm.bia.owner')}: {process.process_owner}
          </p>
        )}
      </CardContent>
    </Card>
  )
}

export default function BIAPage() {
  const { t } = useTranslation()
  const [dialogOpen, setDialogOpen] = useState(false)
  const [editId, setEditId] = useState<string | null>(null)
  const [form, setForm] = useState<CreateBIAProcessInput>(emptyForm())

  const { data: processes = [], isLoading, isError } = useBIAProcesses()
  const create = useCreateBIAProcess()
  const update = useUpdateBIAProcess(editId ?? '')
  const del = useDeleteBIAProcess()

  function openCreate() {
    setEditId(null)
    setForm(emptyForm())
    setDialogOpen(true)
  }

  function openEdit(p: BIAProcess) {
    setEditId(p.id)
    setForm(processToForm(p))
    setDialogOpen(true)
  }

  function handleDelete(id: string) {
    if (confirm(t('bcm.bia.deleteConfirm'))) {
      del.mutate(id)
    }
  }

  function handleSubmit() {
    const action = editId ? update : create
    action.mutate(form, {
      onSuccess: () => { setDialogOpen(false) },
    })
  }

  const isPending = create.isPending || update.isPending

  return (
    <div className="flex flex-col h-full">
      <PageHeader
        title={t('bcm.bia.title')}
        description={t('bcm.bia.description')}
        actions={
          <Button onClick={openCreate}>
            <Plus className="w-4 h-4 mr-1" />
            {t('bcm.bia.new')}
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
            {t('bcm.bia.loadError')}
          </div>
        )}
        {!isLoading && !isError && processes.length === 0 && (
          <EmptyState
            icon={ActivitySquare}
            title={t('bcm.bia.emptyTitle')}
            description={t('bcm.bia.emptyDescription')}
            action={
              <Button onClick={openCreate}>
                <Plus className="w-4 h-4 mr-1" />
                {t('bcm.bia.new')}
              </Button>
            }
          />
        )}
        {!isLoading && !isError && processes.length > 0 && (
          <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 gap-4">
            {processes.map((p) => (
              <BIACard
                key={p.id}
                process={p}
                onEdit={() => { openEdit(p) }}
                onDelete={() => { handleDelete(p.id) }}
              />
            ))}
          </div>
        )}
      </div>

      <Dialog open={dialogOpen} onOpenChange={setDialogOpen}>
        <DialogContent className="max-w-lg">
          <DialogHeader>
            <DialogTitle>
              {editId ? t('bcm.bia.edit') : t('bcm.bia.new')}
            </DialogTitle>
          </DialogHeader>
          <div className="space-y-3 py-2">
            <div className="space-y-1.5">
              <Label>{t('bcm.bia.name')} *</Label>
              <Input
                value={form.name}
                placeholder={t('bcm.bia.namePlaceholder')}
                onChange={(e) => { setForm((f) => ({ ...f, name: e.target.value })) }}
              />
            </div>
            <div className="space-y-1.5">
              <Label>{t('bcm.bia.descriptionLabel')}</Label>
              <Textarea
                rows={2}
                value={form.description ?? ''}
                onChange={(e) => { setForm((f) => ({ ...f, description: e.target.value })) }}
              />
            </div>
            <div className="grid grid-cols-2 gap-3">
              <div className="space-y-1.5">
                <Label>{t('bcm.bia.criticalityLabel')} *</Label>
                <Select
                  value={form.criticality}
                  onValueChange={(v) => { setForm((f) => ({ ...f, criticality: v as BIACriticality })) }}
                >
                  <SelectTrigger><SelectValue /></SelectTrigger>
                  <SelectContent>
                    <SelectItem value="critical">{t('bcm.bia.criticality.critical')}</SelectItem>
                    <SelectItem value="high">{t('bcm.bia.criticality.high')}</SelectItem>
                    <SelectItem value="medium">{t('bcm.bia.criticality.medium')}</SelectItem>
                    <SelectItem value="low">{t('bcm.bia.criticality.low')}</SelectItem>
                  </SelectContent>
                </Select>
              </div>
              <div className="space-y-1.5">
                <Label>{t('bcm.bia.schutzbedarfsklasse')}</Label>
                <Select
                  value={String(form.schutzbedarfsklasse ?? 2)}
                  onValueChange={(v) => {
                    setForm((f) => ({ ...f, schutzbedarfsklasse: Number(v) as 1 | 2 | 3 }))
                  }}
                >
                  <SelectTrigger><SelectValue /></SelectTrigger>
                  <SelectContent>
                    <SelectItem value="1">1 — {t('bcm.bia.klasse1')}</SelectItem>
                    <SelectItem value="2">2 — {t('bcm.bia.klasse2')}</SelectItem>
                    <SelectItem value="3">3 — {t('bcm.bia.klasse3')}</SelectItem>
                  </SelectContent>
                </Select>
              </div>
            </div>
            <div className="grid grid-cols-3 gap-3">
              <div className="space-y-1.5">
                <Label>RTO (h) *</Label>
                <Input
                  type="number"
                  min={0}
                  value={form.rto_hours}
                  onChange={(e) => { setForm((f) => ({ ...f, rto_hours: Number(e.target.value) })) }}
                />
              </div>
              <div className="space-y-1.5">
                <Label>RPO (h) *</Label>
                <Input
                  type="number"
                  min={0}
                  value={form.rpo_hours}
                  onChange={(e) => { setForm((f) => ({ ...f, rpo_hours: Number(e.target.value) })) }}
                />
              </div>
              <div className="space-y-1.5">
                <Label>MBCO (%)</Label>
                <Input
                  type="number"
                  min={0}
                  max={100}
                  value={form.mbco_percent ?? 50}
                  onChange={(e) => { setForm((f) => ({ ...f, mbco_percent: Number(e.target.value) })) }}
                />
              </div>
            </div>
            <div className="space-y-1.5">
              <Label>{t('bcm.bia.owner')}</Label>
              <Input
                value={form.process_owner ?? ''}
                placeholder={t('bcm.bia.ownerPlaceholder')}
                onChange={(e) => { setForm((f) => ({ ...f, process_owner: e.target.value })) }}
              />
            </div>
          </div>
          <DialogFooter>
            <Button variant="outline" onClick={() => { setDialogOpen(false) }}>
              {t('common.cancel')}
            </Button>
            <Button onClick={handleSubmit} disabled={!form.name || isPending}>
              {isPending ? t('common.saving') : editId ? t('common.save') : t('common.add')}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </div>
  )
}
