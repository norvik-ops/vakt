import { useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { ClipboardCheck, Plus } from 'lucide-react'
import { Spinner } from '../../../components/Spinner'
import { useTranslation } from 'react-i18next'
import { Button } from '../../../components/ui/button'
import { Card, CardContent } from '../../../components/ui/card'
import { Badge } from '../../../components/ui/badge'
import { Dialog, DialogContent, DialogHeader, DialogTitle, DialogFooter } from '../../../components/ui/dialog'
import { Input } from '../../../components/ui/input'
import { Label } from '../../../components/ui/label'
import { Textarea } from '../../../components/ui/textarea'
import { PageHeader } from '../../../shared/components/PageHeader'
import { EmptyState } from '../../../shared/components/EmptyState'
import { ComplianceTooltip } from '../../../shared/components/ComplianceTooltip'
import { useAuditRecords, useCreateAuditRecord } from '../hooks/useAudits'
import type { AuditRecord, CreateAuditRecordInput } from '../types'
import { useFormatDate } from '../../../shared/hooks/useFormatDate'

const STATUS_CLASS: Record<AuditRecord['status'], string> = {
  planned: 'bg-secondary text-secondary-foreground',
  in_progress: 'bg-amber-500/20 text-amber-400 border-amber-500/30',
  completed: 'bg-green-500/20 text-green-400 border-green-500/30',
}

function AuditCard({ record, onClick }: { record: AuditRecord; onClick: () => void }) {
  const { t } = useTranslation()
  const { formatDate } = useFormatDate()
  const STATUS_LABELS: Record<AuditRecord['status'], string> = {
    planned: t('vaktcomply.auditsPage.statusPlanned'),
    in_progress: t('vaktcomply.auditsPage.statusInProgress'),
    completed: t('vaktcomply.auditsPage.statusCompleted'),
  }
  const date = formatDate(record.audit_date, { year: 'numeric', month: 'short', day: 'numeric' })
  return (
    <Card className="cursor-pointer hover:border-brand/50 transition-colors" onClick={onClick}>
      <CardContent className="pt-5 space-y-2">
        <div className="flex items-start justify-between gap-2">
          <p className="font-medium text-sm">{record.title}</p>
          <Badge className={STATUS_CLASS[record.status]}>{STATUS_LABELS[record.status]}</Badge>
        </div>
        {record.scope && <p className="text-xs text-muted-foreground">{t('vaktcomply.auditsPage.scope')}: {record.scope}</p>}
        {record.auditor && <p className="text-xs text-muted-foreground">{t('vaktcomply.auditsPage.auditor')}: {record.auditor}</p>}
        <p className="text-xs text-muted-foreground">{t('vaktcomply.auditsPage.date')}: {date}</p>
        {record.findings && (
          <p className="text-xs text-muted-foreground line-clamp-2 border-t border-border pt-2 mt-2">{record.findings}</p>
        )}
      </CardContent>
    </Card>
  )
}

function emptyForm(): CreateAuditRecordInput {
  return {
    title: '',
    scope: '',
    auditor: '',
    audit_date: new Date().toISOString().slice(0, 10),
    findings: '',
    recommendations: '',
  }
}

export default function AuditsPage() {
  const { t } = useTranslation()
  const navigate = useNavigate()
  const [dialogOpen, setDialogOpen] = useState(false)
  const [form, setForm] = useState<CreateAuditRecordInput>(emptyForm())

  const { data: records, isLoading, isError } = useAuditRecords()
  const createRecord = useCreateAuditRecord()

  function openDialog() {
    setForm(emptyForm())
    setDialogOpen(true)
  }

  function handleSubmit() {
    createRecord.mutate(form, { onSuccess: () => { setDialogOpen(false); } })
  }

  return (
    <div className="flex flex-col h-full">
      <PageHeader
        title={t('vaktcomply.auditsPage.title')}
        description={t('vaktcomply.auditsPage.description')}
        actions={
          <Button onClick={openDialog}>
            <Plus className="w-4 h-4 mr-1" />
            {t('vaktcomply.auditsPage.createAudit')}
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
          <div className="text-sm text-red-400 p-4 bg-red-500/10 rounded-lg">{t('vaktcomply.auditsPage.loadError')}</div>
        )}
        {!isLoading && !isError && records?.length === 0 && (
          <EmptyState
            icon={ClipboardCheck}
            title={t('vaktcomply.auditsPage.noAudits')}
            description={t('vaktcomply.auditsPage.noAuditsDesc')}
            action={<Button onClick={openDialog}><Plus className="w-4 h-4 mr-1" />{t('vaktcomply.auditsPage.createAudit')}</Button>}
          />
        )}
        {!isLoading && !isError && records && records.length > 0 && (
          <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 gap-4">
            {records.map((r) => <AuditCard key={r.id} record={r} onClick={() => { navigate(`/vaktcomply/audits/${r.id}`); }} />)}
          </div>
        )}
      </div>

      <Dialog open={dialogOpen} onOpenChange={setDialogOpen}>
        <DialogContent className="max-w-lg max-h-[90vh] overflow-y-auto">
          <DialogHeader><DialogTitle><ComplianceTooltip term="audit">{t('vaktcomply.auditsPage.dialogTitle')}</ComplianceTooltip></DialogTitle></DialogHeader>
          <div className="space-y-4 py-2">
            <div className="space-y-1.5">
              <Label htmlFor="audit-title">{t('vaktcomply.auditsPage.labelTitle')} *</Label>
              <Input id="audit-title" placeholder={t('vaktcomply.auditsPage.placeholderTitle')} value={form.title}
                onChange={(e) => { setForm((f) => ({ ...f, title: e.target.value })); }} />
            </div>
            <div className="space-y-1.5">
              <Label htmlFor="audit-scope">{t('vaktcomply.auditsPage.labelScope')}</Label>
              <Input id="audit-scope" placeholder={t('vaktcomply.auditsPage.placeholderScope')} value={form.scope ?? ''}
                onChange={(e) => { setForm((f) => ({ ...f, scope: e.target.value })); }} />
            </div>
            <div className="grid grid-cols-2 gap-3">
              <div className="space-y-1.5">
                <Label htmlFor="audit-auditor">{t('vaktcomply.auditsPage.labelAuditor')}</Label>
                <Input id="audit-auditor" placeholder={t('vaktcomply.auditsPage.placeholderAuditor')} value={form.auditor ?? ''}
                  onChange={(e) => { setForm((f) => ({ ...f, auditor: e.target.value })); }} />
              </div>
              <div className="space-y-1.5">
                <Label htmlFor="audit-date">{t('vaktcomply.auditsPage.labelDate')} *</Label>
                <Input id="audit-date" type="date" value={form.audit_date}
                  onChange={(e) => { setForm((f) => ({ ...f, audit_date: e.target.value })); }} />
              </div>
            </div>
            <div className="space-y-1.5">
              <Label htmlFor="audit-findings">{t('vaktcomply.auditsPage.labelFindings')}</Label>
              <Textarea id="audit-findings" rows={3} placeholder={t('vaktcomply.auditsPage.placeholderFindings')} value={form.findings ?? ''}
                onChange={(e) => { setForm((f) => ({ ...f, findings: e.target.value })); }} />
            </div>
            <div className="space-y-1.5">
              <Label htmlFor="audit-recommendations">{t('vaktcomply.auditsPage.labelRecommendations')}</Label>
              <Textarea id="audit-recommendations" rows={2} placeholder={t('vaktcomply.auditsPage.placeholderRecommendations')} value={form.recommendations ?? ''}
                onChange={(e) => { setForm((f) => ({ ...f, recommendations: e.target.value })); }} />
            </div>
          </div>
          <DialogFooter>
            <Button variant="outline" onClick={() => { setDialogOpen(false); }}>{t('common.cancel')}</Button>
            <Button onClick={handleSubmit} disabled={!form.title || !form.audit_date || createRecord.isPending}>
              {createRecord.isPending ? t('vaktcomply.auditsPage.saving') : t('vaktcomply.auditsPage.createAudit')}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </div>
  )
}
