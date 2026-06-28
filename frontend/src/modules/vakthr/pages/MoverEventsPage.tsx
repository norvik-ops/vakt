import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { Plus, ArrowRightLeft, CheckCircle2, Clock } from 'lucide-react'
import { Button } from '../../../components/ui/button'
import { Badge } from '../../../components/ui/badge'
import { Dialog, DialogContent, DialogHeader, DialogTitle, DialogFooter } from '../../../components/ui/dialog'
import { Input } from '../../../components/ui/input'
import { Label } from '../../../components/ui/label'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '../../../components/ui/select'
import { apiFetch } from '../../../api/client'
import { EmptyState } from '../../../shared/components/EmptyState'
import { SkeletonTable } from '../../../shared/components/SkeletonLoaders'
import { PageHeader } from '../../../shared/components/PageHeader'
import { useEmployees } from '../hooks/useHR'

interface MoverEvent {
  id: string
  org_id: string
  employee_id: string
  from_department?: string
  from_job_title?: string
  to_department: string
  to_job_title: string
  effective_date: string
  due_date: string
  status: 'pending' | 'in_progress' | 'completed' | 'overdue' | 'cancelled'
  completed_at?: string
  created_at: string
}

const STATUS_BADGE: Record<string, { label: string; variant: 'outline' | 'warning' | 'success' | 'destructive' }> = {
  pending: { label: 'Ausstehend', variant: 'outline' },
  in_progress: { label: 'In Bearbeitung', variant: 'warning' },
  completed: { label: 'Abgeschlossen', variant: 'success' },
  overdue: { label: 'Überfällig', variant: 'destructive' },
  cancelled: { label: 'Abgebrochen', variant: 'destructive' },
}

function MoverEventRow({ event, onStatusUpdate }: { event: MoverEvent; onStatusUpdate: (id: string, status: string) => void }) {
  const { t } = useTranslation()
  const badge = STATUS_BADGE[event.status] ?? { label: event.status, variant: 'outline' as const }
  const statusLabels: Record<string, string> = {
    pending: t('vakthr.mover.statusPending'),
    in_progress: t('vakthr.mover.statusInProgress'),
    completed: t('vakthr.mover.statusCompleted'),
    overdue: t('vakthr.mover.statusOverdue'),
    cancelled: t('vakthr.mover.statusCancelled'),
  }
  const displayLabel = statusLabels[event.status] ?? badge.label
  const fromLabel = [event.from_job_title, event.from_department].filter(Boolean).join(' / ') || '–'
  const toLabel = [event.to_job_title, event.to_department].filter(Boolean).join(' / ')
  return (
    <div className="flex items-start justify-between px-4 py-3 bg-surface border border-border rounded-lg gap-3">
      <div className="flex items-start gap-3 min-w-0">
        <ArrowRightLeft className="w-4 h-4 text-secondary mt-0.5 shrink-0" />
        <div className="min-w-0">
          <p className="text-sm font-medium font-mono text-xs text-secondary">{event.employee_id.slice(0, 8)}…</p>
          <p className="text-sm font-medium">
            {fromLabel} → {toLabel}
          </p>
          <p className="text-xs text-secondary">
            {t('vakthr.mover.effective', { date: new Date(event.effective_date).toLocaleDateString('de-DE') })} · {t('vakthr.mover.due', { date: new Date(event.due_date).toLocaleDateString('de-DE') })}
          </p>
        </div>
      </div>
      <div className="flex items-center gap-2 shrink-0">
        <Badge variant={badge.variant} className="text-xs">{displayLabel}</Badge>
        {event.status === 'pending' && (
          <Button size="sm" variant="outline" onClick={() => { onStatusUpdate(event.id, 'in_progress'); }}>
            {t('vakthr.mover.startButton')}
          </Button>
        )}
        {event.status === 'in_progress' && (
          <Button size="sm" variant="outline" onClick={() => { onStatusUpdate(event.id, 'completed'); }}>
            <CheckCircle2 className="w-3 h-3 mr-1" />
            {t('vakthr.mover.completeButton')}
          </Button>
        )}
      </div>
    </div>
  )
}

export default function MoverEventsPage() {
  const { t } = useTranslation()
  const qc = useQueryClient()
  const [showCreate, setShowCreate] = useState(false)

  const { data: events, isLoading } = useQuery<MoverEvent[]>({
    queryKey: ['hr', 'mover-events'],
    queryFn: () => apiFetch<MoverEvent[]>('/vakthr/mover-events'),
    staleTime: 2 * 60 * 1000,
  })

  const { data: employees = [] } = useEmployees(1, 100)

  const createMutation = useMutation({
    mutationFn: (body: Record<string, unknown>) =>
      apiFetch('/vakthr/mover-events', { method: 'POST', body: JSON.stringify(body) }),
    onSuccess: () => {
      void qc.invalidateQueries({ queryKey: ['hr', 'mover-events'] })
      setShowCreate(false)
    },
  })

  const statusMutation = useMutation({
    mutationFn: ({ id, status }: { id: string; status: string }) =>
      apiFetch(`/vakthr/mover-events/${id}/status`, { method: 'PATCH', body: JSON.stringify({ status }) }),
    onSuccess: () => {
      void qc.invalidateQueries({ queryKey: ['hr', 'mover-events'] })
    },
  })

  function handleCreate(e: React.FormEvent<HTMLFormElement>) {
    e.preventDefault()
    const fd = new FormData(e.currentTarget)
    createMutation.mutate({
      employee_id: fd.get('employee_id'),
      from_job_title: fd.get('from_job_title') || undefined,
      from_department: fd.get('from_department') || undefined,
      to_job_title: fd.get('to_job_title'),
      to_department: fd.get('to_department'),
      effective_date: fd.get('effective_date'),
    })
  }

  const active = (events ?? []).filter((e) => e.status === 'pending' || e.status === 'in_progress' || e.status === 'overdue')
  const done = (events ?? []).filter((e) => e.status === 'completed' || e.status === 'cancelled')

  return (
    <div className="flex flex-col h-full">
      <PageHeader
        title={t('vakthr.mover.title')}
        description={t('vakthr.mover.description')}
        actions={
          <Button size="sm" onClick={() => { setShowCreate(true); }}>
            <Plus className="w-4 h-4 mr-1" />
            {t('vakthr.mover.addButton')}
          </Button>
        }
      />

      <div className="flex-1 p-6 space-y-6">
        {isLoading ? (
          <SkeletonTable rows={4} cols={3} />
        ) : !events || events.length === 0 ? (
          <EmptyState
            icon={ArrowRightLeft}
            title={t('vakthr.mover.emptyTitle')}
            description={t('vakthr.mover.emptyDesc')}
          />
        ) : (
          <div className="space-y-6">
            {active.length > 0 && (
              <div className="space-y-2">
                <div className="flex items-center gap-2 text-xs font-medium text-secondary uppercase tracking-wide">
                  <Clock className="w-3 h-3" />
                  {t('vakthr.mover.groupOpen', { count: active.length })}
                </div>
                {active.map((ev) => (
                  <MoverEventRow
                    key={ev.id}
                    event={ev}
                    onStatusUpdate={(id, status) => { statusMutation.mutate({ id, status }); }}
                  />
                ))}
              </div>
            )}
            {done.length > 0 && (
              <div className="space-y-2">
                <div className="flex items-center gap-2 text-xs font-medium text-secondary uppercase tracking-wide">
                  <CheckCircle2 className="w-3 h-3" />
                  {t('vakthr.mover.groupDone', { count: done.length })}
                </div>
                {done.map((ev) => (
                  <MoverEventRow
                    key={ev.id}
                    event={ev}
                    onStatusUpdate={(id, status) => { statusMutation.mutate({ id, status }); }}
                  />
                ))}
              </div>
            )}
          </div>
        )}
      </div>

      <Dialog open={showCreate} onOpenChange={setShowCreate}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>{t('vakthr.mover.dialogTitle')}</DialogTitle>
          </DialogHeader>
          <form onSubmit={handleCreate} className="space-y-4">
            <div className="space-y-1.5">
              <Label htmlFor="employee_id">{t('vakthr.checklistRunPage.employee')}</Label>
              {employees.length > 0 ? (
                <Select name="employee_id" required>
                  <SelectTrigger><SelectValue placeholder={t('vakthr.mover.selectEmployee')} /></SelectTrigger>
                  <SelectContent>
                    {employees.map((emp) => (
                      <SelectItem key={emp.id} value={emp.id}>
                        {emp.first_name} {emp.last_name} ({emp.email})
                      </SelectItem>
                    ))}
                  </SelectContent>
                </Select>
              ) : (
                <Input id="employee_id" name="employee_id" required placeholder="Employee UUID" pattern="[0-9a-f-]{36}" />
              )}
            </div>
            <div className="grid grid-cols-2 gap-3">
              <div className="space-y-1.5">
                <Label htmlFor="from_job_title">{t('vakthr.mover.fromJob')}</Label>
                <Input id="from_job_title" name="from_job_title" placeholder="Junior Developer" />
              </div>
              <div className="space-y-1.5">
                <Label htmlFor="from_department">{t('vakthr.mover.fromDept')}</Label>
                <Input id="from_department" name="from_department" placeholder="Engineering" />
              </div>
            </div>
            <div className="grid grid-cols-2 gap-3">
              <div className="space-y-1.5">
                <Label htmlFor="to_job_title">{t('vakthr.mover.toJob')} *</Label>
                <Input id="to_job_title" name="to_job_title" required placeholder="Senior Developer" />
              </div>
              <div className="space-y-1.5">
                <Label htmlFor="to_department">{t('vakthr.mover.toDept')} *</Label>
                <Input id="to_department" name="to_department" required placeholder="Platform" />
              </div>
            </div>
            <div className="space-y-1.5">
              <Label htmlFor="effective_date">{t('vakthr.mover.effectiveDate')} *</Label>
              <Input id="effective_date" name="effective_date" type="date" required />
            </div>
            <DialogFooter>
              <Button type="button" variant="outline" onClick={() => { setShowCreate(false); }}>{t('common.cancel')}</Button>
              <Button type="submit" disabled={createMutation.isPending}>
                {createMutation.isPending ? t('common.saving') : t('vakthr.mover.submit')}
              </Button>
            </DialogFooter>
          </form>
        </DialogContent>
      </Dialog>
    </div>
  )
}
