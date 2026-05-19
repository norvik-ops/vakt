import { useState } from 'react'
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
import {
  useDeleteControlException,
  type ControlException,
} from '../hooks/useExceptions'

function statusBadge(status: ControlException['status']) {
  if (status === 'active') return <Badge className="bg-amber-500/20 text-amber-400 border-amber-500/30">Aktiv</Badge>
  if (status === 'expired') return <Badge variant="secondary">Abgelaufen</Badge>
  return <Badge variant="destructive">Widerrufen</Badge>
}

function statusIcon(status: ControlException['status']) {
  if (status === 'active') return <AlertTriangle className="w-4 h-4 text-amber-400" />
  if (status === 'expired') return <XCircle className="w-4 h-4 text-slate-400" />
  return <CheckCircle2 className="w-4 h-4 text-red-400" />
}

export default function ExceptionsPage() {
  const navigate = useNavigate()
  const { user } = useAuthStore()
  const isAdmin = user?.roles?.includes('Admin') ?? false
  const [confirmDelete, setConfirmDelete] = useState<ControlException | null>(null)
  // Optimistically hidden items — ids of items removed from view while timer is running
  const [hiddenIds, setHiddenIds] = useState<Set<string>>(new Set())

  const queryClient = useQueryClient()

  const { data: exceptions = [], isLoading, error } = useQuery<ControlException[]>({
    queryKey: ['secvitals', 'exceptions'],
    queryFn: () => apiFetch<ControlException[]>('/secvitals/exceptions'),
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
      void queryClient.invalidateQueries({ queryKey: ['secvitals', 'exceptions'] })
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
        title="Ausnahmegenehmigungen"
        description="Formale Ausnahmen und Waivers für Compliance-Kontrollen"
      />

      {isLoading && (
        <div className="flex items-center justify-center py-16 text-slate-400">Lade Ausnahmen…</div>
      )}
      {error && (
        <div className="text-red-400 text-sm">Fehler beim Laden der Ausnahmen.</div>
      )}

      {!isLoading && !error && visibleExceptions.length === 0 && (
        <EmptyState
          icon={ShieldAlert}
          title="Keine Ausnahmen"
          description="Ausnahmen werden direkt am Control erstellt, wenn eine Anforderung formal nicht erfüllbar ist."
          action={
            <Button size="sm" variant="outline" onClick={() => navigate('/secvitals/frameworks')}>
              Controls anzeigen
            </Button>
          }
        />
      )}

      {active.length > 0 && (
        <div className="space-y-3">
          <h2 className="text-sm font-semibold text-slate-300 uppercase tracking-wide">Aktive Ausnahmen ({active.length})</h2>
          {active.map(e => (
            <ExceptionCard
              key={e.id}
              exception={e}
              isAdmin={isAdmin}
              onNavigate={() => navigate(`/secvitals/controls/${e.control_id}`)}
              onDelete={() => setConfirmDelete(e)}
            />
          ))}
        </div>
      )}

      {inactive.length > 0 && (
        <div className="space-y-3">
          <h2 className="text-sm font-semibold text-slate-400 uppercase tracking-wide">Inaktive Ausnahmen ({inactive.length})</h2>
          {inactive.map(e => (
            <ExceptionCard
              key={e.id}
              exception={e}
              isAdmin={isAdmin}
              onNavigate={() => navigate(`/secvitals/controls/${e.control_id}`)}
              onDelete={() => setConfirmDelete(e)}
            />
          ))}
        </div>
      )}

      {/* Delete confirm dialog — kept intentionally: exceptions are important compliance records */}
      <Dialog open={!!confirmDelete} onOpenChange={() => setConfirmDelete(null)}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>Ausnahme löschen</DialogTitle>
          </DialogHeader>
          <p className="text-sm text-slate-400">
            Soll die Ausnahme „{confirmDelete?.title}" gelöscht werden? Die Aktion kann innerhalb von 5 Sekunden rückgängig gemacht werden.
          </p>
          <DialogFooter>
            <Button variant="ghost" onClick={() => setConfirmDelete(null)}>Abbrechen</Button>
            <Button variant="destructive" onClick={handleConfirmDelete}>
              Löschen
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
  return (
    <Card className="bg-slate-800/50 border-slate-700">
      <CardHeader className="pb-2">
        <div className="flex items-start justify-between gap-3">
          <div className="flex items-center gap-2">
            {statusIcon(e.status)}
            <CardTitle className="text-sm font-medium text-slate-200">{e.title}</CardTitle>
            {statusBadge(e.status)}
          </div>
          <div className="flex items-center gap-2">
            <Button
              variant="ghost"
              size="sm"
              className="text-slate-400 hover:text-slate-200 h-7 px-2"
              onClick={onNavigate}
            >
              <ExternalLink className="w-3.5 h-3.5 mr-1" />
              Kontrolle
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
        <p><span className="text-slate-300">Begründung:</span> {e.reason}</p>
        <p><span className="text-slate-300">Akzeptiertes Risiko:</span> {e.risk_accepted}</p>
        {e.approved_by && <p><span className="text-slate-300">Genehmigt von:</span> {e.approved_by}</p>}
        {e.expires_at && (
          <p><span className="text-slate-300">Läuft ab:</span> {new Date(e.expires_at).toLocaleDateString('de-DE')}</p>
        )}
        <p className="text-slate-500">Erstellt: {new Date(e.created_at).toLocaleDateString('de-DE')}</p>
      </CardContent>
    </Card>
  )
}
