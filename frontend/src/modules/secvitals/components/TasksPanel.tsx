import { useState } from 'react'
import { Plus, Trash2, CheckCircle2, Clock, Circle, AlertCircle } from 'lucide-react'
import { Spinner } from '../../../components/Spinner'
import { Card, CardContent, CardHeader, CardTitle } from '../../../components/ui/card'
import { Badge } from '../../../components/ui/badge'
import { Button } from '../../../components/ui/button'
import { Input } from '../../../components/ui/input'
import { Label } from '../../../components/ui/label'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '../../../components/ui/select'
import { cn } from '../../../lib/utils'
import { useTasks, useCreateTask, useUpdateTask, useDeleteTask } from '../hooks/useTasks'
import type { CollabTask, TaskPriority, TaskStatus, CreateCollabTaskInput } from '../types'
import { formatLocale } from '../../../shared/utils/locale'

// ── Priority config ───────────────────────────────────────────────────────────

const PRIORITY_CONFIG: Record<TaskPriority, { label: string; className: string }> = {
  critical: { label: 'Kritisch', className: 'bg-red-500/10 text-red-600 border-red-500/30' },
  high:     { label: 'Hoch',     className: 'bg-orange-500/10 text-orange-600 border-orange-500/30' },
  medium:   { label: 'Mittel',   className: 'bg-amber-500/10 text-amber-600 border-amber-500/30' },
  low:      { label: 'Niedrig',  className: 'bg-slate-500/10 text-slate-600 border-slate-500/30' },
}

const STATUS_CONFIG: Record<TaskStatus, { label: string; icon: React.ReactNode; className: string }> = {
  open:        { label: 'Offen',          icon: <Circle className="w-3.5 h-3.5" />,        className: 'text-secondary' },
  in_progress: { label: 'In Bearbeitung', icon: <Clock className="w-3.5 h-3.5" />,         className: 'text-yellow-600' },
  done:        { label: 'Erledigt',       icon: <CheckCircle2 className="w-3.5 h-3.5" />,  className: 'text-green-600' },
}

function nextStatus(current: TaskStatus): TaskStatus {
  if (current === 'open') return 'in_progress'
  if (current === 'in_progress') return 'done'
  return 'open'
}

function isOverdue(task: CollabTask): boolean {
  if (!task.due_date || task.status === 'done') return false
  return new Date(task.due_date) < new Date()
}

// ── Add task form ─────────────────────────────────────────────────────────────

function AddTaskForm({
  entityType,
  entityId,
  onCancel,
}: {
  entityType: string
  entityId: string
  onCancel: () => void
}) {
  const [title, setTitle] = useState('')
  const [assigneeEmail, setAssigneeEmail] = useState('')
  const [dueDate, setDueDate] = useState('')
  const [priority, setPriority] = useState<TaskPriority>('medium')
  const createTask = useCreateTask(entityType, entityId)

  function handleSubmit(e: React.FormEvent) {
    e.preventDefault()
    if (!title.trim()) return
    const input: CreateCollabTaskInput = {
      title: title.trim(),
      assignee_email: assigneeEmail.trim() || undefined,
      due_date: dueDate || undefined,
      priority,
    }
    createTask.mutate(input, {
      onSuccess: () => {
        setTitle('')
        setAssigneeEmail('')
        setDueDate('')
        setPriority('medium')
        onCancel()
      },
    })
  }

  return (
    <form
      onSubmit={handleSubmit}
      className="border border-border rounded-lg p-4 space-y-3 bg-surface2"
    >
      <div className="space-y-1.5">
        <Label htmlFor="task-title">Titel *</Label>
        <Input
          id="task-title"
          value={title}
          onChange={(e) => setTitle(e.target.value)}
          placeholder="Aufgabenbeschreibung …"
          required
          minLength={2}
          maxLength={200}
          className="h-8 text-sm"
        />
      </div>
      <div className="grid grid-cols-2 gap-3">
        <div className="space-y-1.5">
          <Label htmlFor="task-assignee">Verantwortlich (E-Mail)</Label>
          <Input
            id="task-assignee"
            type="email"
            value={assigneeEmail}
            onChange={(e) => setAssigneeEmail(e.target.value)}
            placeholder="name@company.com"
            className="h-8 text-sm"
          />
        </div>
        <div className="space-y-1.5">
          <Label htmlFor="task-due">Fälligkeitsdatum</Label>
          <Input
            id="task-due"
            type="date"
            value={dueDate}
            onChange={(e) => setDueDate(e.target.value)}
            className="h-8 text-sm"
          />
        </div>
      </div>
      <div className="space-y-1.5">
        <Label>Priorität</Label>
        <Select
          value={priority}
          onValueChange={(v) => setPriority(v as TaskPriority)}
        >
          <SelectTrigger className="h-8 text-sm w-40">
            <SelectValue />
          </SelectTrigger>
          <SelectContent>
            <SelectItem value="low">Niedrig</SelectItem>
            <SelectItem value="medium">Mittel</SelectItem>
            <SelectItem value="high">Hoch</SelectItem>
            <SelectItem value="critical">Kritisch</SelectItem>
          </SelectContent>
        </Select>
      </div>
      <div className="flex gap-2 justify-end">
        <Button type="button" variant="outline" size="sm" onClick={onCancel}>
          Abbrechen
        </Button>
        <Button type="submit" size="sm" disabled={!title.trim() || createTask.isPending}>
          {createTask.isPending ? 'Wird gespeichert…' : 'Aufgabe hinzufügen'}
        </Button>
      </div>
    </form>
  )
}

// ── Task row ──────────────────────────────────────────────────────────────────

function TaskRow({
  task,
  entityType,
  entityId,
}: {
  task: CollabTask
  entityType: string
  entityId: string
}) {
  const updateTask = useUpdateTask()
  const deleteTask = useDeleteTask(entityType, entityId)
  const overdue = isOverdue(task)
  const statusCfg = STATUS_CONFIG[task.status]
  const priorityCfg = PRIORITY_CONFIG[task.priority]

  function handleStatusToggle() {
    updateTask.mutate({
      taskId: task.id,
      entityType,
      entityId,
      status: nextStatus(task.status),
    })
  }

  return (
    <li
      className={cn(
        'flex items-start gap-3 p-3 rounded-lg border group',
        overdue ? 'border-red-400/40 bg-red-500/5' : 'border-border bg-surface',
        task.status === 'done' && 'opacity-60',
      )}
    >
      {/* Status toggle */}
      <button
        type="button"
        onClick={handleStatusToggle}
        disabled={updateTask.isPending}
        title={`Status: ${statusCfg.label} → ${STATUS_CONFIG[nextStatus(task.status)].label}`}
        className={cn('mt-0.5 shrink-0 transition-colors', statusCfg.className)}
      >
        {statusCfg.icon}
      </button>

      {/* Content */}
      <div className="flex-1 min-w-0 space-y-1">
        <p className={cn('text-sm font-medium leading-snug', task.status === 'done' && 'line-through text-muted-foreground')}>
          {task.title}
        </p>
        <div className="flex items-center gap-2 flex-wrap">
          <span
            className={cn(
              'inline-flex items-center text-xs px-1.5 py-0.5 rounded border font-medium',
              priorityCfg.className,
            )}
          >
            {priorityCfg.label}
          </span>
          {task.assignee_email && (
            <span className="text-xs text-secondary truncate">{task.assignee_email}</span>
          )}
          {task.due_date && (
            <span className={cn('text-xs', overdue ? 'text-red-500 font-medium' : 'text-secondary')}>
              {overdue && <AlertCircle className="inline w-3 h-3 mr-0.5" />}
              {new Date(task.due_date).toLocaleDateString(formatLocale())}
            </span>
          )}
        </div>
      </div>

      {/* Delete */}
      <button
        type="button"
        onClick={() => deleteTask.mutate(task.id)}
        disabled={deleteTask.isPending}
        className="opacity-0 group-hover:opacity-100 text-muted-foreground hover:text-destructive transition-opacity shrink-0"
        title="Aufgabe löschen"
      >
        <Trash2 className="w-3.5 h-3.5" />
      </button>
    </li>
  )
}

// ── Main component ────────────────────────────────────────────────────────────

export function TasksPanel({
  entityType,
  entityId,
}: {
  entityType: string
  entityId: string
}) {
  const { data: tasks, isLoading } = useTasks(entityType, entityId)
  const [addOpen, setAddOpen] = useState(false)

  const openCount = tasks?.filter((t) => t.status !== 'done').length ?? 0

  return (
    <Card>
      <CardHeader className="pb-3">
        <div className="flex items-center justify-between gap-2">
          <CardTitle className="text-sm">
            Aufgaben
            {openCount > 0 && (
              <Badge variant="secondary" className="ml-2 text-xs">
                {openCount} offen
              </Badge>
            )}
          </CardTitle>
          {!addOpen && (
            <Button size="sm" variant="outline" className="h-7 text-xs" onClick={() => setAddOpen(true)}>
              <Plus className="w-3.5 h-3.5 mr-1" />
              Aufgabe hinzufügen
            </Button>
          )}
        </div>
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
            ) : !addOpen ? (
              <p className="text-xs text-muted-foreground">
                Noch keine Aufgaben. Füge Aufgaben hinzu und weise sie Teammitgliedern zu.
              </p>
            ) : null}
            {addOpen && (
              <AddTaskForm
                entityType={entityType}
                entityId={entityId}
                onCancel={() => setAddOpen(false)}
              />
            )}
          </>
        )}
      </CardContent>
    </Card>
  )
}
