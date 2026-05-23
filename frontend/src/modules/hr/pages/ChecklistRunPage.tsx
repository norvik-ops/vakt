import { useParams, Link } from 'react-router-dom'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { CheckSquare, Square, ChevronRight, AlertCircle } from 'lucide-react'
import { apiFetch } from '../../../api/client'
import { Button } from '../../../components/ui/button'
import { Skeleton } from '../../../components/ui/skeleton'
import type { ChecklistRun, Checklist, Employee } from '../types'
import { useFormatDate } from '../../../shared/hooks/useFormatDate'

export default function ChecklistRunPage() {
  const { id } = useParams<{ id: string }>()
  const queryClient = useQueryClient()
  const { formatDate } = useFormatDate()

  const {
    data: run,
    isLoading: runLoading,
    isError: runError,
  } = useQuery<ChecklistRun>({
    queryKey: ['hr', 'checklist-runs', id],
    queryFn: () => apiFetch<ChecklistRun>(`/hr/checklist-runs/${id ?? ''}`),
    enabled: !!id,
    staleTime: 30 * 1000,
  })

  const {
    data: checklist,
    isLoading: checklistLoading,
  } = useQuery<Checklist>({
    queryKey: ['hr', 'checklists', run?.checklist_id],
    queryFn: () => apiFetch<Checklist>(`/hr/checklists/${run?.checklist_id ?? ''}`),
    enabled: !!run?.checklist_id,
    staleTime: 5 * 60 * 1000,
  })

  const {
    data: employee,
    isLoading: employeeLoading,
  } = useQuery<Employee>({
    queryKey: ['hr', 'employees', run?.employee_id],
    queryFn: () => apiFetch<Employee>(`/hr/employees/${run?.employee_id ?? ''}`),
    enabled: !!run?.employee_id,
    staleTime: 5 * 60 * 1000,
  })

  const updateRun = useMutation<ChecklistRun, Error, { completed_items: string[]; status: ChecklistRun['status'] }>({
    mutationFn: ({ completed_items, status }) =>
      apiFetch<ChecklistRun>(`/hr/checklist-runs/${id ?? ''}`, {
        method: 'PUT',
        body: JSON.stringify({ completed_items, status }),
      }),
    onSuccess: (updated) => {
      queryClient.setQueryData(['hr', 'checklist-runs', id], updated)
      if (run?.employee_id) {
        void queryClient.invalidateQueries({ queryKey: ['hr', 'checklist-runs', run.employee_id] })
      }
    },
  })

  function toggleItem(itemId: string) {
    if (!run) return
    const already = run.completed_items.includes(itemId)
    const next = already
      ? run.completed_items.filter((x) => x !== itemId)
      : [...run.completed_items, itemId]
    updateRun.mutate({ completed_items: next, status: 'in_progress' })
  }

  function completeRun() {
    if (!run) return
    updateRun.mutate({ completed_items: run.completed_items, status: 'completed' })
  }

  const isLoading = runLoading || checklistLoading || employeeLoading

  if (runError) {
    return (
      <div className="p-6 flex items-center gap-2 text-red-400 text-sm">
        <AlertCircle className="w-4 h-4" />
        Checkliste konnte nicht geladen werden.
      </div>
    )
  }

  const totalItems = checklist?.items.length ?? 0
  const completedCount = run?.completed_items.length ?? 0
  const progressPct = totalItems > 0 ? Math.round((completedCount / totalItems) * 100) : 0

  const allRequiredChecked = checklist?.items
    .filter((item) => item.required)
    .every((item) => run?.completed_items.includes(item.id)) ?? false

  const employeeName = employee
    ? `${employee.first_name} ${employee.last_name}`
    : '…'

  return (
    <div className="p-6 max-w-2xl mx-auto space-y-6">
      {/* Breadcrumb */}
      <nav className="flex items-center gap-1 text-[11px] text-secondary" aria-label="Breadcrumb">
        <Link to="/hr/employees" className="hover:text-primary transition-colors">HR</Link>
        <ChevronRight className="w-3 h-3" aria-hidden="true" />
        <Link to="/hr/employees" className="hover:text-primary transition-colors">Mitarbeiter</Link>
        <ChevronRight className="w-3 h-3" aria-hidden="true" />
        {isLoading ? (
          <Skeleton className="h-3 w-24" />
        ) : (
          <span className="text-secondary">{employeeName}</span>
        )}
        <ChevronRight className="w-3 h-3" aria-hidden="true" />
        {isLoading ? (
          <Skeleton className="h-3 w-24" />
        ) : (
          <span className="text-primary font-medium">{checklist?.name ?? '…'}</span>
        )}
      </nav>

      {/* Header */}
      <div>
        {isLoading ? (
          <Skeleton className="h-7 w-64 mb-1" />
        ) : (
          <h1 className="text-[20px] font-bold text-primary">
            Checkliste ausführen: {checklist?.name}
          </h1>
        )}
        {!isLoading && employee && (
          <p className="text-[13px] text-secondary mt-0.5">
            Mitarbeiter: {employeeName}
          </p>
        )}
      </div>

      {/* Completed state */}
      {run?.status === 'completed' && (
        <div className="flex items-center gap-3 rounded-lg border border-green-500/30 bg-green-500/10 px-4 py-3">
          <CheckSquare className="w-5 h-5 text-green-400 shrink-0" aria-hidden="true" />
          <div>
            <p className="text-[13px] font-semibold text-green-400">Checkliste abgeschlossen</p>
            {run.completed_at && (
              <p className="text-[11px] text-secondary">am {formatDate(run.completed_at, { day: '2-digit', month: '2-digit', year: 'numeric' })}</p>
            )}
          </div>
        </div>
      )}

      {/* Progress bar */}
      {!isLoading && totalItems > 0 && (
        <div>
          <div className="flex items-center justify-between mb-1.5 text-[12px]">
            <span className="text-secondary">{completedCount} von {totalItems} Schritten erledigt</span>
            <span className="font-semibold text-primary">{progressPct}%</span>
          </div>
          <div
            className="h-2 rounded-full bg-border overflow-hidden"
            role="progressbar"
            aria-valuenow={progressPct}
            aria-valuemin={0}
            aria-valuemax={100}
            aria-label={`Fortschritt: ${String(progressPct)}%`}
          >
            <div
              className={`h-full rounded-full transition-all duration-300 ${progressPct === 100 ? 'bg-green-500' : 'bg-brand'}`}
              style={{ width: `${String(progressPct)}%` }}
            />
          </div>
        </div>
      )}

      {/* Checklist items */}
      {isLoading ? (
        <div className="space-y-2">
          {Array.from({ length: 4 }).map((_, i) => (
            <Skeleton key={i} className="h-12 w-full rounded-lg" />
          ))}
        </div>
      ) : (
        <div className="rounded-lg border border-border bg-surface overflow-hidden">
          {checklist?.items.map((item, idx) => {
            const checked = run?.completed_items.includes(item.id) ?? false
            const disabled = run?.status === 'completed' || updateRun.isPending
            return (
              <button
                key={item.id}
                disabled={disabled}
                onClick={() => { toggleItem(item.id) }}
                className={`w-full flex items-center gap-3 px-4 py-3 text-left transition-colors border-b border-border last:border-0
                  ${disabled ? 'cursor-default' : 'hover:bg-border/30 cursor-pointer'}
                  ${checked ? 'opacity-70' : ''}
                `}
                aria-checked={checked}
                role="checkbox"
              >
                <span className="shrink-0" aria-hidden="true">
                  {checked ? (
                    <CheckSquare className="w-4 h-4 text-green-400" />
                  ) : (
                    <Square className="w-4 h-4 text-secondary" />
                  )}
                </span>
                <span className="flex-1 text-[13px] text-primary">
                  <span className="text-[10px] text-secondary mr-2 font-medium">#{idx + 1}</span>
                  {item.label}
                  {item.required && (
                    <span className="text-red-400 ml-1" aria-label="Pflichtfeld">*</span>
                  )}
                </span>
              </button>
            )
          })}
        </div>
      )}

      {/* Required hint */}
      {!isLoading && checklist?.items.some((i) => i.required) && run?.status !== 'completed' && (
        <p className="text-[11px] text-secondary">
          <span className="text-red-400">*</span> Pflichtfelder müssen abgehakt werden, bevor die Checkliste abgeschlossen werden kann.
        </p>
      )}

      {/* Complete button */}
      {run?.status !== 'completed' && !isLoading && (
        <div className="flex justify-end">
          <Button
            onClick={completeRun}
            disabled={!allRequiredChecked || updateRun.isPending}
          >
            {updateRun.isPending ? 'Wird gespeichert…' : 'Abschließen'}
          </Button>
        </div>
      )}
    </div>
  )
}
