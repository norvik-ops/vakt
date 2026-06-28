import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { useNavigate } from 'react-router-dom'
import { ShieldAlert, ExternalLink, Trash2, AlertTriangle, CheckCircle2, XCircle } from 'lucide-react'
import { Button } from '../../../components/ui/button'
import { Badge } from '../../../components/ui/badge'
import { Card, CardContent, CardHeader, CardTitle } from '../../../components/ui/card'
import { Dialog, DialogContent, DialogHeader, DialogTitle, DialogFooter } from '../../../components/ui/dialog'
import { PageHeader } from '../../../shared/components/PageHeader'
import { EmptyState } from '../../../shared/components/EmptyState'
import { useAuthStore } from '../../../shared/stores/auth'
import { useDeferredDelete } from '../../../shared/hooks/useDeferredDelete'
import { useQuery, useQueryClient } from '@tanstack/react-query'
import { apiFetch } from '../../../api/client'
import { useFormatDate } from '../../../shared/hooks/useFormatDate'
import {
  useDeleteControlException,
  type ControlException,
} from '../hooks/useExceptions'

function StatusBadge({ status }: { status: ControlException['status'] }) {
  const { t } = useTranslation()
  if (status === 'active') return <Badge className="bg-amber-500/20 text-amber-400 border-amber-500/30">{t('vaktcomply.exceptions.status.active')}</Badge>
  if (status === 'expired') return <Badge variant="secondary">{t('vaktcomply.exceptions.status.expired')}</Badge>
  return <Badge variant="destructive">{t('vaktcomply.exceptions.status.revoked')}</Badge>
}

function statusIcon(status: ControlException['status']) {
  if (status === 'active') return <AlertTriangle className="w-4 h-4 text-amber-400" />
  if (status === 'expired') return <XCircle className="w-4 h-4 text-slate-400" />
  return <CheckCircle2 className="w-4 h-4 text-red-400" />
}

export default function ExceptionsPage() {
  const { t } = useTranslation()
  const navigate = useNavigate()
  const { user } = useAuthStore()
  const isAdmin = user?.roles?.includes('Admin') ?? false
  const [confirmDelete, setConfirmDelete] = useState<ControlException | null>(null)
  // Optimistically hidden items — ids of items removed from view while timer is running
  const [hiddenIds, setHiddenIds] = useState<Set<string>>(new Set())

  const queryClient = useQueryClient()

  const { data: exceptions = [], isLoading, error } = useQuery<ControlException[]>({
    queryKey: ['vaktcomply', 'exceptions'],
    queryFn: () => apiFetch<ControlException[]>('/vaktcomply/exceptions'),
    staleTime: 2 * 60 * 1000,
  })

  const deleteException = useDeleteControlException()

  const { scheduleDelete } = useDeferredDelete<ControlException>({
    getLabel: (e) => e.title,
    onDelete: async (e) => {
      await deleteException.mutateAsync({ id: e.id, controlId: e.control_id })
      // Remove from hidden set after confirmed delete
      setHiddenIds((prev) => {
        const next = new Set(prev)
        next.delete(e.id)
        return next
      })
    },
    onUndo: (_e) => {
      // Refetch from server — item reappears
      void queryClient.invalidateQueries({ queryKey: ['vaktcomply', 'exceptions'] })
      setHiddenIds((prev) => {
        const next = new Set(prev)
        next.delete(_e.id)
        return next
      })
    },
    delayMs: 5000,
  })

  const handleConfirmDelete = () => {
    if (!confirmDelete) return
    const item = confirmDelete
    setConfirmDelete(null)
    scheduleDelete(item, () => {
      // Optimistically remove from view
      setHiddenIds((prev) => new Set(prev).add(item.id))
    })
  }

  const visibleExceptions = exceptions.filter((e) => !hiddenIds.has(e.id))
  const active = visibleExceptions.filter(e => e.status === 'active')
  const inactive = visibleExceptions.filter(e => e.status !== 'active')

  return (
    <div className="space-y-6">
      <PageHeader
        title={t('vaktcomply.exceptions.title')}
        description={t('vaktcomply.exceptions.description')}
      />

      {isLoading && (
        <div className="flex items-center justify-center py-16 text-slate-400">{t('vaktcomply.exceptions.loading')}</div>
      )}
      {error && (
        <div className="text-red-400 text-sm">{t('vaktcomply.exceptions.errorLoading')}</div>
      )}

      {!isLoading && !error && visibleExceptions.length === 0 && (
        <EmptyState
          icon={ShieldAlert}
          title={t('vaktcomply.exceptions.emptyTitle')}
          description={t('vaktcomply.exceptions.emptyDescription')}
          action={
            <Button size="sm" variant="outline" onClick={() => { navigate('/vaktcomply/frameworks'); }}>
              {t('vaktcomply.exceptions.showControls')}
            </Button>
          }
        />
      )}

      {active.length > 0 && (
        <div className="space-y-3">
          <h2 className="text-sm font-semibold text-slate-300 uppercase tracking-wide">{t('vaktcomply.exceptions.activeSection', { count: active.length })}</h2>
          {active.map(e => (
            <ExceptionCard
              key={e.id}
              exception={e}
              isAdmin={isAdmin}
              onNavigate={() => { navigate(`/vaktcomply/controls/${e.control_id}`); }}
              onDelete={() => { setConfirmDelete(e); }}
            />
          ))}
        </div>
      )}

      {inactive.length > 0 && (
        <div className="space-y-3">
          <h2 className="text-sm font-semibold text-slate-400 uppercase tracking-wide">{t('vaktcomply.exceptions.inactiveSection', { count: inactive.length })}</h2>
          {inactive.map(e => (
            <ExceptionCard
              key={e.id}
              exception={e}
              isAdmin={isAdmin}
              onNavigate={() => { navigate(`/vaktcomply/controls/${e.control_id}`); }}
              onDelete={() => { setConfirmDelete(e); }}
            />
          ))}
        </div>
      )}

      {/* Delete confirm dialog — kept intentionally: exceptions are important compliance records */}
      <Dialog open={!!confirmDelete} onOpenChange={() => { setConfirmDelete(null); }}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>{t('vaktcomply.exceptions.deletePrompt')}</DialogTitle>
          </DialogHeader>
          <p className="text-sm text-slate-400">
            {t('vaktcomply.exceptions.deleteConfirmDescription', { title: confirmDelete?.title ?? '' })}
          </p>
          <DialogFooter>
            <Button variant="ghost" onClick={() => { setConfirmDelete(null); }}>{t('vaktcomply.exceptions.cancel')}</Button>
            <Button variant="destructive" onClick={handleConfirmDelete}>
              {t('vaktcomply.exceptions.delete')}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </div>
  )
}

function ExceptionCard({
  exception: e,
  isAdmin,
  onNavigate,
  onDelete,
}: {
  exception: ControlException
  isAdmin: boolean
  onNavigate: () => void
  onDelete: () => void
}) {
  const { t } = useTranslation()
  const { formatDate } = useFormatDate()
  return (
    <Card className="bg-slate-800/50 border-slate-700">
      <CardHeader className="pb-2">
        <div className="flex items-start justify-between gap-3">
          <div className="flex items-center gap-2">
            {statusIcon(e.status)}
            <CardTitle className="text-sm font-medium text-slate-200">{e.title}</CardTitle>
            <StatusBadge status={e.status} />
          </div>
          <div className="flex items-center gap-2">
            <Button
              variant="ghost"
              size="sm"
              className="text-slate-400 hover:text-slate-200 h-7 px-2"
              onClick={onNavigate}
            >
              <ExternalLink className="w-3.5 h-3.5 mr-1" />
              {t('vaktcomply.exceptions.showControl')}
            </Button>
            {isAdmin && (
              <Button
                variant="ghost"
                size="sm"
                className="text-slate-400 hover:text-red-400 h-7 px-2"
                onClick={onDelete}
              >
                <Trash2 className="w-3.5 h-3.5" />
              </Button>
            )}
          </div>
        </div>
      </CardHeader>
      <CardContent className="text-xs text-slate-400 space-y-1">
        <p><span className="text-slate-300">{t('vaktcomply.exceptions.fieldReason')}:</span> {e.reason}</p>
        <p><span className="text-slate-300">{t('vaktcomply.exceptions.fieldRiskAccepted')}:</span> {e.risk_accepted}</p>
        {e.approved_by && <p><span className="text-slate-300">{t('vaktcomply.exceptions.fieldApprovedBy')}:</span> {e.approved_by}</p>}
        {e.expires_at && (
          <p><span className="text-slate-300">{t('vaktcomply.exceptions.fieldExpiresAt')}:</span> {formatDate(e.expires_at)}</p>
        )}
        <p className="text-slate-500">{t('vaktcomply.exceptions.fieldCreatedAt')}: {formatDate(e.created_at)}</p>
      </CardContent>
    </Card>
  )
}
