import { useNavigate } from 'react-router-dom'
import { ListTodo, TriangleAlert, Flame, Shield, Zap } from 'lucide-react'
import { useQuery } from '@tanstack/react-query'
import { apiFetch } from '../api/client'
import { useRisks } from '../modules/vaktcomply/hooks/useRisks'
import { useIncidents } from '../modules/vaktcomply/hooks/useIncidents'
import { useAuthStore } from '../shared/stores/auth'
import { Skeleton } from '../components/ui/skeleton'
import { Button } from '../components/ui/button'
import type { Risk, Incident, Control } from '../modules/vaktcomply/types'

interface MyTask {
  id: string
  title: string
  type: 'control' | 'risk'
  status: string
  framework_id?: string
}

function useQuickWinsControls() {
  return useQuery<Control[]>({
    queryKey: ['vaktcomply', 'controls', 'quick-wins'],
    queryFn: () => apiFetch<Control[]>('/vaktcomply/controls?status=missing&limit=20'),
    staleTime: 5 * 60 * 1000,
  })
}

export function TodayWidget() {
  const navigate = useNavigate()
  const today = new Date().toISOString().slice(0, 10)
  const { data: risks } = useRisks(1, 100)
  const { data: incidents } = useIncidents(1, 100)

  const todayRisks = (risks ?? []).filter((r: Risk) => {
    if (r.status !== 'open') return false
    const due = r.treatment_due_date
    return due ? due.slice(0, 10) <= today : false
  })
  const todayIncidents = (incidents ?? []).filter((i: Incident) => {
    if (i.status === 'resolved' || i.status === 'closed') return false
    return i.created_at.slice(0, 10) === today || i.status === 'open'
  })
  const total = todayRisks.length + todayIncidents.length

  if (total === 0) {
    return (
      <section className="rounded-xl border border-border bg-surface p-4">
        <div className="flex items-center gap-2 mb-3">
          <ListTodo className="w-4 h-4 text-secondary" aria-hidden="true" />
          <h2 className="text-[13px] font-semibold text-primary">Heute zu tun</h2>
        </div>
        <p className="text-[12px] text-secondary">Nichts Dringendes heute.</p>
      </section>
    )
  }

  return (
    <section className="rounded-lg border border-border bg-surface p-4">
      <div className="flex items-center gap-2 mb-3">
        <ListTodo className="w-4 h-4 text-brand" aria-hidden="true" />
        <h2 className="text-[13px] font-semibold text-primary">Heute zu tun</h2>
        <span className="ml-auto text-[11px] font-bold text-brand">{total}</span>
      </div>
      <div className="space-y-3">
        {todayRisks.length > 0 && (
          <div>
            <p className="text-[10px] font-semibold text-secondary uppercase tracking-wider mb-1.5">Risiken fällig</p>
            <ol className="space-y-1.5">
              {todayRisks.slice(0, 5).map((r) => {
                const isOverdue = (r.treatment_due_date ?? '') < today
                return (
                  <li key={r.id}>
                    <button
                      className="w-full flex items-center gap-2 text-left rounded-md px-2 py-1.5 hover:bg-border/50 transition-colors group"
                      onClick={() => { navigate(`/vaktcomply/risks/${r.id}`) }}
                    >
                      <TriangleAlert className={`w-3.5 h-3.5 shrink-0 ${isOverdue ? 'text-severity-critical' : 'text-severity-medium'}`} aria-hidden="true" />
                      <span className="text-[12px] text-primary flex-1 truncate group-hover:text-brand">{r.title}</span>
                      <span className={`text-[10px] shrink-0 ${isOverdue ? 'text-severity-critical' : 'text-secondary'}`}>
                        {isOverdue ? 'Überfällig' : 'Heute fällig'}
                      </span>
                    </button>
                  </li>
                )
              })}
            </ol>
          </div>
        )}
        {todayIncidents.length > 0 && (
          <div>
            <p className="text-[10px] font-semibold text-secondary uppercase tracking-wider mb-1.5">Vorfälle offen</p>
            <ol className="space-y-1.5">
              {todayIncidents.slice(0, 5).map((i) => (
                <li key={i.id}>
                  <button
                    className="w-full flex items-center gap-2 text-left rounded-md px-2 py-1.5 hover:bg-border/50 transition-colors group"
                    onClick={() => { navigate(`/vaktcomply/incidents/${i.id}`) }}
                  >
                    <Flame className="w-3.5 h-3.5 shrink-0 text-severity-critical" aria-hidden="true" />
                    <span className="text-[12px] text-primary flex-1 truncate group-hover:text-brand">{i.title}</span>
                    <span className="text-[10px] text-secondary shrink-0">{i.severity}</span>
                  </button>
                </li>
              ))}
            </ol>
          </div>
        )}
      </div>
    </section>
  )
}

export function MyTasksWidget() {
  const navigate = useNavigate()
  const user = useAuthStore((s) => s.user)
  const { data: tasks = [], isLoading } = useQuery<MyTask[]>({
    queryKey: ['my-tasks'],
    queryFn: () => apiFetch<MyTask[]>('/vaktcomply/my-tasks'),
    enabled: !!user,
    staleTime: 2 * 60 * 1000,
  })

  return (
    <section className="rounded-xl border border-border bg-surface p-4">
      <div className="flex items-center gap-2 mb-3">
        <ListTodo className="w-4 h-4 text-brand" aria-hidden="true" />
        <h2 className="text-[13px] font-semibold text-primary">Meine Aufgaben</h2>
        {tasks.length > 0 && (
          <span className="ml-auto text-[11px] font-bold text-brand">{tasks.length}</span>
        )}
      </div>
      {isLoading ? (
        <div className="space-y-2">
          <Skeleton className="h-7 w-full" />
          <Skeleton className="h-7 w-3/4" />
        </div>
      ) : tasks.length === 0 ? (
        <p className="text-[12px] text-secondary">Keine Aufgaben zugewiesen.</p>
      ) : (
        <ol className="space-y-1.5">
          {tasks.slice(0, 8).map((task) => (
            <li key={task.id}>
              <button
                className="w-full flex items-center gap-2 text-left rounded-md px-2 py-1.5 hover:bg-border/50 transition-colors group"
                onClick={() => {
                  if (task.type === 'control') {
                    // S121-F1 (F1-UI): the /frameworks/:id/controls/:id route does not
                    // exist — control detail is served at /vaktcomply/controls/:id.
                    navigate(`/vaktcomply/controls/${task.id}`)
                  } else if (task.type === 'risk') {
                    navigate(`/vaktcomply/risks/${task.id}`)
                  }
                }}
              >
                {task.type === 'control'
                  ? <Shield className="w-3.5 h-3.5 shrink-0 text-brand" aria-hidden="true" />
                  : <TriangleAlert className="w-3.5 h-3.5 shrink-0 text-severity-medium" aria-hidden="true" />}
                <span className="text-[12px] text-primary flex-1 truncate group-hover:text-brand">{task.title}</span>
                <span className="text-[10px] text-secondary shrink-0 capitalize">{task.status || '—'}</span>
              </button>
            </li>
          ))}
        </ol>
      )}
    </section>
  )
}

export function QuickWinsCard() {
  const navigate = useNavigate()
  const { data: controls } = useQuickWinsControls()

  const quickWins = (controls ?? [])
    .filter((c) => c.status === 'missing')
    .slice(0, 5)
    .map((c) => {
      const staleDays = c.last_reviewed_at
        ? Math.floor((Date.now() - new Date(c.last_reviewed_at).getTime()) / 86_400_000)
        : null
      const hint = staleDays !== null && staleDays > 30
        ? `Seit ${staleDays.toString()} Tagen nicht überprüft`
        : 'Noch nicht gestartet — schnell umsetzbar'
      return { control: c, hint }
    })

  if (quickWins.length === 0) return null

  return (
    <div className="bg-surface border border-border rounded-xl p-5">
      <div className="flex items-center gap-2 mb-4">
        <Zap className="w-4 h-4 text-amber-500" aria-hidden="true" />
        <h2 className="text-sm font-semibold text-primary">Quick Wins ({quickWins.length})</h2>
        <span className="text-xs text-secondary">— kleine Maßnahmen, große Wirkung</span>
      </div>
      <div className="space-y-2">
        {quickWins.map(({ control, hint }) => (
          <div key={control.id} className="flex items-center justify-between gap-3 text-sm">
            <span className="text-primary font-medium truncate min-w-0">
              {control.control_id} {control.title}
            </span>
            <span className="text-xs text-secondary shrink-0 hidden sm:block">{hint}</span>
            <Button
              size="sm"
              variant="outline"
              className="h-7 text-xs shrink-0"
              onClick={() => { navigate(`/vaktcomply/controls/${control.id}`) }}
            >
              Öffnen
            </Button>
          </div>
        ))}
      </div>
    </div>
  )
}
