import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { ShieldAlert, Plus, Pencil, Trash2, Paperclip, Link2 } from 'lucide-react'
import { Spinner } from '../../../components/Spinner'
import { PageHeader } from '../../../shared/components/PageHeader'
import { EmptyState } from '../../../shared/components/EmptyState'
import { ProGate } from '../../../shared/components/ProGate'
import { Button } from '../../../components/ui/button'
import { Badge } from '../../../components/ui/badge'
import { Card, CardContent } from '../../../components/ui/card'
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
  useResilienceTests,
  useCreateResilienceTest,
  useUpdateResilienceTest,
  useDeleteResilienceTest,
  useUploadResilienceTestAttachment,
  useLinkResilienceTestAsEvidence,
} from '../hooks/useResilienceTests'
import type { ResilienceTest, CreateResilienceTestInput } from '../types'
import { useFormatDate } from '../../../shared/hooks/useFormatDate'

// ─── Type helpers ─────────────────────────────────────────────────────────────

const TYPE_LABEL_KEYS: Record<ResilienceTest['type'], string> = {
  tlpt: 'TLPT',
  pentest: 'vaktcomply.resilienceTests.kind.pentest',
  scenario_based: 'vaktcomply.resilienceTests.kind.scenario',
  vulnerability_assessment: 'vaktcomply.resilienceTests.typeLabels.vulnerabilityAssessment',
}

const TYPE_CLASS: Record<ResilienceTest['type'], string> = {
  tlpt: 'bg-red-500/20 text-red-400 border-red-500/30',
  pentest: 'bg-amber-500/20 text-amber-400 border-amber-500/30',
  scenario_based: 'bg-secondary text-secondary-foreground',
  vulnerability_assessment: 'bg-secondary text-secondary-foreground',
}

const REMEDIATION_LABEL_KEYS: Record<ResilienceTest['remediation_status'], string> = {
  open: 'vaktcomply.resilienceTests.status.open',
  in_progress: 'vaktcomply.resilienceTests.status.in_progress',
  completed: 'vaktcomply.resilienceTests.status.completed',
  accepted: 'vaktcomply.resilienceTests.status.accepted',
}

const REMEDIATION_CLASS: Record<ResilienceTest['remediation_status'], string> = {
  open: 'bg-red-500/20 text-red-400 border-red-500/30',
  in_progress: 'bg-amber-500/20 text-amber-400 border-amber-500/30',
  completed: 'bg-green-500/20 text-green-400 border-green-500/30',
  accepted: 'bg-secondary text-secondary-foreground',
}

// ─── Empty form ───────────────────────────────────────────────────────────────

function emptyForm(): CreateResilienceTestInput {
  return {
    type: '',
    scope: '',
    provider: '',
    test_date: '',
    summary: '',
    remediation_status: 'open',
  }
}

function testToForm(t: ResilienceTest): CreateResilienceTestInput {
  return {
    type: t.type,
    scope: t.scope ?? '',
    provider: t.provider ?? '',
    test_date: t.test_date ? t.test_date.slice(0, 10) : '',
    summary: t.summary ?? '',
    remediation_status: t.remediation_status,
  }
}

// ─── Row component ────────────────────────────────────────────────────────────

function ResilienceTestRow({
  test,
  onEdit,
  onDelete,
  onLinkEvidence,
}: {
  test: ResilienceTest
  onEdit: () => void
  onDelete: () => void
  onLinkEvidence: () => void
}) {
  const { t } = useTranslation()
  const { formatDate } = useFormatDate()
  const typeKey = TYPE_LABEL_KEYS[test.type]
  const typeLabel = typeKey.startsWith('vaktcomply') ? t(typeKey) : typeKey
  return (
    <Card>
      <CardContent className="pt-5 space-y-2">
        <div className="flex items-start justify-between gap-2">
          <div className="space-y-1 flex-1">
            <div className="flex items-center gap-2 flex-wrap">
              <Badge className={TYPE_CLASS[test.type]}>{typeLabel}</Badge>
              {test.overdue_warning && (
                <Badge className="bg-red-500/20 text-red-400 border-red-500/30 text-xs">
                  {t('vaktcomply.resilienceTests.overdue')}
                </Badge>
              )}
              <Badge className={REMEDIATION_CLASS[test.remediation_status]}>
                {t(REMEDIATION_LABEL_KEYS[test.remediation_status])}
              </Badge>
            </div>
            <p className="text-sm font-medium">
              {formatDate(test.test_date)}
              {test.provider ? ` · ${test.provider}` : ''}
            </p>
            {test.scope && (
              <p className="text-xs text-muted-foreground">{t('vaktcomply.resilienceTests.scope')}: {test.scope}</p>
            )}
            {test.summary && (
              <p className="text-xs text-muted-foreground line-clamp-2">{test.summary}</p>
            )}
            {test.attachment_url && (
              <p className="text-xs text-muted-foreground flex items-center gap-1">
                <Paperclip className="w-3 h-3" />
                {t('vaktcomply.resilienceTests.attachmentPresent')}
              </p>
            )}
          </div>
          <div className="flex items-center gap-1 shrink-0">
            <Button variant="ghost" size="icon" className="h-7 w-7" onClick={onEdit} title={t('vaktcomply.resilienceTests.editTitle')}>
              <Pencil className="w-3.5 h-3.5" />
            </Button>
            <Button variant="ghost" size="icon" className="h-7 w-7" onClick={onLinkEvidence} title={t('vaktcomply.resilienceTests.linkEvidenceTitle')}>
              <Link2 className="w-3.5 h-3.5" />
            </Button>
            <Button
              variant="ghost"
              size="icon"
              className="h-7 w-7 text-red-400 hover:text-red-300"
              onClick={onDelete}
              title={t('vaktcomply.resilienceTests.deleteTitle')}
            >
              <Trash2 className="w-3.5 h-3.5" />
            </Button>
          </div>
        </div>
      </CardContent>
    </Card>
  )
}

// ─── Page ─────────────────────────────────────────────────────────────────────

export default function ResilienceTestsPage() {
  const { t } = useTranslation()
  const [dialogOpen, setDialogOpen] = useState(false)
  const [editId, setEditId] = useState<string | null>(null)
  const [form, setForm] = useState<CreateResilienceTestInput>(emptyForm())
  const [linkTestId, setLinkTestId] = useState<string | null>(null)
  const [linkControlId, setLinkControlId] = useState('')

  const { data, isLoading, isError, error } = useResilienceTests()
  const createTest = useCreateResilienceTest()
  const updateTest = useUpdateResilienceTest(editId ?? '')
  const deleteTest = useDeleteResilienceTest()
  const uploadAttachment = useUploadResilienceTestAttachment(editId ?? '')
  const linkEvidence = useLinkResilienceTestAsEvidence(linkTestId ?? '')

  const tests = data?.tests ?? []
  const tlptOverdueWarning = data?.tlpt_overdue_warning ?? false

  function openCreate() {
    setEditId(null)
    setForm(emptyForm())
    setDialogOpen(true)
  }

  function openEdit(test: ResilienceTest) {
    setEditId(test.id)
    setForm(testToForm(test))
    setDialogOpen(true)
  }

  function handleDelete(id: string) {
    if (confirm(t('vaktcomply.resilienceTests.deleteConfirm'))) {
      deleteTest.mutate(id)
    }
  }

  function handleSubmit() {
    const payload = {
      ...form,
      test_date: form.test_date ? new Date(form.test_date).toISOString() : '',
    }
    if (editId) {
      updateTest.mutate(
        { ...payload, remediation_status: payload.remediation_status ?? 'open' },
        { onSuccess: () => { setDialogOpen(false); } },
      )
    } else {
      createTest.mutate(payload, { onSuccess: () => { setDialogOpen(false); } })
    }
  }

  function handleFileUpload(e: React.ChangeEvent<HTMLInputElement>) {
    const file = e.target.files?.[0]
    if (!file || !editId) return
    const fd = new FormData()
    fd.append('file', file)
    uploadAttachment.mutate(fd)
  }

  const isPending =
    createTest.isPending || updateTest.isPending || uploadAttachment.isPending

  return (
    <div className="flex flex-col h-full">
      <ProGate error={error}>{null}</ProGate>
      {tlptOverdueWarning && (
        <div
          data-testid="tlpt-overdue-warning"
          className="mx-6 mt-4 p-4 rounded-lg bg-red-500/10 border border-red-500/30 text-red-400 text-sm flex items-start gap-2"
        >
          <ShieldAlert className="w-5 h-5 shrink-0 mt-0.5" />
          <span>
            {t('vaktcomply.resilienceTests.overdueWarning')}
          </span>
        </div>
      )}

      <PageHeader
        title={t('vaktcomply.resilienceTests.pageTitle')}
        description={t('vaktcomply.resilienceTests.pageDescription')}
        actions={
          <Button onClick={openCreate}>
            <Plus className="w-4 h-4 mr-1" />
            {t('vaktcomply.resilienceTests.newTest')}
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
            {t('vaktcomply.resilienceTests.loadError')}
          </div>
        )}
        {!isLoading && !isError && tests.length === 0 && (
          <EmptyState
            icon={ShieldAlert}
            title={t('vaktcomply.resilienceTests.emptyTitle')}
            description={t('vaktcomply.resilienceTests.emptyDescription')}
            action={
              <Button onClick={openCreate}>
                <Plus className="w-4 h-4 mr-1" />
                {t('vaktcomply.resilienceTests.newTest')}
              </Button>
            }
          />
        )}
        {!isLoading && !isError && tests.length > 0 && (
          <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 gap-4">
            {tests.map((test) => (
              <ResilienceTestRow
                key={test.id}
                test={test}
                onEdit={() => { openEdit(test); }}
                onDelete={() => { handleDelete(test.id); }}
                onLinkEvidence={() => { setLinkTestId(test.id); setLinkControlId('') }}
              />
            ))}
          </div>
        )}
      </div>

      {/* Link-as-evidence dialog (S40-1) */}
      <Dialog open={!!linkTestId} onOpenChange={(v) => { if (!v) { setLinkTestId(null); } }}>
        <DialogContent className="max-w-sm">
          <DialogHeader>
            <DialogTitle>{t('vaktcomply.resilienceTests.linkEvidenceDialogTitle')}</DialogTitle>
          </DialogHeader>
          <div className="space-y-3 py-2">
            <p className="text-sm text-muted-foreground">
              {t('vaktcomply.resilienceTests.linkEvidenceDesc')}
            </p>
            <div className="space-y-1.5">
              <Label>{t('vaktcomply.resilienceTests.controlIdLabel')}</Label>
              <Input
                placeholder={t('vaktcomply.resilienceTests.controlIdPlaceholder')}
                value={linkControlId}
                onChange={(e) => { setLinkControlId(e.target.value); }}
              />
            </div>
          </div>
          <DialogFooter>
            <Button variant="outline" onClick={() => { setLinkTestId(null); }}>
              {t('vaktcomply.resilienceTests.cancel')}
            </Button>
            <Button
              disabled={!linkControlId || linkEvidence.isPending}
              onClick={() => {
                linkEvidence.mutate({ control_id: linkControlId }, {
                  onSuccess: () => { setLinkTestId(null); },
                })
              }}
            >
              {linkEvidence.isPending ? t('vaktcomply.resilienceTests.linking') : t('vaktcomply.resilienceTests.saveAsEvidence')}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      <Dialog open={dialogOpen} onOpenChange={setDialogOpen}>
        <DialogContent className="max-w-lg max-h-[90vh] overflow-y-auto">
          <DialogHeader>
            <DialogTitle>
              {editId ? t('vaktcomply.resilienceTests.dialogTitleEdit') : t('vaktcomply.resilienceTests.dialogTitleNew')}
            </DialogTitle>
          </DialogHeader>
          <div className="space-y-4 py-2">
            <div className="space-y-1.5">
              <Label>{t('vaktcomply.resilienceTests.typeLabelRequired')}</Label>
              <Select
                value={form.type}
                onValueChange={(v) => { setForm((f) => ({ ...f, type: v })); }}
              >
                <SelectTrigger>
                  <SelectValue placeholder={t('vaktcomply.resilienceTests.typePlaceholder')} />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="tlpt">TLPT</SelectItem>
                  <SelectItem value="pentest">{t('vaktcomply.resilienceTests.kind.pentest')}</SelectItem>
                  <SelectItem value="scenario_based">{t('vaktcomply.resilienceTests.kind.scenario')}</SelectItem>
                  <SelectItem value="vulnerability_assessment">
                    {t('vaktcomply.resilienceTests.typeLabels.vulnerabilityAssessment')}
                  </SelectItem>
                </SelectContent>
              </Select>
            </div>

            <div className="space-y-1.5">
              <Label>{t('vaktcomply.resilienceTests.dateLabelRequired')}</Label>
              <Input
                type="date"
                value={form.test_date ?? ''}
                onChange={(e) => { setForm((f) => ({ ...f, test_date: e.target.value })); }}
              />
            </div>

            <div className="space-y-1.5">
              <Label>{t('vaktcomply.resilienceTests.scopeLabel')}</Label>
              <Input
                placeholder={t('vaktcomply.resilienceTests.scopePlaceholder')}
                value={form.scope ?? ''}
                onChange={(e) => { setForm((f) => ({ ...f, scope: e.target.value })); }}
              />
            </div>

            <div className="space-y-1.5">
              <Label>{t('vaktcomply.resilienceTests.providerLabel')}</Label>
              <Input
                placeholder={t('vaktcomply.resilienceTests.providerPlaceholder')}
                value={form.provider ?? ''}
                onChange={(e) => { setForm((f) => ({ ...f, provider: e.target.value })); }}
              />
            </div>

            <div className="space-y-1.5">
              <Label>{t('vaktcomply.resilienceTests.remediationStatus')}</Label>
              <Select
                value={form.remediation_status ?? 'open'}
                onValueChange={(v) => { setForm((f) => ({ ...f, remediation_status: v })); }}
              >
                <SelectTrigger>
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="open">{t('vaktcomply.resilienceTests.status.open')}</SelectItem>
                  <SelectItem value="in_progress">{t('vaktcomply.resilienceTests.status.in_progress')}</SelectItem>
                  <SelectItem value="completed">{t('vaktcomply.resilienceTests.status.completed')}</SelectItem>
                  <SelectItem value="accepted">{t('vaktcomply.resilienceTests.status.accepted')}</SelectItem>
                </SelectContent>
              </Select>
            </div>

            <div className="space-y-1.5">
              <Label>{t('vaktcomply.resilienceTests.summaryLabel')}</Label>
              <Textarea
                rows={4}
                placeholder={t('vaktcomply.resilienceTests.summaryPlaceholder')}
                value={form.summary ?? ''}
                onChange={(e) => { setForm((f) => ({ ...f, summary: e.target.value })); }}
              />
            </div>

            {editId && (
              <div className="space-y-1.5">
                <Label>{t('vaktcomply.resilienceTests.attachmentLabel')}</Label>
                <Input type="file" onChange={handleFileUpload} />
              </div>
            )}
          </div>
          <DialogFooter>
            <Button variant="outline" onClick={() => { setDialogOpen(false); }}>
              {t('vaktcomply.resilienceTests.cancel')}
            </Button>
            <Button
              onClick={handleSubmit}
              disabled={!form.type || !form.test_date || isPending}
            >
              {isPending
                ? t('vaktcomply.resilienceTests.saving')
                : editId
                  ? t('vaktcomply.resilienceTests.save')
                  : t('vaktcomply.resilienceTests.add')}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </div>
  )
}
