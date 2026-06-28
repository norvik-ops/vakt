import { useState, useEffect } from 'react'
import { useParams, useNavigate } from 'react-router-dom'
import { ArrowLeft, Save, ClipboardCheck, Plus } from 'lucide-react'
import { Spinner } from '../../../components/Spinner'
import { PageHeader } from '../../../shared/components/PageHeader'
import { Button } from '../../../components/ui/button'
import { Card, CardContent, CardHeader, CardTitle } from '../../../components/ui/card'
import { Input } from '../../../components/ui/input'
import { Label } from '../../../components/ui/label'
import { Textarea } from '../../../components/ui/textarea'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '../../../components/ui/select'
import { Dialog, DialogContent, DialogHeader, DialogTitle, DialogFooter } from '../../../components/ui/dialog'
import { useAuditRecord, useUpdateAuditRecord } from '../hooks/useAudits'
import { useCreateCAPA } from '../hooks/useCAPAs'
import type { AuditRecord, UpdateAuditRecordInput } from '../types'
import { useFormatDate } from '../../../shared/hooks/useFormatDate'
import { useTranslation } from 'react-i18next'

export default function AuditDetailPage() {
  const { t } = useTranslation()
  const { id } = useParams<{ id: string }>()
  const navigate = useNavigate()
  const { formatDate } = useFormatDate()
  const { data: record, isLoading, isError } = useAuditRecord(id ?? '')
  const update = useUpdateAuditRecord(id ?? '')
  const createCAPA = useCreateCAPA()

  const STATUS_LABELS: Record<AuditRecord['status'], string> = {
    planned: t('vaktcomply.auditDetail.statusPlanned'),
    in_progress: t('vaktcomply.auditDetail.statusInProgress'),
    completed: t('vaktcomply.auditDetail.statusCompleted'),
  }

  const [form, setForm] = useState<UpdateAuditRecordInput | null>(null)
  const [dirty, setDirty] = useState(false)
  const [capaDialogOpen, setCAPADialogOpen] = useState(false)
  const [capaTitle, setCAPATitle] = useState('')

  useEffect(() => {
    if (record && !form) {
      setForm({
        title: record.title,
        scope: record.scope ?? '',
        auditor: record.auditor ?? '',
        audit_date: record.audit_date.slice(0, 10),
        status: record.status,
        findings: record.findings ?? '',
        recommendations: record.recommendations ?? '',
      })
    }
  }, [record, form])

  function set<K extends keyof UpdateAuditRecordInput>(key: K, value: UpdateAuditRecordInput[K]) {
    setForm((f) => f ? { ...f, [key]: value } : f)
    setDirty(true)
  }

  function handleSave() {
    if (!form) return
    update.mutate(form, { onSuccess: () => { setDirty(false); } })
  }

  function handleCreateCAPA() {
    if (!capaTitle.trim() || !id) return
    createCAPA.mutate(
      { source_type: 'audit', source_id: id, title: capaTitle, priority: 'medium' },
      { onSuccess: () => { setCAPATitle(''); setCAPADialogOpen(false) } },
    )
  }

  if (isLoading) return (
    <div className="flex items-center justify-center h-48">
      <Spinner size="lg" color="primary" />
    </div>
  )
  if (isError || !record) return (
    <div className="p-6 text-sm text-red-400">{t('vaktcomply.auditDetail.notFound')}</div>
  )

  return (
    <div className="flex flex-col h-full">
      <PageHeader
        title={record.title}
        description={record.scope || t('vaktcomply.auditDetail.defaultDescription')}
        actions={
          <div className="flex gap-2">
            <Button variant="outline" onClick={() => { navigate('/vaktcomply/audits'); }}>
              <ArrowLeft className="w-4 h-4 mr-1" />
              {t('vaktcomply.auditDetail.back')}
            </Button>
            <Button variant="outline" onClick={() => { setCAPADialogOpen(true); }}>
              <ClipboardCheck className="w-4 h-4 mr-1" />
              {t('vaktcomply.auditDetail.createCAPA')}
            </Button>
            <Button onClick={handleSave} disabled={!dirty || update.isPending}>
              <Save className="w-4 h-4 mr-1" />
              {update.isPending ? t('vaktcomply.auditDetail.saving') : t('vaktcomply.auditDetail.save')}
            </Button>
          </div>
        }
      />

      {form && (
        <div className="flex-1 p-6 grid grid-cols-1 lg:grid-cols-3 gap-6">
          <div className="lg:col-span-2 space-y-4">
            <Card>
              <CardHeader><CardTitle className="text-sm">{t('vaktcomply.auditDetail.cardAuditData')}</CardTitle></CardHeader>
              <CardContent className="space-y-4">
                <div className="space-y-1.5">
                  <Label>{t('vaktcomply.auditDetail.labelTitle')}</Label>
                  <Input value={form.title} onChange={(e) => { set('title', e.target.value); }} />
                </div>
                <div className="space-y-1.5">
                  <Label>{t('vaktcomply.auditDetail.labelScope')}</Label>
                  <Input value={form.scope ?? ''} placeholder={t('vaktcomply.auditDetail.placeholderScope')} onChange={(e) => { set('scope', e.target.value); }} />
                </div>
                <div className="grid grid-cols-2 gap-3">
                  <div className="space-y-1.5">
                    <Label>{t('vaktcomply.auditDetail.labelAuditor')}</Label>
                    <Input value={form.auditor ?? ''} onChange={(e) => { set('auditor', e.target.value); }} />
                  </div>
                  <div className="space-y-1.5">
                    <Label>{t('vaktcomply.auditDetail.labelAuditDate')}</Label>
                    <Input type="date" value={form.audit_date} onChange={(e) => { set('audit_date', e.target.value); }} />
                  </div>
                </div>
              </CardContent>
            </Card>

            <Card>
              <CardHeader><CardTitle className="text-sm">{t('vaktcomply.auditDetail.cardResults')}</CardTitle></CardHeader>
              <CardContent className="space-y-4">
                <div className="space-y-1.5">
                  <Label>{t('vaktcomply.auditDetail.labelFindings')}</Label>
                  <Textarea rows={4} value={form.findings ?? ''} placeholder={t('vaktcomply.auditDetail.placeholderFindings')} onChange={(e) => { set('findings', e.target.value); }} />
                </div>
                <div className="space-y-1.5">
                  <Label>{t('vaktcomply.auditDetail.labelRecommendations')}</Label>
                  <Textarea rows={3} value={form.recommendations ?? ''} placeholder={t('vaktcomply.auditDetail.placeholderRecommendations')} onChange={(e) => { set('recommendations', e.target.value); }} />
                </div>
              </CardContent>
            </Card>
          </div>

          <div className="space-y-4">
            <Card>
              <CardHeader><CardTitle className="text-sm">{t('vaktcomply.auditDetail.cardStatus')}</CardTitle></CardHeader>
              <CardContent>
                <Select value={form.status} onValueChange={(v) => { set('status', v as AuditRecord['status']); }}>
                  <SelectTrigger><SelectValue /></SelectTrigger>
                  <SelectContent>
                    {(Object.keys(STATUS_LABELS) as AuditRecord['status'][]).map((k) => (
                      <SelectItem key={k} value={k}>{STATUS_LABELS[k]}</SelectItem>
                    ))}
                  </SelectContent>
                </Select>
              </CardContent>
            </Card>

            <Card>
              <CardContent className="pt-4 space-y-1 text-xs text-muted-foreground">
                <p>{t('vaktcomply.auditDetail.created')}: {formatDate(record.created_at)}</p>
                <p>{t('vaktcomply.auditDetail.changed')}: {formatDate(record.updated_at)}</p>
              </CardContent>
            </Card>
          </div>
        </div>
      )}

      {/* CAPA quick-create dialog */}
      <Dialog open={capaDialogOpen} onOpenChange={(v) => { if (!v) { setCAPADialogOpen(false); } }}>
        <DialogContent className="sm:max-w-md">
          <DialogHeader>
            <DialogTitle>{t('vaktcomply.auditDetail.capaDialogTitle')}</DialogTitle>
          </DialogHeader>
          <div className="space-y-3 py-2">
            <div className="space-y-1.5">
              <Label>{t('vaktcomply.auditDetail.capaLabelTitle')}</Label>
              <Input value={capaTitle} onChange={(e) => { setCAPATitle(e.target.value); }} placeholder={t('vaktcomply.auditDetail.capaPlaceholder')} />
            </div>
          </div>
          <DialogFooter>
            <Button variant="outline" onClick={() => { setCAPADialogOpen(false); }}>{t('common.cancel')}</Button>
            <Button onClick={handleCreateCAPA} disabled={!capaTitle.trim() || createCAPA.isPending}>
              <Plus className="w-4 h-4 mr-1" />
              {createCAPA.isPending ? t('vaktcomply.auditDetail.capaCreating') : t('vaktcomply.auditDetail.capaCreate')}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </div>
  )
}
