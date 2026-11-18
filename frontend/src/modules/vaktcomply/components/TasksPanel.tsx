import { useState } from 'react'
import { useCanWrite } from '../../../shared/hooks/useCanWrite'
import { useTranslation } from 'react-i18next'
import { Plus, Trash2, CheckCircle2, Circle, AlertCircle, ListChecks, ChevronDown, ChevronRight } from 'lucide-react'
import { Spinner } from '../../../components/Spinner'
import { Card, CardContent, CardHeader, CardTitle } from '../../../components/ui/card'
import { Badge } from '../../../components/ui/badge'
import { Button } from '../../../components/ui/button'
import { Input } from '../../../components/ui/input'
import { Label } from '../../../components/ui/label'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '../../../components/ui/select'
import { cn } from '../../../lib/utils'
import { useTasks, useCreateTask, useUpdateTask, useDeleteTask } from '../hooks/useTasks'
import type { CollabTask, TaskPriority, CreateCollabTaskInput } from '../types'
import { useFormatDate } from '../../../shared/hooks/useFormatDate'

// Eine Liste „Aufgaben" (UX-32 / Migration 273): Die frühere Checkliste
// „Umsetzungsschritte" und die zugewiesenen Aufgaben sind ein Datenmodell
// (ck_tasks). Jede Aufgabe ist abhakbar; Person, Frist und Priorität sind
// optional und stehen hinter „Person und Frist".

const PRIORITY_CLASS: Record<TaskPriority, string> = {
  critical: 'bg-red-500/10 text-red-600 border-red-500/30',
  high:     'bg-orange-500/10 text-orange-600 border-orange-500/30',
  medium:   'bg-amber-500/10 text-amber-600 border-amber-500/30',
  low:      'bg-slate-500/10 text-slate-600 border-slate-500/30',
}

const PRIORITY_KEY: Record<TaskPriority, string> = {
  critical: 'vaktcomply.tasksPanel.priorityCritical',
  high:     'vaktcomply.tasksPanel.priorityHigh',
  medium:   'vaktcomply.tasksPanel.priorityMedium',
  low:      'vaktcomply.tasksPanel.priorityLow',
}

function isOverdue(task: CollabTask): boolean {
  if (!task.due_date || task.status === 'done') return false
  return new Date(task.due_date) < new Date()
}

// ── Add task form ─────────────────────────────────────────────────────────────

function AddTaskForm({ entityType, entityId }: { entityType: string; entityId: string }) {
  const { t } = useTranslation()
  const [title, setTitle] = useState('')
  const [detailsOpen, setDetailsOpen] = useState(false)
  const [assigneeEmail, setAssigneeEmail] = useState('')
  const [dueDate, setDueDate] = useState('')
  const [priority, setPriority] = useState<TaskPriority>('medium')
  const createTask = useCreateTask(entityType, entityId)

  function reset() {
    setTitle('')
    setAssigneeEmail('')
    setDueDate('')
    setPriority('medium')
    setDetailsOpen(false)
  }

  function handleSubmit(e: React.FormEvent) {
    e.preventDefault()
    if (!title.trim()) return
    const input: CreateCollabTaskInput = {
      title: title.trim(),
      assignee_email: assigneeEmail.trim() || undefined,
      due_date: dueDate || undefined,
      priority,
    }
    createTask.mutate(input, { onSuccess: reset })
  }

  return (
    <form onSubmit={handleSubmit} className="space-y-2" aria-label={t('vaktcomply.tasksPanel.addButton')}>
      <div className="flex gap-2">
        <Input
          aria-label={t('vaktcomply.tasksPanel.labelTitle')}
          placeholder={t('vaktcomply.tasksPanel.addPlaceholder')}
          value={title}
          onChange={(e) => { setTitle(e.target.value); }}
          maxLength={500}
          className="flex-1 h-8 text-sm"
        />
        <Button
          type="submit"
          size="sm"
          className="h-8"
          disabled={!title.trim() || createTask.isPending}
          title={t('vaktcomply.tasksPanel.addButton')}
          aria-label={t('vaktcomply.tasksPanel.addButton')}
        >
          <Plus className="w-3.5 h-3.5" />
        </Button>
      </div>
      <button
        type="button"
        className="inline-flex items-center gap-1 text-xs text-secondary hover:text-foreground"
        aria-expanded={detailsOpen}
        onClick={() => { setDetailsOpen((v) => !v); }}
      >
        {detailsOpen ? <ChevronDown className="w-3.5 h-3.5" /> : <ChevronRight className="w-3.5 h-3.5" />}
        {t('vaktcomply.tasksPanel.details')}
      </button>
      {detailsOpen && (
        <div className="grid grid-cols-1 sm:grid-cols-3 gap-3 border border-border rounded-lg p-3 bg-surface2">
          <div className="space-y-1.5">
            <Label htmlFor="task-assignee">{t('vaktcomply.tasksPanel.labelAssignee')}</Label>
            <Input
              id="task-assignee"
              type="email"
              value={assigneeEmail}
              onChange={(e) => { setAssigneeEmail(e.target.value); }}
              placeholder="name@company.com"
              className="h-8 text-sm"
            />
          </div>
          <div className="space-y-1.5">
            <Label htmlFor="task-due">{t('vaktcomply.tasksPanel.dueDate')}</Label>
            <Input
              id="task-due"
              type="date"
              value={dueDate}
              onChange={(e) => { setDueDate(e.target.value); }}
              className="h-8 text-sm"
            />
          </div>
          <div className="space-y-1.5">
            <Label>{t('vaktcomply.tasksPanel.priority')}</Label>
            <Select value={priority} onValueChange={(v) => { setPriority(v as TaskPriority); }}>
              <SelectTrigger className="h-8 text-sm">
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value="low">{t('vaktcomply.tasksPanel.priorityLow')}</SelectItem>
                <SelectItem value="medium">{t('vaktcomply.tasksPanel.priorityMedium')}</SelectItem>
                <SelectItem value="high">{t('vaktcomply.tasksPanel.priorityHigh')}</SelectItem>
                <SelectItem value="critical">{t('vaktcomply.tasksPanel.priorityCritical')}</SelectItem>
              </SelectContent>
            </Select>
          </div>
        </div>
      )}
    </form>
  )
}

// ── Task row ──────────────────────────────────────────────────────────────────

function TaskRow({ task, entityType, entityId }: { task: CollabTask; entityType: string; entityId: string }) {
  const { t } = useTranslation()
  const canWrite = useCanWrite()
  const updateTask = useUpdateTask()
  const deleteTask = useDeleteTask(entityType, entityId)
  const { formatDate } = useFormatDate()
  const overdue = isOverdue(task)
  const done = task.status === 'done'

  const box = (
    <span
      className={cn(
        'w-4 h-4 rounded border flex-shrink-0 flex items-center justify-center transition-colors',
        done ? 'bg-green-500 border-green-500 text-white' : 'border-border',
        canWrite && !done && 'hover:border-green-400',
      )}
    >
      {done && <CheckCircle2 className="w-3 h-3" />}
    </span>
  )

  return (
    <li
      className={cn(
        'flex items-start gap-3 p-2 rounded-lg border group',
        overdue ? 'border-red-400/40 bg-red-500/5' : 'border-border bg-surface',
      )}
    >
      {canWrite ? (
        <button
          type="button"
          role="checkbox"
          aria-checked={done}
          aria-label={done ? t('vaktcomply.tasksPanel.markOpen') : t('vaktcomply.tasksPanel.markDone')}
          title={done ? t('vaktcomply.tasksPanel.markOpen') : t('vaktcomply.tasksPanel.markDone')}
          className="mt-0.5 shrink-0"
          disabled={updateTask.isPending}
          onClick={() => {
            updateTask.mutate({ taskId: task.id, entityType, entityId, status: done ? 'open' : 'done' })
          }}
        >
          {box}
        </button>
      ) : (
        // Leser: Zustand sichtbar, aber kein Knopf.
        <span className="mt-0.5 shrink-0" aria-label={done ? t('vaktcomply.tasksPanel.done') : t('vaktcomply.tasksPanel.open')}>
          {done ? box : <Circle className="w-4 h-4 text-muted-foreground" />}
        </span>
      )}

      <div className="flex-1 min-w-0 space-y-1">
        <p className={cn('text-sm leading-snug break-words', done && 'line-through text-muted-foreground')}>
          {task.title}
        </p>
        {(task.status === 'in_progress' || task.priority !== 'medium' || task.assignee_email || task.due_date) && (
          <div className="flex items-center gap-2 flex-wrap">
            {task.status === 'in_progress' && (
              <span className="text-xs text-yellow-600">{t('vaktcomply.tasksPanel.inProgress')}</span>
            )}
            {task.priority !== 'medium' && (
              <span className={cn('inline-flex items-center text-xs px-1.5 py-0.5 rounded border font-medium', PRIORITY_CLASS[task.priority])}>
                {t(PRIORITY_KEY[task.priority])}
              </span>
            )}
            {task.assignee_email && (
              <span className="text-xs text-secondary truncate">{task.assignee_email}</span>
            )}
            {task.due_date && (
              <span className={cn('text-xs', overdue ? 'text-red-500 font-medium' : 'text-secondary')}>
                {overdue && <AlertCircle className="inline w-3 h-3 mr-0.5" />}
                {formatDate(task.due_date)}
              </span>
            )}
          </div>
        )}
      </div>

      {canWrite && (
        <button
          type="button"
          onClick={() => { deleteTask.mutate(task.id); }}
          disabled={deleteTask.isPending}
          className="opacity-0 group-hover:opacity-100 focus:opacity-100 text-muted-foreground hover:text-destructive transition-opacity shrink-0"
          title={t('vaktcomply.tasksPanel.delete')}
          aria-label={t('vaktcomply.tasksPanel.delete')}
        >
          <Trash2 className="w-3.5 h-3.5" />
        </button>
      )}
    </li>
  )
}

// ── Main component ────────────────────────────────────────────────────────────

export function TasksPanel({ entityType, entityId }: { entityType: string; entityId: string }) {
  const { t } = useTranslation()
  const { data: tasks, isLoading } = useTasks(entityType, entityId)
  const canWrite = useCanWrite()

  const openCount = tasks?.filter((task) => task.status !== 'done').length ?? 0

  return (
    <Card>
      <CardHeader className="pb-3">
        <CardTitle className="text-sm flex items-center gap-2">
          <ListChecks className="w-4 h-4" />
          {t('vaktcomply.tasksPanel.panelTitle')}
          {openCount > 0 && (
            <Badge variant="secondary" className="text-xs">
              {t('vaktcomply.tasksPanel.openCount', { count: openCount })}
            </Badge>
          )}
        </CardTitle>
        <p className="text-xs text-secondary mt-1">{t('vaktcomply.tasksPanel.panelHint')}</p>
      </CardHeader>
      <CardContent className="space-y-3">
        {isLoading ? (
          <div className="flex justify-center py-6">
            <Spinner size="md" />
          </div>
        ) : (
          <>
            {tasks && tasks.length > 0 ? (
              <ul className="space-y-2">
                {tasks.map((task) => (
                  <TaskRow key={task.id} task={task} entityType={entityType} entityId={entityId} />
                ))}
              </ul>
            ) : (
              <p className="text-xs text-muted-foreground">
                {canWrite ? t('vaktcomply.tasksPanel.empty') : t('vaktcomply.tasksPanel.emptyReader')}
              </p>
            )}
            {canWrite && <AddTaskForm entityType={entityType} entityId={entityId} />}
          </>
        )}
      </CardContent>
    </Card>
  )
}
