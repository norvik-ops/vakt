import { useState } from 'react'
import { Handshake, Plus, Pencil, Trash2, FileDown, LayoutTemplate } from 'lucide-react'
import { Spinner } from '../../../components/Spinner'
import { useTranslation } from 'react-i18next'
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
import { useFormatDate } from '../../../shared/hooks/useFormatDate'

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
  const { t } = useTranslation()
  const { formatDate } = useFormatDate()
  const STATUS_LABELS: Record<AVV['status'], string> = {
    active: t('secprivacy.avvPage.statusActive'),
    expired: t('secprivacy.avvPage.statusExpired'),
    terminated: t('secprivacy.avvPage.statusTerminated'),
  }
  const contractDate = avv.contract_date
    ? formatDate(avv.contract_date, { year: 'numeric', month: 'short', day: 'numeric' })
    : null
  const reviewDate = avv.review_date
    ? formatDate(avv.review_date, { year: 'numeric', month: 'short', day: 'numeric' })
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
          {contractDate && <span>{t('secprivacy.avvPage.contractDate')}: {contractDate}</span>}
          {reviewDate && <span>{t('secprivacy.avvPage.reviewDate')}: {reviewDate}</span>}
        </div>
        {avv.template_id && (
          <p className="text-xs text-primary/70">{t('secprivacy.avvPage.template')}: {avv.template_id}</p>
        )}
        {avv.notes && <p className="text-xs text-muted-foreground italic line-clamp-1">{avv.notes}</p>}
        <div className="flex justify-end gap-1 pt-1">
          {avv.body && (
            <Button
              size="icon"
              variant="ghost"
              className="h-7 w-7"
              title={t('common.export')}
              onClick={() => { onDownloadPDF(avv.id); }}
            >
              <FileDown className="w-3.5 h-3.5" />
            </Button>
          )}
          <Button size="icon" variant="ghost" className="h-7 w-7" aria-label={t('common.edit')} onClick={() => { onEdit(avv); }}>
            <Pencil className="w-3.5 h-3.5" />
          </Button>
          <Button
            size="icon"
            variant="ghost"
            className="h-7 w-7 text-destructive hover:text-destructive"
            aria-label={t('common.delete')}
            onClick={() => { onDelete(avv.id); }}
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
  const { t } = useTranslation()
  const set = (patch: Partial<AVVFormState>) => { onChange({ ...form, ...patch }); }

  return (
    <div className="space-y-4 py-2">
      <div className="space-y-1.5">
        <Label>{t('secprivacy.avvPage.labelProcessor')} *</Label>
        <Input
          placeholder={t('secprivacy.avvPage.placeholderProcessor')}
          value={form.processor_name}
          onChange={(e) => { set({ processor_name: e.target.value }); }}
        />
      </div>
      <div className="space-y-1.5">
        <Label>{t('secprivacy.avvPage.labelService')} *</Label>
        <Textarea
          placeholder={t('secprivacy.avvPage.placeholderService')}
          rows={3}
          value={form.service_description}
          onChange={(e) => { set({ service_description: e.target.value }); }}
        />
      </div>
      <div className="grid grid-cols-2 gap-3">
        <div className="space-y-1.5">
          <Label>{t('secprivacy.avvPage.labelContractDate')}</Label>
          <Input
            type="date"
            value={form.contract_date}
            onChange={(e) => { set({ contract_date: e.target.value }); }}
          />
        </div>
        <div className="space-y-1.5">
          <Label>{t('secprivacy.avvPage.labelReviewDate')}</Label>
          <Input
            type="date"
            value={form.review_date}
            onChange={(e) => { set({ review_date: e.target.value }); }}
          />
        </div>
      </div>
      {showStatus && (
        <div className="space-y-1.5">
          <Label>{t('secprivacy.avvPage.labelStatus')}</Label>
          <Select value={form.status} onValueChange={(v) => { set({ status: v as AVV['status'] }); }}>
            <SelectTrigger>
              <SelectValue />
            </SelectTrigger>
            <SelectContent>
              <SelectItem value="active">{t('secprivacy.avvPage.statusActive')}</SelectItem>
              <SelectItem value="expired">{t('secprivacy.avvPage.statusExpired')}</SelectItem>
              <SelectItem value="terminated">{t('secprivacy.avvPage.statusTerminated')}</SelectItem>
            </SelectContent>
          </Select>
        </div>
      )}
      <div className="space-y-1.5">
        <Label>{t('secprivacy.avvPage.labelNotes')}</Label>
        <Textarea
          placeholder={t('secprivacy.avvPage.placeholderNotes')}
          rows={2}
          value={form.notes}
          onChange={(e) => { set({ notes: e.target.value }); }}
        />
      </div>
    </div>
  )
}

export default function AVVPage() {
  const { t } = useTranslation()
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
      createAVV.mutate(payload, { onSuccess: () => { setDialogMode(null); } })
    } else if (dialogMode === 'edit' && editId) {
      const payload: UpdateAVVInput = {
        processor_name: form.processor_name,
        service_description: form.service_description,
        contract_date: form.contract_date || undefined,
        review_date: form.review_date || undefined,
        status: form.status,
        notes: form.notes || undefined,
      }
      updateAVV.mutate({ id: editId, input: payload }, { onSuccess: () => { setDialogMode(null); } })
    }
  }

  const isPending = createAVV.isPending || updateAVV.isPending
  const canSubmit = form.processor_name && form.service_description && !isPending

  return (
    <div className="flex flex-col h-full">
      <PageHeader
        title={t('secprivacy.avvPage.title')}
        description={t('secprivacy.avvPage.description')}
        actions={
          <div className="flex gap-2">
            <Button variant="outline" onClick={() => { setTemplatePickerOpen(true); }}>
              <LayoutTemplate className="w-4 h-4 mr-1" />
              {t('secprivacy.avvPage.fromTemplate')}
            </Button>
            <Button onClick={openCreate}>
              <Plus className="w-4 h-4 mr-1" />
              {t('secprivacy.avvPage.createAVV')}
            </Button>
          </div>
        }
      />

      <InfoBanner icon={Handshake} title={t('secprivacy.avvPage.infoBannerTitle')}>
        <p>{t('secprivacy.avvPage.infoBannerDesc')}</p>
        <p className="mt-1">{t('secprivacy.avvPage.infoBannerTip')}</p>
      </InfoBanner>

      <ProGate error={pdfError}>{null}</ProGate>

      <div className="flex-1 p-6">
        {isLoading && (
          <div className="flex items-center justify-center h-48">
            <Spinner size="lg" color="primary" />
          </div>
        )}

        {isError && (
          <div className="text-sm text-red-400 p-4 bg-red-500/10 rounded-lg">
            {t('secprivacy.avvPage.loadError')}
          </div>
        )}

        {!isLoading && !isError && avvs?.length === 0 && (
          <EmptyState
            icon={Handshake}
            title={t('secprivacy.avvPage.noAVVs')}
            description={t('secprivacy.avvPage.noAVVsDesc')}
            action={
              <div className="flex gap-2 justify-center">
                <Button variant="outline" onClick={() => { setTemplatePickerOpen(true); }}>
                  <LayoutTemplate className="w-4 h-4 mr-1" />
                  {t('secprivacy.avvPage.fromTemplate')}
                </Button>
                <Button onClick={openCreate}>
                  <Plus className="w-4 h-4 mr-1" />
                  {t('secprivacy.avvPage.createAVV')}
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
                onDownloadPDF={(id) => { void handleDownloadPDF(id) }}
              />
            ))}
          </div>
        )}
      </div>

      <AlertDialog open={deleteId !== null} onOpenChange={(open) => { if (!open) setDeleteId(null) }}>
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>{t('secprivacy.avvPage.deleteDialogTitle')}</AlertDialogTitle>
            <AlertDialogDescription>
              {t('secprivacy.avvPage.deleteDialogDesc')}
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
              {dialogMode === 'create' ? t('secprivacy.avvPage.createDialogTitle') : t('secprivacy.avvPage.editDialogTitle')}
            </DialogTitle>
          </DialogHeader>
          <AVVForm form={form} onChange={setForm} showStatus={dialogMode === 'edit'} />
          <DialogFooter>
            <Button variant="outline" onClick={() => { setDialogMode(null); }}>
              {t('common.cancel')}
            </Button>
            <Button onClick={handleSubmit} disabled={!canSubmit}>
              {isPending ? t('secprivacy.avvPage.saving') : dialogMode === 'create' ? t('secprivacy.avvPage.createAVV') : t('common.save')}
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
