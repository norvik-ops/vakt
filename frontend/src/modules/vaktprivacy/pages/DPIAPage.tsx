import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { FileSearch, Plus, Pencil, Trash2, ShieldCheck, Download, ClipboardCheck } from 'lucide-react'
import { Spinner } from '../../../components/Spinner'
import { Button } from '../../../components/ui/button'
import { Card, CardContent } from '../../../components/ui/card'
import { Badge } from '../../../components/ui/badge'
import { Dialog, DialogContent, DialogHeader, DialogTitle, DialogFooter } from '../../../components/ui/dialog'
import { AlertDialog, AlertDialogContent, AlertDialogHeader, AlertDialogTitle, AlertDialogDescription, AlertDialogFooter, AlertDialogCancel, AlertDialogAction } from '../../../components/ui/alert-dialog'
import { Input } from '../../../components/ui/input'
import { Label } from '../../../components/ui/label'
import { Textarea } from '../../../components/ui/textarea'
import { PageHeader } from '../../../shared/components/PageHeader'
import { ProGate } from '../../../shared/components/ProGate'
import { EmptyState } from '../../../shared/components/EmptyState'
import { InfoBanner } from '../../../shared/components/InfoBanner'
import { TermTooltip } from '../../../shared/components/TermTooltip'
import { FieldError } from '../../../shared/components/FieldError'
import { useFormValidation } from '../../../shared/hooks/useFormValidation'
import { toast } from '../../../shared/hooks/useToast'
import { useDPIAs, useCreateDPIA, useUpdateDPIA, useApproveDPIA, useDeleteDPIA, useExportDPIA } from '../hooks/useDPIAs'
import { useVVT } from '../hooks/useVVT'
import type { DPIA, CreateDPIAInput, UpdateDPIAInput } from '../types'
import { useFormatDate } from '../../../shared/hooks/useFormatDate'

const STATUS_CLASS: Record<DPIA['status'], string> = {
  draft: 'bg-secondary text-secondary-foreground',
  in_review: 'bg-amber-500/20 text-amber-400 border-amber-500/30',
  approved: 'bg-green-500/20 text-green-400 border-green-500/30',
}

interface DPIAFormState {
  title: string
  description: string
  necessity_assessment: string
  risk_assessment: string
  mitigation_measures: string
  residual_risk: string
  dpo_consultation: boolean
  vvt_entry_id?: string
}

function emptyForm(): DPIAFormState {
  return {
    title: '',
    description: '',
    necessity_assessment: '',
    risk_assessment: '',
    mitigation_measures: '',
    residual_risk: '',
    dpo_consultation: false,
    vvt_entry_id: undefined,
  }
}

function formFromEntry(d: DPIA): DPIAFormState {
  return {
    title: d.title,
    description: d.description ?? '',
    necessity_assessment: d.necessity_assessment ?? '',
    risk_assessment: d.risk_assessment ?? '',
    mitigation_measures: d.mitigation_measures ?? '',
    residual_risk: d.residual_risk ?? '',
    dpo_consultation: d.dpo_consultation,
    vvt_entry_id: d.vvt_entry_id,
  }
}

function DPIACard({
  dpia,
  onEdit,
  onDelete,
  onApprove,
}: {
  dpia: DPIA
  onEdit: (d: DPIA) => void
  onDelete: (id: string) => void
  onApprove: (id: string) => void
}) {
  const { t } = useTranslation()
  const { formatDate } = useFormatDate()
  const date = formatDate(dpia.created_at, {
    year: 'numeric', month: 'short', day: 'numeric',
  })
  const getDPIAStatusLabel = (s: DPIA['status']) => t(`vaktprivacy.dpiaPage.status${s.charAt(0).toUpperCase() + s.slice(1).replace(/_([a-z])/g, (_, c: string) => c.toUpperCase())}`, { defaultValue: s })
  return (
    <Card>
      <CardContent className="pt-5 space-y-2">
        <div className="flex items-start justify-between gap-2">
          <p className="font-medium text-sm">{dpia.title}</p>
          <Badge className={STATUS_CLASS[dpia.status]}>{getDPIAStatusLabel(dpia.status)}</Badge>
        </div>
        {dpia.description && (
          <p className="text-xs text-muted-foreground line-clamp-2">{dpia.description}</p>
        )}
        <div className="flex items-center gap-3 text-xs text-muted-foreground">
          {dpia.dpo_consultation && (
            <span className="text-cyan-400">{t('vaktprivacy.dpiaPage.cardDPOConsulted')}</span>
          )}
          <span>{t('vaktprivacy.dpiaPage.cardCreated')} {date}</span>
        </div>
        <div className="flex items-center justify-between pt-1">
          {dpia.status !== 'approved' ? (
            <Button
              size="sm"
              variant="outline"
              className="text-xs border-green-500/40 text-green-400 hover:bg-green-500/10 h-7"
              onClick={() => { onApprove(dpia.id); }}
            >
              <ShieldCheck className="w-3.5 h-3.5 mr-1" />
              {t('vaktprivacy.dpiaPage.cardApprove')}
            </Button>
          ) : (
            <span />
          )}
          <div className="flex gap-1">
            <Button size="icon" variant="ghost" className="h-7 w-7" aria-label={t('vaktprivacy.dpiaPage.ariaEdit')} onClick={() => { onEdit(dpia); }}>
              <Pencil className="w-3.5 h-3.5" />
            </Button>
            <Button
              size="icon"
              variant="ghost"
              className="h-7 w-7 text-destructive hover:text-destructive"
              aria-label={t('vaktprivacy.dpiaPage.ariaDelete')}
              onClick={() => { onDelete(dpia.id); }}
            >
              <Trash2 className="w-3.5 h-3.5" />
            </Button>
          </div>
        </div>
      </CardContent>
    </Card>
  )
}

function DPIAForm({
  form,
  onChange,
  showVvtSelector,
  vvtEntries,
  errors,
  onClearError,
}: {
  form: DPIAFormState
  onChange: (f: DPIAFormState) => void
  showVvtSelector: boolean
  vvtEntries?: { id: string; name: string }[]
  errors?: Partial<Record<string, string>>
  onClearError?: (field: string) => void
}) {
  const set = (patch: Partial<DPIAFormState>) => { onChange({ ...form, ...patch }); }

  const { t } = useTranslation()
  return (
    <div className="space-y-4 py-2">
      <div className="space-y-1.5">
        <Label>{t('vaktprivacy.dpiaPage.formLabelTitle')} <span className="text-red-400 text-xs">*</span></Label>
        <Input
          placeholder="z.B. DSFA für KI-gestützte Videoüberwachung"
          value={form.title}
          onChange={(e) => { set({ title: e.target.value }); onClearError?.('title') }}
        />
        <FieldError error={errors?.title ?? null} />
      </div>
      {showVvtSelector && vvtEntries && vvtEntries.length > 0 && (
        <div className="space-y-1.5">
          <Label>{t('vaktprivacy.dpiaPage.formLabelVVT')}</Label>
          <select
            className="w-full rounded-md border border-input bg-background px-3 py-2 text-sm"
            value={form.vvt_entry_id ?? ''}
            onChange={(e) => { set({ vvt_entry_id: e.target.value || undefined }); }}
          >
            <option value="">{t('vaktprivacy.dpiaPage.formNoVVT')}</option>
            {vvtEntries.map((v) => (
              <option key={v.id} value={v.id}>{v.name}</option>
            ))}
          </select>
        </div>
      )}
      <div className="space-y-1.5">
        <Label>{t('vaktprivacy.dpiaPage.formLabelDesc')}</Label>
        <Textarea
          placeholder="Allgemeine Beschreibung der Verarbeitung …"
          rows={2}
          value={form.description}
          onChange={(e) => { set({ description: e.target.value }); }}
        />
      </div>
      <div className="space-y-1.5">
        <Label>{t('vaktprivacy.dpiaPage.formLabelNecessity')}</Label>
        <Textarea
          placeholder="Warum ist diese Verarbeitung erforderlich und verhältnismäßig?"
          rows={2}
          value={form.necessity_assessment}
          onChange={(e) => { set({ necessity_assessment: e.target.value }); }}
        />
      </div>
      <div className="space-y-1.5">
        <Label>{t('vaktprivacy.dpiaPage.formLabelRisk')}</Label>
        <Textarea
          placeholder="Identifizierte Risiken für die Rechte und Freiheiten der Betroffenen …"
          rows={3}
          value={form.risk_assessment}
          onChange={(e) => { set({ risk_assessment: e.target.value }); }}
        />
      </div>
      <div className="space-y-1.5">
        <Label>{t('vaktprivacy.dpiaPage.formLabelMitigation')}</Label>
        <Textarea
          placeholder="Technische und organisatorische Maßnahmen zur Risikominderung …"
          rows={2}
          value={form.mitigation_measures}
          onChange={(e) => { set({ mitigation_measures: e.target.value }); }}
        />
      </div>
      <div className="space-y-1.5">
        <Label>{t('vaktprivacy.dpiaPage.formLabelResidual')}</Label>
        <Textarea
          placeholder="Verbleibendes Restrisiko nach Maßnahmen …"
          rows={2}
          value={form.residual_risk}
          onChange={(e) => { set({ residual_risk: e.target.value }); }}
        />
      </div>
      <div className="flex items-center gap-2">
        <input
          type="checkbox"
          id="dpia-dpo"
          checked={form.dpo_consultation}
          onChange={(e) => { set({ dpo_consultation: e.target.checked }); }}
          className="w-4 h-4"
        />
        <Label htmlFor="dpia-dpo">{t('vaktprivacy.dpiaPage.formLabelDPO')}</Label>
      </div>
    </div>
  )
}

export default function DPIAPage() {
  const { t } = useTranslation()
  const [dialogMode, setDialogMode] = useState<'create' | 'edit' | null>(null)
  const [editId, setEditId] = useState<string | null>(null)
  const [form, setForm] = useState<DPIAFormState>(emptyForm())
  const [deleteId, setDeleteId] = useState<string | null>(null)
  const [exportError, setExportError] = useState<unknown>(null)

  const { data: dpias, isLoading, isError, error } = useDPIAs()
  const { data: vvtEntries } = useVVT()
  const createDPIA = useCreateDPIA()
  const updateDPIA = useUpdateDPIA()
  const approveDPIA = useApproveDPIA()
  const deleteDPIA = useDeleteDPIA()
  const exportDPIA = useExportDPIA()
  const { errors: dpiaErrors, validate: validateDPIA, clearError: clearDPIAError, clearAll: clearDPIAErrors } = useFormValidation<Record<string, unknown>>({
    title: { required: true },
  })

  async function handleExport() {
    try {
      setExportError(null)
      await exportDPIA()
    } catch (err) {
      setExportError(err)
    }
  }

  function openCreate() {
    setForm(emptyForm())
    setEditId(null)
    clearDPIAErrors()
    setDialogMode('create')
  }

  function openEdit(dpia: DPIA) {
    setForm(formFromEntry(dpia))
    setEditId(dpia.id)
    clearDPIAErrors()
    setDialogMode('edit')
  }

  function handleDelete(id: string) {
    setDeleteId(id)
  }

  function confirmDelete() {
    if (deleteId) deleteDPIA.mutate(deleteId)
    setDeleteId(null)
  }

  function handleApprove(id: string) {
    approveDPIA.mutate(id)
  }

  function handleSubmit() {
    if (!validateDPIA({ title: form.title })) return
    if (dialogMode === 'create') {
      const payload: CreateDPIAInput = {
        title: form.title,
        description: form.description || undefined,
        necessity_assessment: form.necessity_assessment || undefined,
        risk_assessment: form.risk_assessment || undefined,
        mitigation_measures: form.mitigation_measures || undefined,
        residual_risk: form.residual_risk || undefined,
        dpo_consultation: form.dpo_consultation,
        vvt_entry_id: form.vvt_entry_id,
      }
      createDPIA.mutate(payload, {
        onSuccess: () => {
          setDialogMode(null)
          toast(t('vaktprivacy.dpiaPage.toastCreated', { title: form.title }), 'success')
        },
      })
    } else if (dialogMode === 'edit' && editId) {
      const payload: UpdateDPIAInput = {
        title: form.title,
        description: form.description || undefined,
        necessity_assessment: form.necessity_assessment || undefined,
        risk_assessment: form.risk_assessment || undefined,
        mitigation_measures: form.mitigation_measures || undefined,
        residual_risk: form.residual_risk || undefined,
        dpo_consultation: form.dpo_consultation,
      }
      updateDPIA.mutate({ id: editId, input: payload }, { onSuccess: () => { setDialogMode(null); } })
    }
  }

  const isPending = createDPIA.isPending || updateDPIA.isPending

  return (
    <ProGate error={isError ? error : null}>
    <div className="flex flex-col h-full">
      <PageHeader
        title={t('vaktprivacy.dpiaPage.title')}
        description={t('vaktprivacy.dpiaPage.description')}
        actions={
          <div className="flex gap-2">
            <Button variant="outline" onClick={() => { void handleExport() }} disabled={!dpias || dpias.length === 0}>
              <Download className="w-4 h-4 mr-1" />
              {t('vaktprivacy.dpiaPage.exportPDF')}
            </Button>
            <Button onClick={openCreate}>
              <Plus className="w-4 h-4 mr-1" />
              {t('vaktprivacy.dpiaPage.createDSFA')}
            </Button>
          </div>
        }
      />
      <ProGate error={exportError}>{null}</ProGate>

      <InfoBanner icon={ClipboardCheck} title={t('vaktprivacy.dpiaPage.infoBannerTitle')}>
        <p>
          <TermTooltip term="DSFA" explanation={t('vaktprivacy.dpiaPage.bannerTooltipDSFA')}>DSFA</TermTooltip>
          {t('vaktprivacy.dpiaPage.bannerDesc1')}
        </p>
        <p className="mt-1">{t('vaktprivacy.dpiaPage.bannerDesc2')}</p>
      </InfoBanner>

      <div className="flex-1 p-6">
        {isLoading && (
          <div className="flex items-center justify-center h-48">
            <Spinner size="lg" color="primary" />
          </div>
        )}

        {isError && (
          <div className="text-sm text-red-400 p-4 bg-red-500/10 rounded-lg">
            {t('vaktprivacy.dpiaPage.errorLoading')}
          </div>
        )}

        {!isLoading && !isError && dpias?.length === 0 && (
          <EmptyState
            icon={FileSearch}
            title={t('vaktprivacy.dpiaPage.emptyTitle')}
            description={t('vaktprivacy.dpiaPage.emptyDesc')}
            action={
              <Button onClick={openCreate}>
                <Plus className="w-4 h-4 mr-1" />
                {t('vaktprivacy.dpiaPage.createDSFA')}
              </Button>
            }
          />
        )}

        {!isLoading && !isError && dpias && dpias.length > 0 && (
          <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 gap-4">
            {dpias.map((d) => (
              <DPIACard
                key={d.id}
                dpia={d}
                onEdit={openEdit}
                onDelete={handleDelete}
                onApprove={handleApprove}
              />
            ))}
          </div>
        )}
      </div>

      <AlertDialog open={deleteId !== null} onOpenChange={(open) => { if (!open) setDeleteId(null) }}>
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>{t('vaktprivacy.dpiaPage.deleteTitle')}</AlertDialogTitle>
            <AlertDialogDescription>
              {t('vaktprivacy.dpiaPage.deleteDesc')}
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel onClick={() => { setDeleteId(null); }}>{t('common.cancel')}</AlertDialogCancel>
            <AlertDialogAction onClick={confirmDelete} className="bg-destructive text-destructive-foreground hover:bg-destructive/90">{t('common.delete')}</AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>

      <Dialog open={dialogMode !== null} onOpenChange={(open) => { if (!open) setDialogMode(null) }}>
        <DialogContent className="max-w-lg max-h-[90vh] overflow-y-auto">
          <DialogHeader>
            <DialogTitle>
              {dialogMode === 'create' ? t('vaktprivacy.dpiaPage.createDSFA') : t('vaktprivacy.dpiaPage.editDSFA')}
            </DialogTitle>
          </DialogHeader>
          <DPIAForm
            form={form}
            onChange={setForm}
            showVvtSelector={dialogMode === 'create'}
            vvtEntries={vvtEntries}
            errors={dpiaErrors}
            onClearError={clearDPIAError}
          />
          <DialogFooter>
            <Button variant="outline" onClick={() => { setDialogMode(null); }}>
              {t('common.cancel')}
            </Button>
            <Button onClick={handleSubmit} disabled={isPending}>
              {isPending ? t('vaktprivacy.dpiaPage.savingPending') : dialogMode === 'create' ? t('vaktprivacy.dpiaPage.createDSFA') : t('vaktprivacy.dpiaPage.saveButton')}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </div>
    </ProGate>
  )
}
