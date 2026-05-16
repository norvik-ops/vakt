import { useState } from 'react'
import { useParams, useNavigate, useSearchParams } from 'react-router-dom'
import {
  ArrowLeft, Plus, Download, FileText, ChevronRight, RefreshCw, Info,
  Circle, Clock, CheckCircle2, MinusCircle, Trash2, ListChecks,
} from 'lucide-react'
import { PageHeader } from '../../../shared/components/PageHeader'
import { Button } from '../../../components/ui/button'
import { Badge } from '../../../components/ui/badge'
import { Card, CardContent, CardHeader, CardTitle } from '../../../components/ui/card'
import { Dialog, DialogContent, DialogHeader, DialogTitle, DialogFooter } from '../../../components/ui/dialog'
import { Input } from '../../../components/ui/input'
import { Label } from '../../../components/ui/label'
import { Select, SelectTrigger, SelectContent, SelectItem, SelectValue } from '../../../components/ui/select'
import { Table, TableHeader, TableBody, TableRow, TableHead, TableCell } from '../../../components/ui/table'
import { cn } from '../../../lib/utils'
import { useEvidence, useReviewEvidence } from '../hooks/useEvidence'
import {
  useAddEvidence, useUploadEvidence, useCollectEvidence,
  useExportControl, useControl, useUpdateControl,
} from '../hooks/useControls'
import { useFrameworkControls, useFramework } from '../hooks/useFrameworks'
import { useControlTasks, useCreateControlTask, useToggleControlTask, useDeleteControlTask } from '../hooks/useControlTasks'
import { MeasuresList } from '../components/MeasuresList'
import { TasksPanel } from '../components/TasksPanel'
import { CommentThread } from '../components/CommentThread'
import { EvidenceFileUpload } from '../components/EvidenceFileUpload'
import { ControlReviewPanel } from '../components/ControlReviewPanel'
import type { Evidence, Control } from '../types'
import { maturityLabel, maturityColor } from '../utils/tisax'
import { ControlMappingsBadge } from '../components/ControlMappingsBadge'

// ── Status config ────────────────────────────────────────────────────────────

type StatusChoice = 'missing' | 'in_progress' | 'implemented' | 'not_applicable'

const STATUS_CONFIG: Record<StatusChoice, { icon: React.ReactNode; label: string; className: string }> = {
  missing:        { icon: <Circle className="w-3.5 h-3.5" />,       label: 'Offen',           className: 'text-red-500' },
  in_progress:    { icon: <Clock className="w-3.5 h-3.5" />,        label: 'In Bearbeitung',  className: 'text-yellow-600' },
  implemented:    { icon: <CheckCircle2 className="w-3.5 h-3.5" />, label: 'Umgesetzt',       className: 'text-green-600' },
  not_applicable: { icon: <MinusCircle className="w-3.5 h-3.5" />,  label: 'Nicht anwendbar', className: 'text-secondary' },
}

function toStatusChoice(status: Control['status']): StatusChoice {
  if (status === 'covered' || status === 'implemented') return 'implemented'
  if (status === 'not_applicable') return 'not_applicable'
  if (status === 'partial' || status === 'in_progress') return 'in_progress'
  return 'missing'
}

// ── Evidence helpers ──────────────────────────────────────────────────────────

const evidenceStatusVariant: Record<Evidence['status'], React.ComponentProps<typeof Badge>['variant']> = {
  pending_review: 'secondary',
  approved: 'success',
  rejected: 'destructive',
  expired: 'secondary',
}

const evidenceStatusLabel: Record<Evidence['status'], string> = {
  pending_review: 'Ausstehend',
  approved: 'Genehmigt',
  rejected: 'Abgelehnt',
  expired: 'Abgelaufen',
}

const evidenceTypeLabel: Record<Evidence['type'], string> = {
  manual: 'Manuell',
  automated: 'Automatisiert',
  document: 'Dokument',
}

// ── NA dialog ────────────────────────────────────────────────────────────────

function NotApplicableDialog({
  control,
  frameworkId,
  open,
  onClose,
}: {
  control: Control
  frameworkId: string
  open: boolean
  onClose: () => void
}) {
  const [reason, setReason] = useState(control.not_applicable_reason ?? '')
  const updateControl = useUpdateControl(frameworkId)

  function handleConfirm() {
    updateControl.mutate(
      { controlId: control.id, not_applicable: true, reason, manual_status: '' },
      { onSuccess: onClose },
    )
  }

  return (
    <Dialog open={open} onOpenChange={(v) => { if (!v) onClose() }}>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>Control als „Nicht anwendbar" markieren</DialogTitle>
        </DialogHeader>
        <div className="py-3 space-y-3">
          <p className="text-sm text-secondary">
            <span className="font-mono text-xs bg-surface2 px-1.5 py-0.5 rounded">{control.control_id}</span>
            {' '}{control.title}
          </p>
          <div className="space-y-1.5">
            <Label htmlFor="na-reason">
              Begründung <span className="text-secondary">(für Auditor sichtbar)</span>
            </Label>
            <textarea
              id="na-reason"
              rows={3}
              className="w-full rounded-md border border-border bg-surface2 text-primary px-3 py-2 text-sm focus:outline-none focus:ring-1 focus:ring-brand"
              value={reason}
              onChange={(e) => setReason(e.target.value)}
              placeholder="z.B. Trifft auf unsere Organisation nicht zu."
            />
          </div>
        </div>
        <DialogFooter>
          <Button variant="outline" onClick={onClose}>Abbrechen</Button>
          <Button onClick={handleConfirm} disabled={updateControl.isPending}>
            {updateControl.isPending ? 'Wird gespeichert…' : 'Bestätigen'}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}

// ── Page ─────────────────────────────────────────────────────────────────────

export default function ControlDetailPage() {
  const { id } = useParams<{ id: string }>()
  const navigate = useNavigate()
  const [searchParams] = useSearchParams()
  const controlId = id ?? ''
  const frameworkId = searchParams.get('frameworkId') ?? ''

  const { data: control, isLoading: controlLoading } = useControl(controlId)
  const { data: framework } = useFramework(frameworkId)
  const { data: allControls } = useFrameworkControls(frameworkId)
  const { data: evidence, isLoading: evidenceLoading } = useEvidence(controlId)
  const { data: tasks } = useControlTasks(controlId)
  const updateControl = useUpdateControl(frameworkId)
  const addEvidence = useAddEvidence(controlId)
  const uploadEvidence = useUploadEvidence(controlId)
  const collectEvidence = useCollectEvidence(controlId)
  const exportControl = useExportControl(controlId)
  const createTask = useCreateControlTask(controlId)
  const toggleTask = useToggleControlTask(controlId)
  const deleteTask = useDeleteControlTask(controlId)

  const subControls = allControls?.filter(
    (c) => c.control_id.startsWith((control?.control_id ?? '') + '.') && c.id !== controlId,
  ) ?? []

  // Checklist
  const [newTaskText, setNewTaskText] = useState('')

  function handleAddTask(e: React.FormEvent) {
    e.preventDefault()
    if (!newTaskText.trim()) return
    createTask.mutate({ text: newTaskText.trim() }, { onSuccess: () => setNewTaskText('') })
  }

  // Evidence form
  const [addOpen, setAddOpen] = useState(false)
  const [title, setTitle] = useState('')
  const [type, setType] = useState<Evidence['type']>('manual')
  const [notes, setNotes] = useState('')
  const [file, setFile] = useState<File | null>(null)

  // Review dialog
  const [reviewOpen, setReviewOpen] = useState(false)
  const [reviewingId, setReviewingId] = useState('')
  const [reviewStatus, setReviewStatus] = useState<'approved' | 'rejected'>('approved')
  const [reviewNotes, setReviewNotes] = useState('')
  const review = useReviewEvidence(reviewingId, controlId)

  // NA dialog
  const [naOpen, setNaOpen] = useState(false)

  function handleStatusChange(value: string) {
    if (!control) return
    if (value === 'not_applicable') { setNaOpen(true); return }
    updateControl.mutate({
      controlId: control.id,
      not_applicable: false,
      reason: '',
      manual_status: value === 'missing' ? '' : value as '' | 'in_progress' | 'implemented',
    })
  }

  function handleMaturityChange(score: number) {
    if (!control) return
    updateControl.mutate({
      controlId: control.id,
      not_applicable: control.not_applicable,
      reason: control.not_applicable_reason ?? '',
      manual_status: (['in_progress', 'implemented'] as const).includes(control.status as 'in_progress' | 'implemented') ? control.status as 'in_progress' | 'implemented' : '',
      maturity_score: score,
    })
  }

  const isTISAX = framework?.name === 'TISAX'

  function resetAddForm() {
    setAddOpen(false)
    setTitle('')
    setType('manual')
    setNotes('')
    setFile(null)
  }

  function handleAdd(e: React.FormEvent) {
    e.preventDefault()
    if (type === 'document' && file) {
      const fd = new FormData()
      fd.append('file', file)
      fd.append('title', title)
      if (notes) fd.append('notes', notes)
      uploadEvidence.mutate(fd, { onSuccess: resetAddForm })
    } else {
      addEvidence.mutate(
        { title, type, notes: notes || undefined },
        { onSuccess: resetAddForm },
      )
    }
  }

  function openReview(evidenceId: string) {
    setReviewingId(evidenceId)
    setReviewStatus('approved')
    setReviewNotes('')
    setReviewOpen(true)
  }

  function handleReview(e: React.FormEvent) {
    e.preventDefault()
    review.mutate(
      { status: reviewStatus, notes: reviewNotes || undefined },
      { onSuccess: () => setReviewOpen(false) },
    )
  }

  const backTo = frameworkId
    ? `/secvitals/frameworks/${frameworkId}`
    : '/secvitals/frameworks'

  const currentStatus = control ? toStatusChoice(control.status) : 'missing'

  return (
    <div className="flex flex-col h-full">
      <PageHeader
        title={controlLoading ? '…' : (control?.title ?? 'Control')}
        description={control ? `${control.control_id} · ${control.domain}` : ''}
        actions={
          <div className="flex items-center gap-2">
            <Button variant="outline" size="sm" onClick={exportControl}>
              <Download className="w-4 h-4 mr-1" />
              Export
            </Button>
            <Button size="sm" onClick={() => setAddOpen(true)}>
              <Plus className="w-4 h-4 mr-1" />
              Nachweis hinzufügen
            </Button>
            <Button variant="outline" size="sm" onClick={() => navigate(backTo)}>
              <ArrowLeft className="w-4 h-4 mr-1" />
              Zurück
            </Button>
          </div>
        }
      />

      <div className="flex-1 p-6 space-y-6">
        {/* Control info card */}
        {controlLoading ? (
          <div className="flex justify-center py-8">
            <div className="w-5 h-5 border-2 border-brand border-t-transparent rounded-full animate-spin" />
          </div>
        ) : control ? (
          <div className="p-5 bg-surface border border-border rounded-lg space-y-4">
            <div className="flex items-start justify-between gap-4 flex-wrap">
              <div className="flex items-center gap-3 flex-wrap">
                <Badge variant="secondary" className="font-mono">{control.control_id}</Badge>
                {control.domain && (
                  <span className="text-xs text-secondary">{control.domain}</span>
                )}
                {control.iso27001_mapping && (
                  <span
                    data-testid="iso27001-mapping-badge"
                    className="inline-flex items-center gap-1.5 text-xs text-orange-600 bg-orange-500/10 border border-orange-500/20 rounded px-2 py-0.5"
                  >
                    ISO 27001: {control.iso27001_mapping}
                  </span>
                )}
              </div>
              {/* Status selector: maturity radio buttons for TISAX, status toggle for others */}
              {isTISAX ? (
                <div data-testid="maturity-radio-group" className="flex items-center gap-2 flex-wrap">
                  {([0, 1, 2, 3] as const).map((score) => (
                    <button
                      key={score}
                      type="button"
                      data-testid={`maturity-radio-${score}`}
                      disabled={updateControl.isPending}
                      onClick={() => handleMaturityChange(score)}
                      className={cn(
                        'px-2.5 py-1 text-xs rounded-md border transition-colors',
                        (control?.maturity_score ?? 0) === score
                          ? `${maturityColor(score)} border-current bg-current/10 font-medium`
                          : 'text-secondary border-border hover:border-brand',
                      )}
                    >
                      {score} – {maturityLabel(score)}
                    </button>
                  ))}
                </div>
              ) : (
                <Select
                  value={currentStatus}
                  onValueChange={handleStatusChange}
                  disabled={updateControl.isPending}
                >
                  <SelectTrigger className="h-8 text-xs w-48 gap-1.5">
                    <span className={cn('flex items-center gap-1.5', STATUS_CONFIG[currentStatus].className)}>
                      {STATUS_CONFIG[currentStatus].icon}
                      {STATUS_CONFIG[currentStatus].label}
                    </span>
                  </SelectTrigger>
                  <SelectContent>
                    {(Object.entries(STATUS_CONFIG) as [StatusChoice, typeof STATUS_CONFIG[StatusChoice]][]).map(([val, cfg]) => (
                      <SelectItem key={val} value={val} className="text-xs">
                        <span className={cn('flex items-center gap-2', cfg.className)}>
                          {cfg.icon}
                          {cfg.label}
                        </span>
                      </SelectItem>
                    ))}
                  </SelectContent>
                </Select>
              )}
            </div>

            {control.not_applicable && control.not_applicable_reason && (
              <p className="text-xs text-secondary italic border-l-2 border-border pl-3">
                Begründung: {control.not_applicable_reason}
              </p>
            )}

            {control.description ? (
              <p className="text-sm text-secondary leading-relaxed">{control.description}</p>
            ) : (
              <p className="text-sm text-secondary italic">Keine Beschreibung vorhanden.</p>
            )}

            <ControlMappingsBadge controlId={control.id} />
          </div>
        ) : null}

        {/* Sub-controls */}
        {subControls.length > 0 && (
          <Card>
            <CardHeader>
              <CardTitle className="text-sm">Unterkontrollen</CardTitle>
            </CardHeader>
            <CardContent className="p-0">
              <Table>
                <TableHeader>
                  <TableRow>
                    <TableHead>Control ID</TableHead>
                    <TableHead>Titel</TableHead>
                    <TableHead>Status</TableHead>
                    <TableHead></TableHead>
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {subControls.map((sub) => {
                    const subStatus = toStatusChoice(sub.status)
                    return (
                      <TableRow
                        key={sub.id}
                        className="cursor-pointer hover:bg-surface2"
                        onClick={() =>
                          navigate(
                            `/secvitals/controls/${sub.id}${frameworkId ? `?frameworkId=${frameworkId}` : ''}`,
                          )
                        }
                      >
                        <TableCell className="font-mono text-xs">{sub.control_id}</TableCell>
                        <TableCell className="font-medium">{sub.title}</TableCell>
                        <TableCell>
                          <span className={cn('flex items-center gap-1.5 text-xs', STATUS_CONFIG[subStatus].className)}>
                            {STATUS_CONFIG[subStatus].icon}
                            {STATUS_CONFIG[subStatus].label}
                          </span>
                        </TableCell>
                        <TableCell className="text-secondary">
                          <ChevronRight className="w-4 h-4" />
                        </TableCell>
                      </TableRow>
                    )
                  })}
                </TableBody>
              </Table>
            </CardContent>
          </Card>
        )}

        {/* Implementation checklist */}
        <Card>
          <CardHeader>
            <CardTitle className="text-sm flex items-center gap-2">
              <ListChecks className="w-4 h-4" />
              Umsetzungsschritte
            </CardTitle>
          </CardHeader>
          <CardContent className="space-y-3">
            {/* Task list */}
            {tasks && tasks.length > 0 && (
              <ul className="space-y-2">
                {tasks.map((task) => (
                  <li key={task.id} className="flex items-start gap-3 group">
                    <button
                      type="button"
                      className={cn(
                        'mt-0.5 w-4 h-4 rounded border flex-shrink-0 flex items-center justify-center transition-colors',
                        task.completed
                          ? 'bg-green-500 border-green-500 text-white'
                          : 'border-border hover:border-green-400',
                      )}
                      onClick={() => toggleTask.mutate({ taskId: task.id, completed: !task.completed })}
                      title={task.completed ? 'Als offen markieren' : 'Als erledigt markieren'}
                    >
                      {task.completed && <CheckCircle2 className="w-3 h-3" />}
                    </button>
                    <span className={cn('text-sm flex-1', task.completed && 'line-through text-muted-foreground')}>
                      {task.text}
                    </span>
                    <button
                      type="button"
                      className="opacity-0 group-hover:opacity-100 text-muted-foreground hover:text-destructive transition-opacity"
                      onClick={() => deleteTask.mutate(task.id)}
                      title="Schritt löschen"
                    >
                      <Trash2 className="w-3.5 h-3.5" />
                    </button>
                  </li>
                ))}
              </ul>
            )}
            {/* Add task form */}
            <form onSubmit={handleAddTask} className="flex gap-2">
              <Input
                placeholder="Umsetzungsschritt hinzufügen …"
                value={newTaskText}
                onChange={(e) => setNewTaskText(e.target.value)}
                className="flex-1 h-8 text-sm"
              />
              <Button type="submit" size="sm" disabled={!newTaskText.trim() || createTask.isPending} className="h-8">
                <Plus className="w-3.5 h-3.5" />
              </Button>
            </form>
            {(!tasks || tasks.length === 0) && !newTaskText && (
              <p className="text-xs text-muted-foreground">
                Füge konkrete Umsetzungsschritte hinzu und hake sie ab, wenn sie abgeschlossen sind.
              </p>
            )}
          </CardContent>
        </Card>

        {/* Measures catalogue */}
        <MeasuresList controlId={controlId} />

        {/* Control Review Cycle */}
        {control && (
          <ControlReviewPanel
            controlId={controlId}
            lastReviewedAt={control.last_reviewed_at}
            nextReviewDue={control.next_review_due}
            isOverdue={control.is_review_overdue}
            reviewIntervalDays={control.review_interval_days}
            lastReviewedBy={control.last_reviewed_by}
          />
        )}

        {/* Evidence */}
        <Card>
          <CardHeader>
            <CardTitle className="text-sm">Nachweise</CardTitle>
          </CardHeader>
          <CardContent className="p-0">
            {evidenceLoading ? (
              <div className="flex justify-center py-8">
                <div className="w-5 h-5 border-2 border-brand border-t-transparent rounded-full animate-spin" />
              </div>
            ) : !evidence || evidence.length === 0 ? (
              <div className="flex flex-col items-center py-12 text-center">
                <FileText className="w-10 h-10 text-gray-300 mb-3" />
                <p className="text-sm font-medium text-secondary">Noch keine Nachweise</p>
                <p className="text-xs text-secondary mt-1">Füge Nachweise hinzu, um die Compliance zu belegen.</p>
              </div>
            ) : (
              <Table>
                <TableHeader>
                  <TableRow>
                    <TableHead>Titel</TableHead>
                    <TableHead>Typ</TableHead>
                    <TableHead>Status</TableHead>
                    <TableHead>Hinzugefügt</TableHead>
                    <TableHead></TableHead>
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {evidence.map((ev) => (
                    <TableRow key={ev.id}>
                      <TableCell className="font-medium">{ev.title}</TableCell>
                      <TableCell className="text-secondary">{evidenceTypeLabel[ev.type]}</TableCell>
                      <TableCell>
                        <Badge variant={evidenceStatusVariant[ev.status]}>
                          {evidenceStatusLabel[ev.status]}
                        </Badge>
                      </TableCell>
                      <TableCell className="text-sm text-secondary">
                        {new Date(ev.created_at).toLocaleDateString('de-DE')}
                      </TableCell>
                      <TableCell>
                        {ev.status === 'pending_review' && (
                          <Button
                            variant="outline"
                            size="sm"
                            onClick={() => openReview(ev.id)}
                          >
                            Prüfen
                          </Button>
                        )}
                      </TableCell>
                    </TableRow>
                  ))}
                </TableBody>
              </Table>
            )}
          </CardContent>
        </Card>

        {/* Evidence File Attachments */}
        <Card>
          <CardHeader>
            <CardTitle className="text-sm">Anhänge</CardTitle>
          </CardHeader>
          <CardContent>
            <EvidenceFileUpload controlId={controlId} />
          </CardContent>
        </Card>

        {/* Collaborative Tasks */}
        <TasksPanel entityType="control" entityId={controlId} />

        {/* Comments */}
        <CommentThread entityType="control" entityId={controlId} />
      </div>

      {/* NA Dialog */}
      {control && (
        <NotApplicableDialog
          control={control}
          frameworkId={frameworkId}
          open={naOpen}
          onClose={() => setNaOpen(false)}
        />
      )}

      {/* Add Evidence Dialog */}
      <Dialog open={addOpen} onOpenChange={setAddOpen}>
        <DialogContent>
          <DialogHeader><DialogTitle>Nachweis hinzufügen</DialogTitle></DialogHeader>
          <form onSubmit={(e) => { void handleAdd(e) }}>
            <div className="py-4 space-y-4">
              <div className="space-y-1.5">
                <Label htmlFor="ev-title">Titel</Label>
                <Input
                  id="ev-title"
                  value={title}
                  onChange={(e) => setTitle(e.target.value)}
                  placeholder="Richtliniendokument, Screenshot, etc."
                  required
                />
              </div>
              <div className="space-y-1.5">
                <Label>Typ</Label>
                <Select value={type} onValueChange={(v) => { setType(v as Evidence['type']); setFile(null) }}>
                  <SelectTrigger>
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent>
                    <SelectItem value="manual">Manuell</SelectItem>
                    <SelectItem value="document">Dokument (Datei hochladen)</SelectItem>
                    <SelectItem value="automated">Automatisiert</SelectItem>
                  </SelectContent>
                </Select>
              </div>

              {type === 'document' && (
                <div className="space-y-1.5">
                  <Label htmlFor="ev-file">Datei</Label>
                  <input
                    id="ev-file"
                    type="file"
                    className="w-full text-sm text-primary file:mr-3 file:py-1.5 file:px-3 file:rounded-md file:border file:border-border file:bg-surface2 file:text-xs file:font-medium file:text-primary hover:file:bg-surface cursor-pointer"
                    onChange={(e) => setFile(e.target.files?.[0] ?? null)}
                  />
                  {file && <p className="text-xs text-secondary">{file.name} ({(file.size / 1024).toFixed(1)} KB)</p>}
                </div>
              )}

              {type === 'automated' && (
                <div className="flex items-start gap-2 p-3 bg-surface2 rounded-lg text-xs text-secondary">
                  <Info className="w-3.5 h-3.5 mt-0.5 shrink-0 text-brand" />
                  <div>
                    <p className="font-medium text-primary mb-1">Woher kommen automatisierte Nachweise?</p>
                    <p>Scanner aus SecPulse (Trivy, Nuclei, OpenVAS) und Cloud-Integrationen (GitHub, AWS, Azure, AD) können automatisch Nachweise sammeln.</p>
                    <Button
                      type="button"
                      size="sm"
                      variant="outline"
                      className="mt-2 h-6 text-xs"
                      onClick={() => collectEvidence.mutate()}
                      disabled={collectEvidence.isPending}
                    >
                      <RefreshCw className="w-3 h-3 mr-1" />
                      {collectEvidence.isPending ? 'Sammle…' : 'Jetzt sammeln'}
                    </Button>
                  </div>
                </div>
              )}

              <div className="space-y-1.5">
                <Label htmlFor="ev-notes">Notizen (optional)</Label>
                <textarea
                  id="ev-notes"
                  rows={3}
                  className="w-full rounded-md border border-border bg-surface2 text-primary px-3 py-2 text-sm focus:outline-none focus:ring-1 focus:ring-brand"
                  value={notes}
                  onChange={(e) => setNotes(e.target.value)}
                  placeholder="Zusätzlicher Kontext…"
                />
              </div>
            </div>
            <DialogFooter>
              <Button type="button" variant="outline" onClick={() => setAddOpen(false)}>Abbrechen</Button>
              <Button
                type="submit"
                disabled={
                  addEvidence.isPending ||
                  uploadEvidence.isPending ||
                  (type === 'document' && !file)
                }
              >
                {addEvidence.isPending || uploadEvidence.isPending ? 'Wird hinzugefügt…' : 'Nachweis hinzufügen'}
              </Button>
            </DialogFooter>
          </form>
        </DialogContent>
      </Dialog>

      {/* Review Dialog */}
      <Dialog open={reviewOpen} onOpenChange={setReviewOpen}>
        <DialogContent>
          <DialogHeader><DialogTitle>Nachweis prüfen</DialogTitle></DialogHeader>
          <form onSubmit={(e) => { void handleReview(e) }}>
            <div className="py-4 space-y-4">
              <div className="space-y-1.5">
                <Label>Entscheidung</Label>
                <Select value={reviewStatus} onValueChange={(v) => setReviewStatus(v as 'approved' | 'rejected')}>
                  <SelectTrigger>
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent>
                    <SelectItem value="approved">Genehmigen</SelectItem>
                    <SelectItem value="rejected">Ablehnen</SelectItem>
                  </SelectContent>
                </Select>
              </div>
              <div className="space-y-1.5">
                <Label htmlFor="review-notes">Notizen (optional)</Label>
                <textarea
                  id="review-notes"
                  rows={3}
                  className="w-full rounded-md border border-border bg-surface2 text-primary px-3 py-2 text-sm focus:outline-none focus:ring-1 focus:ring-brand"
                  value={reviewNotes}
                  onChange={(e) => setReviewNotes(e.target.value)}
                  placeholder="Prüfer-Notizen…"
                />
              </div>
            </div>
            <DialogFooter>
              <Button type="button" variant="outline" onClick={() => setReviewOpen(false)}>Abbrechen</Button>
              <Button type="submit" disabled={review.isPending}>
                {review.isPending ? 'Wird gespeichert…' : 'Prüfung abschließen'}
              </Button>
            </DialogFooter>
          </form>
        </DialogContent>
      </Dialog>
    </div>
  )
}
