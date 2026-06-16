import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { DatabaseBackup, Plus, Pencil, Trash2, ShieldCheck, AlertTriangle } from 'lucide-react'
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
  useBackupJobs,
  useBackupSummary,
  useCreateBackupJob,
  useUpdateBackupJob,
  useDeleteBackupJob,
  useCreateRestoreTest,
} from '../hooks/useBackupJobs'
import type {
  BackupJob,
  BackupJobInput,
  BackupFrequency,
  StalenessStatus,
  RestoreTestInput,
  RestoreResult,
} from '../types'

const STATUS_CLASS: Record<StalenessStatus, string> = {
  on_track: 'bg-green-500/20 text-green-400 border-green-500/30',
  at_risk: 'bg-amber-500/20 text-amber-400 border-amber-500/30',
  overdue: 'bg-red-500/20 text-red-400 border-red-500/30',
}

function emptyJob(): BackupJobInput {
  return {
    name: '',
    source: '',
    destination: '',
    frequency: 'daily',
    encrypted: true,
    restore_max_age_days: 365,
    notes: '',
  }
}

function jobToForm(j: BackupJob): BackupJobInput {
  return {
    name: j.name,
    source: j.source,
    destination: j.destination,
    frequency: j.frequency,
    encrypted: j.encrypted,
    restore_max_age_days: j.restore_max_age_days,
    notes: j.notes,
    last_status: j.last_status,
    last_success_at: j.last_success_at ?? undefined,
  }
}

function emptyRestoreTest(): RestoreTestInput {
  return {
    tested_at: new Date().toISOString().slice(0, 10),
    result: 'success',
    rto_target_hours: 4,
    rto_actual_hours: 0,
    tester: '',
    notes: '',
  }
}

function RestoreTestDialog({ jobId, onClose }: { jobId: string; onClose: () => void }) {
  const { t } = useTranslation()
  const [form, setForm] = useState<RestoreTestInput>(emptyRestoreTest())
  const create = useCreateRestoreTest(jobId)

  function submit() {
    create.mutate(form, { onSuccess: onClose })
  }

  return (
    <Dialog open onOpenChange={(o) => { if (!o) onClose() }}>
      <DialogContent className="max-w-lg">
        <DialogHeader>
          <DialogTitle>{t('backup.restoreTest.new')}</DialogTitle>
        </DialogHeader>
        <div className="space-y-3 py-2">
          <div className="grid grid-cols-2 gap-3">
            <div className="space-y-1.5">
              <Label>{t('backup.restoreTest.testedAt')} *</Label>
              <Input
                type="date"
                value={form.tested_at}
                onChange={(e) => { setForm((f) => ({ ...f, tested_at: e.target.value })) }}
              />
            </div>
            <div className="space-y-1.5">
              <Label>{t('backup.restoreTest.result')} *</Label>
              <Select
                value={form.result}
                onValueChange={(v) => { setForm((f) => ({ ...f, result: v as RestoreResult })) }}
              >
                <SelectTrigger><SelectValue /></SelectTrigger>
                <SelectContent>
                  <SelectItem value="success">{t('backup.restoreTest.resultSuccess')}</SelectItem>
                  <SelectItem value="partial">{t('backup.restoreTest.resultPartial')}</SelectItem>
                  <SelectItem value="failed">{t('backup.restoreTest.resultFailed')}</SelectItem>
                </SelectContent>
              </Select>
            </div>
          </div>
          <div className="grid grid-cols-2 gap-3">
            <div className="space-y-1.5">
              <Label>{t('backup.restoreTest.rtoTarget')}</Label>
              <Input
                type="number"
                min={0}
                value={form.rto_target_hours}
                onChange={(e) => { setForm((f) => ({ ...f, rto_target_hours: Number(e.target.value) })) }}
              />
            </div>
            <div className="space-y-1.5">
              <Label>{t('backup.restoreTest.rtoActual')}</Label>
              <Input
                type="number"
                min={0}
                value={form.rto_actual_hours}
                onChange={(e) => { setForm((f) => ({ ...f, rto_actual_hours: Number(e.target.value) })) }}
              />
            </div>
          </div>
          <div className="space-y-1.5">
            <Label>{t('backup.restoreTest.tester')}</Label>
            <Input
              value={form.tester ?? ''}
              onChange={(e) => { setForm((f) => ({ ...f, tester: e.target.value })) }}
            />
          </div>
          <div className="space-y-1.5">
            <Label>{t('backup.restoreTest.notes')}</Label>
            <Textarea
              rows={2}
              value={form.notes ?? ''}
              onChange={(e) => { setForm((f) => ({ ...f, notes: e.target.value })) }}
            />
          </div>
        </div>
        <DialogFooter>
          <Button variant="outline" onClick={onClose}>{t('common.cancel')}</Button>
          <Button onClick={submit} disabled={create.isPending}>
            {create.isPending ? t('common.saving') : t('common.add')}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}

export default function BackupEvidencePage() {
  const { t } = useTranslation()
  const [dialogOpen, setDialogOpen] = useState(false)
  const [editId, setEditId] = useState<string | null>(null)
  const [form, setForm] = useState<BackupJobInput>(emptyJob())
  const [restoreJobId, setRestoreJobId] = useState<string | null>(null)

  const { data: jobs = [], isLoading, isError } = useBackupJobs()
  const { data: summary } = useBackupSummary()
  const create = useCreateBackupJob()
  const update = useUpdateBackupJob(editId ?? '')
  const del = useDeleteBackupJob()

  function openCreate() {
    setEditId(null)
    setForm(emptyJob())
    setDialogOpen(true)
  }
  function openEdit(j: BackupJob) {
    setEditId(j.id)
    setForm(jobToForm(j))
    setDialogOpen(true)
  }
  function handleDelete(id: string) {
    if (confirm(t('backup.deleteConfirm'))) del.mutate(id)
  }
  function handleSubmit() {
    const action = editId ? update : create
    action.mutate(form, { onSuccess: () => { setDialogOpen(false) } })
  }
  const isPending = create.isPending || update.isPending

  return (
    <div className="flex flex-col h-full">
      <PageHeader
        title={t('backup.title')}
        description={t('backup.description')}
        actions={
          <Button onClick={openCreate}>
            <Plus className="w-4 h-4 mr-1" />
            {t('backup.new')}
          </Button>
        }
      />

      <div className="flex-1 p-6 space-y-6">
        {summary && (
          <div className="grid grid-cols-2 lg:grid-cols-4 gap-4">
            <Card><CardContent className="pt-5">
              <p className="text-xs text-muted-foreground flex items-center gap-1.5">
                <DatabaseBackup className="w-3.5 h-3.5" />{t('backup.summary.total')}
              </p>
              <p className="text-2xl font-bold">{summary.total_jobs}</p>
            </CardContent></Card>
            <Card><CardContent className="pt-5">
              <p className="text-xs text-muted-foreground flex items-center gap-1.5">
                <ShieldCheck className="w-3.5 h-3.5" />{t('backup.summary.tested')}
              </p>
              <p className="text-2xl font-bold">{summary.tested_jobs}</p>
            </CardContent></Card>
            <Card><CardContent className="pt-5">
              <p className="text-xs text-muted-foreground flex items-center gap-1.5">
                <AlertTriangle className="w-3.5 h-3.5" />{t('backup.summary.overdueBackups')}
              </p>
              <p className={`text-2xl font-bold ${summary.overdue_backups > 0 ? 'text-red-400' : ''}`}>
                {summary.overdue_backups}
              </p>
            </CardContent></Card>
            <Card><CardContent className="pt-5">
              <p className="text-xs text-muted-foreground flex items-center gap-1.5">
                <AlertTriangle className="w-3.5 h-3.5" />{t('backup.summary.overdueRestores')}
              </p>
              <p className={`text-2xl font-bold ${summary.overdue_restores > 0 ? 'text-red-400' : ''}`}>
                {summary.overdue_restores}
              </p>
            </CardContent></Card>
          </div>
        )}

        {isLoading && (
          <div className="flex items-center justify-center h-48"><Spinner size="lg" color="primary" /></div>
        )}
        {isError && (
          <div className="text-sm text-red-400 p-4 bg-red-500/10 rounded-lg">{t('backup.loadError')}</div>
        )}
        {!isLoading && !isError && jobs.length === 0 && (
          <EmptyState
            icon={DatabaseBackup}
            title={t('backup.emptyTitle')}
            description={t('backup.emptyDescription')}
            action={<Button onClick={openCreate}><Plus className="w-4 h-4 mr-1" />{t('backup.new')}</Button>}
          />
        )}
        {!isLoading && !isError && jobs.length > 0 && (
          <div className="grid grid-cols-1 lg:grid-cols-2 gap-4">
            {jobs.map((j) => (
              <Card key={j.id}>
                <CardHeader className="pb-2">
                  <div className="flex items-start justify-between gap-2">
                    <div className="flex-1 min-w-0">
                      <CardTitle className="text-sm leading-tight">{j.name}</CardTitle>
                      <div className="flex items-center gap-1.5 flex-wrap mt-1.5">
                        <Badge variant="outline" className="text-xs">{t(`backup.frequency.${j.frequency}`)}</Badge>
                        {j.encrypted && (
                          <Badge variant="outline" className="text-xs">{t('backup.encrypted')}</Badge>
                        )}
                        <Badge className={STATUS_CLASS[j.backup_status]} variant="outline">
                          {t('backup.label.backup')}: {t(`backup.status.${j.backup_status}`)}
                        </Badge>
                        <Badge className={STATUS_CLASS[j.restore_status]} variant="outline">
                          {t('backup.label.restore')}: {t(`backup.status.${j.restore_status}`)}
                        </Badge>
                      </div>
                    </div>
                    <div className="flex items-center gap-1 shrink-0">
                      <Button variant="ghost" size="icon" className="h-7 w-7" onClick={() => { openEdit(j) }}>
                        <Pencil className="w-3.5 h-3.5" />
                      </Button>
                      <Button variant="ghost" size="icon" className="h-7 w-7 text-red-400 hover:text-red-300" onClick={() => { handleDelete(j.id) }}>
                        <Trash2 className="w-3.5 h-3.5" />
                      </Button>
                    </div>
                  </div>
                </CardHeader>
                <CardContent className="pt-0 space-y-1.5 text-xs text-muted-foreground">
                  {j.source && <p>{t('backup.source')}: {j.source} → {j.destination}</p>}
                  <p>
                    {t('backup.lastSuccess')}:{' '}
                    {j.last_success_at ? new Date(j.last_success_at).toLocaleString() : t('backup.never')}
                  </p>
                  <p>
                    {t('backup.lastRestoreTest')}:{' '}
                    {j.last_restore_test_at ? new Date(j.last_restore_test_at).toLocaleDateString() : t('backup.never')}
                  </p>
                  <Button variant="outline" size="sm" className="mt-2" onClick={() => { setRestoreJobId(j.id) }}>
                    {t('backup.restoreTest.document')}
                  </Button>
                </CardContent>
              </Card>
            ))}
          </div>
        )}
      </div>

      <Dialog open={dialogOpen} onOpenChange={setDialogOpen}>
        <DialogContent className="max-w-lg">
          <DialogHeader>
            <DialogTitle>{editId ? t('backup.edit') : t('backup.new')}</DialogTitle>
          </DialogHeader>
          <div className="space-y-3 py-2">
            <div className="space-y-1.5">
              <Label>{t('backup.name')} *</Label>
              <Input value={form.name} onChange={(e) => { setForm((f) => ({ ...f, name: e.target.value })) }} />
            </div>
            <div className="grid grid-cols-2 gap-3">
              <div className="space-y-1.5">
                <Label>{t('backup.source')}</Label>
                <Input value={form.source ?? ''} onChange={(e) => { setForm((f) => ({ ...f, source: e.target.value })) }} />
              </div>
              <div className="space-y-1.5">
                <Label>{t('backup.destination')}</Label>
                <Input value={form.destination ?? ''} onChange={(e) => { setForm((f) => ({ ...f, destination: e.target.value })) }} />
              </div>
            </div>
            <div className="grid grid-cols-2 gap-3">
              <div className="space-y-1.5">
                <Label>{t('backup.frequencyLabel')} *</Label>
                <Select value={form.frequency} onValueChange={(v) => { setForm((f) => ({ ...f, frequency: v as BackupFrequency })) }}>
                  <SelectTrigger><SelectValue /></SelectTrigger>
                  <SelectContent>
                    <SelectItem value="hourly">{t('backup.frequency.hourly')}</SelectItem>
                    <SelectItem value="daily">{t('backup.frequency.daily')}</SelectItem>
                    <SelectItem value="weekly">{t('backup.frequency.weekly')}</SelectItem>
                    <SelectItem value="monthly">{t('backup.frequency.monthly')}</SelectItem>
                  </SelectContent>
                </Select>
              </div>
              <div className="space-y-1.5">
                <Label>{t('backup.restoreMaxAge')}</Label>
                <Input
                  type="number"
                  min={1}
                  value={form.restore_max_age_days}
                  onChange={(e) => { setForm((f) => ({ ...f, restore_max_age_days: Number(e.target.value) })) }}
                />
              </div>
            </div>
            <label className="flex items-center gap-2 text-sm">
              <input
                type="checkbox"
                checked={form.encrypted}
                onChange={(e) => { setForm((f) => ({ ...f, encrypted: e.target.checked })) }}
              />
              {t('backup.encryptedLabel')}
            </label>
            <div className="space-y-1.5">
              <Label>{t('backup.notes')}</Label>
              <Textarea rows={2} value={form.notes ?? ''} onChange={(e) => { setForm((f) => ({ ...f, notes: e.target.value })) }} />
            </div>
          </div>
          <DialogFooter>
            <Button variant="outline" onClick={() => { setDialogOpen(false) }}>{t('common.cancel')}</Button>
            <Button onClick={handleSubmit} disabled={!form.name || isPending}>
              {isPending ? t('common.saving') : editId ? t('common.save') : t('common.add')}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      {restoreJobId && (
        <RestoreTestDialog jobId={restoreJobId} onClose={() => { setRestoreJobId(null) }} />
      )}
    </div>
  )
}
