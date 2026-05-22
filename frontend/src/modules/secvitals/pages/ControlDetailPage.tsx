import { useState, useEffect } from 'react'
import { useParams, useNavigate, useSearchParams } from 'react-router-dom'
import {
  ArrowLeft, Plus, Download, FileText, ChevronRight, RefreshCw, Info,
  Circle, Clock, CheckCircle2, MinusCircle, Trash2, ListChecks, History,
  Pencil, X, ShieldAlert, CalendarDays,
} from 'lucide-react'
import { Spinner } from '../../../components/Spinner'
import { useQuery } from '@tanstack/react-query'
import { apiFetch } from '../../../api/client'
import { PageHeader } from '../../../shared/components/PageHeader'
import { Breadcrumbs } from '../../../shared/components/Breadcrumbs'
import { trackPage } from '../../../shared/hooks/useRecentPages'
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
import { Comments } from '../../../shared/components/Comments'
import { EvidenceFileUpload } from '../components/EvidenceFileUpload'
import { ControlReviewPanel } from '../components/ControlReviewPanel'
import { EvidenceExpiryBadge } from '../components/EvidenceExpiryBadge'
import type { Evidence, Control, UpdateControlInput } from '../types'
import { maturityLabel, maturityColor } from '../utils/tisax'
import { ControlMappingsBadge } from '../components/ControlMappingsBadge'
import { useTranslation } from 'react-i18next'
import { TermTooltip } from '../../../shared/components/TermTooltip'
import { toast } from '../../../shared/hooks/useToast'
import { handleApiError } from '../../../shared/utils/errorMessages'
import { useAuthStore } from '../../../shared/stores/auth'
import { ErrorState } from '../../../shared/components/ErrorState'
import { SkeletonDetailPage } from '../../../shared/components/SkeletonLoaders'
import { useApprovalSetting, useRequestControlApproval } from '../hooks/useApprovals'
import { Textarea } from '../../../components/ui/textarea'
import {
  useControlExceptions, useCreateControlException, useDeleteControlException,
} from '../hooks/useExceptions'
import type { ControlException, CreateControlExceptionInput } from '../hooks/useExceptions'
import { useEvidenceHistory } from '../hooks/useEvidenceHistory'
import type { EvidenceHistoryEntry } from '../hooks/useEvidenceHistory'
import { formatLocale } from '../../../shared/utils/locale'

// ── Status config ────────────────────────────────────────────────────────────

type StatusChoice = 'missing' | 'in_progress' | 'implemented' | 'not_applicable'

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

// ── Change log ───────────────────────────────────────────────────────────────

interface ChangeLogEntry {
  id: string
  field: string
  old_value?: string | null
  new_value: string
  user_email?: string | null
  changed_at: string
}

function fieldLabel(field: string): string {
  const map: Record<string, string> = {
    status: 'Status',
    not_applicable: 'Nicht anwendbar',
    not_applicable_reason: 'Begründung',
    manual_status: 'Manueller Status',
    maturity_score: 'Reifegrad',
    review_interval_days: 'Prüfungsintervall',
  }
  return map[field] ?? field
}

function formatDate(isoString: string): string {
  return new Date(isoString).toLocaleString(formatLocale(), {
    year: 'numeric', month: '2-digit', day: '2-digit',
    hour: '2-digit', minute: '2-digit',
  })
}

function ChangeLogTab({ controlId }: { controlId: string }) {
  const { data: changes } = useQuery<ChangeLogEntry[]>({
    queryKey: ['control-changelog', controlId],
    queryFn: async () => {
      try {
        return await apiFetch<ChangeLogEntry[]>(`/secvitals/controls/${controlId}/changelog`)
      } catch {
        return []
      }
    },
    enabled: !!controlId,
    staleTime: 2 * 60 * 1000,
  })

  return (
    <div className="space-y-3">
      {(changes ?? []).map((entry) => (
        <div key={entry.id} className="flex gap-3 text-sm">
          <div className="w-7 h-7 rounded-full bg-surface2 flex items-center justify-center shrink-0 mt-0.5">
            <History className="w-3.5 h-3.5 text-secondary" aria-hidden="true" />
          </div>
          <div>
            <p className="text-primary">
              <span className="font-medium">{entry.user_email ?? 'System'}</span>
              {' hat '}
              <span className="text-secondary">{fieldLabel(entry.field)}</span>
              {' geändert'}
            </p>
            <p className="text-xs text-secondary mt-0.5">
              {entry.old_value != null ? (
                <><span className="line-through">{entry.old_value}</span>{' → '}</>
              ) : (
                'Gesetzt auf '
              )}
              <span className="font-medium text-primary">{entry.new_value}</span>
            </p>
            <p className="text-xs text-secondary mt-0.5">{formatDate(entry.changed_at)}</p>
          </div>
        </div>
      ))}
      {(!changes || changes.length === 0) && (
        <p className="text-sm text-secondary py-4 text-center">Noch keine Änderungen aufgezeichnet</p>
      )}
    </div>
  )
}

// ── Evidence history dialog ──────────────────────────────────────────────────

const STATUS_LABEL_MAP: Record<string, string> = {
  pending_review: 'Ausstehende Prüfung',
  approved: 'Genehmigt',
  rejected: 'Abgelehnt',
  expired: 'Abgelaufen',
}

function EvidenceHistoryDialog({
  evidenceId,
  evidenceTitle,
  open,
  onClose,
}: {
  evidenceId: string
  evidenceTitle: string
  open: boolean
  onClose: () => void
}) {
  const { data: history, isLoading } = useEvidenceHistory(evidenceId)

  function formatDateTime(iso: string): string {
    return new Date(iso).toLocaleString(formatLocale(), {
      year: 'numeric', month: '2-digit', day: '2-digit',
      hour: '2-digit', minute: '2-digit',
    })
  }

  function describeEntry(entry: EvidenceHistoryEntry): string {
    if (entry.change_note) return entry.change_note
    if (entry.status) return `Status gesetzt: ${STATUS_LABEL_MAP[entry.status] ?? entry.status}`
    return 'Aktualisiert'
  }

  return (
    <Dialog open={open} onOpenChange={(v) => { if (!v) onClose() }}>
      <DialogContent>
        <DialogHeader>
          <DialogTitle className="flex items-center gap-2">
            <History className="w-4 h-4 text-secondary" aria-hidden="true" />
            Verlauf — {evidenceTitle}
          </DialogTitle>
        </DialogHeader>
        <div className="py-2 space-y-3 max-h-96 overflow-y-auto">
          {isLoading && (
            <div className="flex justify-center py-8">
              <Spinner size="md" />
            </div>
          )}
          {!isLoading && (!history || history.length === 0) && (
            <p className="text-sm text-secondary py-4 text-center">Kein Verlauf vorhanden</p>
          )}
          {!isLoading && history && history.length > 0 && history.map((entry) => (
            <div key={entry.id} className="flex gap-3 text-sm">
              <div className="w-7 h-7 rounded-full bg-surface2 flex items-center justify-center shrink-0 mt-0.5">
                <History className="w-3.5 h-3.5 text-secondary" aria-hidden="true" />
              </div>
              <div>
                <p className="text-primary font-medium">{describeEntry(entry)}</p>
                {entry.status && (
                  <p className="text-xs text-secondary mt-0.5">
                    Status: <span className="font-medium text-primary">{STATUS_LABEL_MAP[entry.status] ?? entry.status}</span>
                  </p>
                )}
                <p className="text-xs text-secondary mt-0.5">{formatDateTime(entry.changed_at)}</p>
              </div>
            </div>
          ))}
        </div>
        <DialogFooter>
          <Button variant="outline" onClick={onClose}>Schließen</Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
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
  const { t } = useTranslation()
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
          <DialogTitle>{t('secvitals.controlDetailPage.naDialogTitle')}</DialogTitle>
        </DialogHeader>
        <div className="py-3 space-y-3">
          <p className="text-sm text-secondary">
            <span className="font-mono text-xs bg-surface2 px-1.5 py-0.5 rounded">{control.control_id}</span>
            {' '}{control.title}
          </p>
          <div className="space-y-1.5">
            <Label htmlFor="na-reason">
              {t('secvitals.controlDetailPage.naReason')} <span className="text-secondary">{t('secvitals.controlDetailPage.naReasonHint')}</span>
            </Label>
            <textarea
              id="na-reason"
              rows={3}
              className="w-full rounded-md border border-border bg-surface2 text-primary px-3 py-2 text-sm focus:outline-none focus:ring-1 focus:ring-brand"
              value={reason}
              onChange={(e) => setReason(e.target.value)}
              placeholder={t('secvitals.controlDetailPage.naPlaceholder')}
            />
          </div>
        </div>
        <DialogFooter>
          <Button variant="outline" onClick={onClose}>{t('common.cancel')}</Button>
          <Button onClick={handleConfirm} disabled={updateControl.isPending}>
            {updateControl.isPending ? t('secvitals.controlDetailPage.naSaving') : t('secvitals.controlDetailPage.naConfirm')}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}

// ── Page ─────────────────────────────────────────────────────────────────────

export default function ControlDetailPage() {
  const { t } = useTranslation()
  const { id } = useParams<{ id: string }>()
  const navigate = useNavigate()
  const [searchParams] = useSearchParams()
  const controlId = id ?? ''
  const frameworkId = searchParams.get('frameworkId') ?? ''

  const STATUS_CONFIG: Record<StatusChoice, { icon: React.ReactNode; label: string; className: string }> = {
    missing:        { icon: <Circle className="w-3.5 h-3.5" />,       label: t('secvitals.controlDetailPage.statusMissing'),        className: 'text-red-500' },
    in_progress:    { icon: <Clock className="w-3.5 h-3.5" />,        label: t('secvitals.controlDetailPage.statusInProgress'),     className: 'text-yellow-600' },
    implemented:    { icon: <CheckCircle2 className="w-3.5 h-3.5" />, label: t('secvitals.controlDetailPage.statusImplemented'),    className: 'text-green-600' },
    not_applicable: { icon: <MinusCircle className="w-3.5 h-3.5" />,  label: t('secvitals.controlDetailPage.statusNotApplicable'), className: 'text-secondary' },
  }

  const evidenceStatusLabel: Record<Evidence['status'], string> = {
    pending_review: t('secvitals.controlDetailPage.evidenceStatusPending'),
    approved: t('secvitals.controlDetailPage.evidenceStatusApproved'),
    rejected: t('secvitals.controlDetailPage.evidenceStatusRejected'),
    expired: t('secvitals.controlDetailPage.evidenceStatusExpired'),
  }

  const evidenceTypeLabel: Record<Evidence['type'], string> = {
    manual: t('secvitals.controlDetailPage.evidenceTypeManualLabel'),
    automated: t('secvitals.controlDetailPage.evidenceTypeAutomatedLabel'),
    document: t('secvitals.controlDetailPage.evidenceTypeDocumentLabel'),
  }

  const { user } = useAuthStore()
  const isAdmin = user?.roles?.includes('Admin') ?? false

  const { data: control, isLoading: controlLoading, isError: controlError, refetch: refetchControl } = useControl(controlId)
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
  const requestApproval = useRequestControlApproval(controlId)
  const { data: approvalSetting } = useApprovalSetting()
  const approvalRequired = approvalSetting?.approval_required ?? false

  // Exceptions
  const { data: exceptions } = useControlExceptions(controlId)
  const createException = useCreateControlException(controlId)
  const deleteException = useDeleteControlException()

  // 4-Augen approval request dialog
  const [approvalDialogOpen, setApprovalDialogOpen] = useState(false)
  const [pendingStatus, setPendingStatus] = useState('')
  const [approvalComment, setApprovalComment] = useState('')

  // Owner inline edit
  const [ownerEditing, setOwnerEditing] = useState(false)
  const [ownerDraft, setOwnerDraft] = useState('')

  // Due date
  const [dueDateDraft, setDueDateDraft] = useState<string>(control?.due_date ?? '')

  useEffect(() => {
    setDueDateDraft(control?.due_date ?? '')
  }, [control?.due_date])

  function dueDateStatus(): 'overdue' | 'soon' | 'ok' | 'none' {
    if (!control?.due_date) return 'none'
    const due = new Date(control.due_date)
    const now = new Date()
    const diffDays = (due.getTime() - now.getTime()) / 86_400_000
    if (diffDays < 0) return 'overdue'
    if (diffDays <= 7) return 'soon'
    return 'ok'
  }

  const statusColors: Record<string, string> = {
    overdue: 'text-severity-critical',
    soon: 'text-severity-medium',
    ok: 'text-severity-low',
    none: 'text-secondary',
  }

  function saveDueDate() {
    if (!control) return
    updateControl.mutate({
      controlId: control.id,
      not_applicable: control.not_applicable,
      reason: control.not_applicable_reason ?? '',
      manual_status: (control.manual_status as UpdateControlInput['manual_status']) ?? '',
      owner: control.owner,
      due_date: dueDateDraft || null,
    })
  }

  function handleOwnerEditStart() {
    setOwnerDraft(control?.owner ?? '')
    setOwnerEditing(true)
  }

  function handleOwnerSave() {
    if (!control) return
    updateControl.mutate(
      {
        controlId: control.id,
        not_applicable: control.not_applicable,
        reason: control.not_applicable_reason ?? '',
        manual_status: (['in_progress', 'implemented'] as const).includes(control.status as 'in_progress' | 'implemented')
          ? control.status as 'in_progress' | 'implemented'
          : '',
        owner: ownerDraft,
      },
      {
        onSuccess: () => {
          setOwnerEditing(false)
          toast('Verantwortlicher gespeichert', 'success')
        },
        onError: (err) => toast(handleApiError(err), 'error'),
      },
    )
  }

  // Exception dialog
  const [exceptionOpen, setExceptionOpen] = useState(false)
  const [exForm, setExForm] = useState<CreateControlExceptionInput>({
    title: '', reason: '', risk_accepted: '', approved_by: '', expires_at: null,
  })

  function resetExForm() {
    setExForm({ title: '', reason: '', risk_accepted: '', approved_by: '', expires_at: null })
    setExceptionOpen(false)
  }

  function handleExceptionSubmit(e: React.FormEvent) {
    e.preventDefault()
    createException.mutate(
      { ...exForm, expires_at: exForm.expires_at || null },
      {
        onSuccess: () => { resetExForm(); toast('Ausnahme gespeichert', 'success') },
        onError: (err) => toast(handleApiError(err), 'error'),
      },
    )
  }

  useEffect(() => {
    if (control) trackPage(window.location.pathname + window.location.search, control.title, '🛡️')
  }, [control?.id])

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
  const [expiresAt, setExpiresAt] = useState('')

  // Review dialog
  const [reviewOpen, setReviewOpen] = useState(false)
  const [reviewingId, setReviewingId] = useState('')
  const [reviewStatus, setReviewStatus] = useState<'approved' | 'rejected'>('approved')
  const [reviewNotes, setReviewNotes] = useState('')
  const review = useReviewEvidence(reviewingId, controlId)

  // Evidence history dialog
  const [historyEvidenceId, setHistoryEvidenceId] = useState('')
  const [historyEvidenceTitle, setHistoryEvidenceTitle] = useState('')
  const [historyOpen, setHistoryOpen] = useState(false)

  function openHistory(evidenceId: string, evidenceTitle: string) {
    setHistoryEvidenceId(evidenceId)
    setHistoryEvidenceTitle(evidenceTitle)
    setHistoryOpen(true)
  }

  // NA dialog
  const [naOpen, setNaOpen] = useState(false)

  function handleStatusChange(value: string) {
    if (!control) return

    // Non-admins must submit an approval request when org has approval_required=true.
    if (approvalRequired && !isAdmin) {
      setPendingStatus(value)
      setApprovalDialogOpen(true)
      return
    }

    if (value === 'not_applicable') { setNaOpen(true); return }
    updateControl.mutate(
      {
        controlId: control.id,
        not_applicable: false,
        reason: '',
        manual_status: value === 'missing' ? '' : value as '' | 'in_progress' | 'implemented',
      },
      {
        onSuccess: () => toast(t('secvitals.controlDetailPage.toastSaved'), 'success'),
        onError: (err) => toast(handleApiError(err), 'error'),
      },
    )
  }

  function handleSubmitApprovalRequest() {
    requestApproval.mutate(
      { requested_status: pendingStatus, comment: approvalComment },
      {
        onSuccess: () => {
          toast(t('secvitals.controlDetailPage.toastApprovalSubmitted'), 'success')
          setApprovalDialogOpen(false)
          setApprovalComment('')
          setPendingStatus('')
        },
        onError: (err) => toast(handleApiError(err), 'error'),
      },
    )
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
    setExpiresAt('')
  }

  function handleAdd(e: React.FormEvent) {
    e.preventDefault()
    const expiresAtISO = expiresAt ? new Date(expiresAt).toISOString() : null
    if (type === 'document' && file) {
      const fd = new FormData()
      fd.append('file', file)
      fd.append('title', title)
      if (notes) fd.append('notes', notes)
      if (expiresAt) fd.append('expires_at', expiresAtISO ?? '')
      uploadEvidence.mutate(fd, {
        onSuccess: () => { resetAddForm(); toast(t('secvitals.controlDetailPage.toastEvidenceUploaded'), 'success') },
        onError: (err) => toast(handleApiError(err), 'error'),
      })
    } else {
      addEvidence.mutate(
        { title, type, notes: notes || undefined, expires_at: expiresAtISO },
        {
          onSuccess: () => { resetAddForm(); toast(t('secvitals.controlDetailPage.toastEvidenceAdded'), 'success') },
          onError: (err) => toast(handleApiError(err), 'error'),
        },
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
      {
        onSuccess: () => {
          setReviewOpen(false)
          toast(t('secvitals.controlDetailPage.toastReviewDone'), 'success')
        },
        onError: (err) => toast(handleApiError(err), 'error'),
      },
    )
  }

  const backTo = frameworkId
    ? `/secvitals/frameworks/${frameworkId}`
    : '/secvitals/frameworks'

  const currentStatus = control ? toStatusChoice(control.status) : 'missing'

  return (
    <div className="flex flex-col h-full">
      <Breadcrumbs items={[
        { label: 'SecVitals', href: '/secvitals' },
        { label: framework?.name ?? 'Framework', href: frameworkId ? `/secvitals/frameworks/${frameworkId}` : '/secvitals/frameworks' },
        { label: control?.title ?? 'Control' },
      ]} />
      <PageHeader
        title={controlLoading ? '…' : (control?.title ?? 'Control')}
        description={control ? `${control.control_id} · ${control.domain}` : ''}
        actions={
          <div className="flex items-center gap-2">
            <Button variant="outline" size="sm" onClick={exportControl}>
              <Download className="w-4 h-4 mr-1" />
              {t('secvitals.controlDetailPage.export')}
            </Button>
            <Button size="sm" onClick={() => setAddOpen(true)}>
              <Plus className="w-4 h-4 mr-1" />
              {t('secvitals.controlDetailPage.addEvidence')}
            </Button>
            <Button variant="outline" size="sm" onClick={() => navigate(backTo)}>
              <ArrowLeft className="w-4 h-4 mr-1" />
              {t('secvitals.controlDetailPage.back')}
            </Button>
          </div>
        }
      />

      <div className="flex-1 p-6 space-y-6">
        {controlError && (
          <ErrorState
            message="Control konnte nicht geladen werden."
            onRetry={() => void refetchControl()}
          />
        )}

        {/* Control info card */}
        {controlLoading ? (
          <SkeletonDetailPage />
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
                  <span className="text-xs text-secondary mr-1">
                    <TermTooltip
                      term="Reifegrad"
                      explanation="Bewertung 0–5 nach CMMI: 0 = nicht vorhanden, 1 = ad-hoc, 3 = definiert, 5 = optimiert"
                    />
                  </span>
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
                <div className="flex flex-col items-end gap-1">
                  <span className="text-xs text-secondary">
                    <TermTooltip
                      term="Status"
                      explanation="Implementierungsstand dieser Kontrolle: Geplant / In Umsetzung / Implementiert / Nicht zutreffend"
                    />
                  </span>
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
                </div>
              )}
            </div>

            {control.not_applicable && control.not_applicable_reason && (
              <p className="text-xs text-secondary italic border-l-2 border-border pl-3">
                {t('secvitals.controlDetailPage.reasonLabel')}: {control.not_applicable_reason}
              </p>
            )}

            {control.description ? (
              <p className="text-sm text-secondary leading-relaxed">{control.description}</p>
            ) : (
              <p className="text-sm text-secondary italic">{t('secvitals.controlDetailPage.noDescription')}</p>
            )}

            {/* Owner inline edit */}
            <div className="flex items-center gap-2 text-sm">
              <span className="text-secondary font-medium w-28 shrink-0">Verantwortlicher</span>
              {ownerEditing ? (
                <div className="flex items-center gap-2 flex-1">
                  <Input
                    value={ownerDraft}
                    onChange={(e) => setOwnerDraft(e.target.value)}
                    placeholder="E-Mail oder Name"
                    className="h-7 text-sm flex-1"
                    autoFocus
                  />
                  <Button
                    size="sm"
                    className="h-7 px-2 text-xs"
                    onClick={handleOwnerSave}
                    disabled={updateControl.isPending}
                  >
                    Speichern
                  </Button>
                  <Button
                    size="sm"
                    variant="ghost"
                    className="h-7 px-2 text-xs"
                    onClick={() => setOwnerEditing(false)}
                  >
                    <X className="w-3.5 h-3.5" />
                  </Button>
                </div>
              ) : (
                <div className="flex items-center gap-2 flex-1">
                  <span className={cn('text-primary', !control.owner && 'text-secondary italic')}>
                    {control.owner || '-'}
                  </span>
                  <button
                    type="button"
                    onClick={handleOwnerEditStart}
                    className="text-secondary hover:text-primary transition-colors"
                    title="Verantwortlichen bearbeiten"
                  >
                    <Pencil className="w-3.5 h-3.5" />
                  </button>
                </div>
              )}
            </div>

            {/* Due date */}
            <div className="flex items-center gap-3 py-2 border-t border-border">
              <CalendarDays className="w-4 h-4 text-secondary shrink-0" aria-hidden="true" />
              <span className="text-[12px] text-secondary w-28 shrink-0">Fälligkeitsdatum</span>
              <div className="flex items-center gap-2 flex-1">
                <input
                  type="date"
                  className="text-[12px] bg-surface border border-border rounded px-2 py-1 text-primary focus:outline-none focus:ring-1 focus:ring-brand"
                  value={dueDateDraft}
                  onChange={(e) => setDueDateDraft(e.target.value)}
                  onBlur={saveDueDate}
                />
                {(() => {
                  const s = dueDateStatus()
                  if (s === 'none') return null
                  return (
                    <span className={`text-[11px] font-medium ${statusColors[s]}`}>
                      {s === 'overdue' ? '● Überfällig' : s === 'soon' ? '● Fällig bald' : '● Fällig'}
                    </span>
                  )
                })()}
              </div>
            </div>

            <ControlMappingsBadge controlId={control.id} />
          </div>
        ) : null}

        {/* Sub-controls */}
        {subControls.length > 0 && (
          <Card>
            <CardHeader>
              <CardTitle className="text-sm">{t('secvitals.controlDetailPage.subControls')}</CardTitle>
            </CardHeader>
            <CardContent className="p-0">
              <div className="overflow-x-auto">
              <Table>
                <TableHeader>
                  <TableRow>
                    <TableHead>{t('secvitals.controlDetailPage.colControlId')}</TableHead>
                    <TableHead>{t('secvitals.controlDetailPage.colTitle')}</TableHead>
                    <TableHead>{t('secvitals.controlDetailPage.colStatus')}</TableHead>
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
              </div>
            </CardContent>
          </Card>
        )}

        {/* Implementation checklist */}
        <Card>
          <CardHeader>
            <CardTitle className="text-sm flex items-center gap-2">
              <ListChecks className="w-4 h-4" />
              {t('secvitals.controlDetailPage.implementationSteps')}
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
                      title={task.completed ? t('secvitals.controlDetailPage.markOpen') : t('secvitals.controlDetailPage.markDone')}
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
                      title={t('secvitals.controlDetailPage.deleteStep')}
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
                placeholder={t('secvitals.controlDetailPage.addStep')}
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
                {t('secvitals.controlDetailPage.stepsEmpty')}
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

        {/* Exceptions / Waivers */}
        <Card>
          <CardHeader>
            <div className="flex items-center justify-between">
              <CardTitle className="text-sm flex items-center gap-2">
                <ShieldAlert className="w-4 h-4" />
                Ausnahmen / Ausnahmegenehmigungen
              </CardTitle>
              <Button size="sm" variant="outline" onClick={() => setExceptionOpen(true)}>
                <Plus className="w-3.5 h-3.5 mr-1" />
                Neue Ausnahme
              </Button>
            </div>
          </CardHeader>
          <CardContent className="p-0">
            {!exceptions || exceptions.length === 0 ? (
              <div className="py-8 text-center text-sm text-secondary">
                Keine Ausnahmen für diesen Control
              </div>
            ) : (
              <div className="divide-y divide-border">
                {exceptions.map((ex: ControlException) => (
                  <div key={ex.id} className="p-4 space-y-1.5">
                    <div className="flex items-start justify-between gap-2">
                      <div className="flex items-center gap-2 flex-wrap">
                        <span className="text-sm font-medium text-primary">{ex.title}</span>
                        <Badge
                          variant={
                            ex.status === 'active' ? 'warning'
                            : ex.status === 'revoked' ? 'destructive'
                            : 'secondary'
                          }
                          className="text-xs"
                        >
                          {ex.status === 'active' ? 'Aktiv' : ex.status === 'expired' ? 'Abgelaufen' : 'Widerrufen'}
                        </Badge>
                      </div>
                      {isAdmin && (
                        <button
                          type="button"
                          className="text-secondary hover:text-destructive transition-colors shrink-0"
                          title="Ausnahme löschen"
                          onClick={() => deleteException.mutate({ id: ex.id, controlId: controlId })}
                        >
                          <Trash2 className="w-4 h-4" />
                        </button>
                      )}
                    </div>
                    <p className="text-xs text-secondary">{ex.reason}</p>
                    <div className="flex flex-wrap gap-4 text-xs text-secondary">
                      {ex.approved_by && (
                        <span>Genehmigt von: <span className="text-primary">{ex.approved_by}</span></span>
                      )}
                      {ex.expires_at && (
                        <span>Gültig bis: <span className="text-primary">{new Date(ex.expires_at).toLocaleDateString(formatLocale())}</span></span>
                      )}
                    </div>
                  </div>
                ))}
              </div>
            )}
          </CardContent>
        </Card>

        {/* Evidence */}
        <Card>
          <CardHeader>
            <CardTitle className="text-sm">{t('secvitals.controlDetailPage.evidence')}</CardTitle>
          </CardHeader>
          <CardContent className="p-0">
            {evidenceLoading ? (
              <div className="flex justify-center py-8">
                <Spinner size="md" />
              </div>
            ) : !evidence || evidence.length === 0 ? (
              <div className="flex flex-col items-center py-12 text-center">
                <FileText className="w-10 h-10 text-gray-300 mb-3" />
                <p className="text-sm font-medium text-secondary">{t('secvitals.controlDetailPage.noEvidence')}</p>
                <p className="text-xs text-secondary mt-1">{t('secvitals.controlDetailPage.noEvidenceDesc')}</p>
              </div>
            ) : (
              <div className="overflow-x-auto">
              <Table>
                <TableHeader>
                  <TableRow>
                    <TableHead>{t('secvitals.controlDetailPage.evidenceColTitle')}</TableHead>
                    <TableHead>{t('secvitals.controlDetailPage.evidenceColType')}</TableHead>
                    <TableHead>{t('secvitals.controlDetailPage.evidenceColStatus')}</TableHead>
                    <TableHead>{t('secvitals.controlDetailPage.evidenceColExpiry')}</TableHead>
                    <TableHead>{t('secvitals.controlDetailPage.evidenceColAdded')}</TableHead>
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
                      <TableCell>
                        <EvidenceExpiryBadge expiresAt={ev.expires_at} />
                      </TableCell>
                      <TableCell className="text-sm text-secondary">
                        {new Date(ev.created_at).toLocaleDateString(formatLocale())}
                      </TableCell>
                      <TableCell>
                        <div className="flex items-center gap-2">
                          {ev.status === 'pending_review' && (
                            <Button
                              variant="outline"
                              size="sm"
                              onClick={() => openReview(ev.id)}
                            >
                              {t('secvitals.controlDetailPage.review')}
                            </Button>
                          )}
                          <Button
                            variant="ghost"
                            size="sm"
                            className="text-secondary hover:text-primary"
                            title="Verlauf anzeigen"
                            onClick={() => openHistory(ev.id, ev.title)}
                          >
                            <History className="w-3.5 h-3.5" aria-hidden="true" />
                          </Button>
                        </div>
                      </TableCell>
                    </TableRow>
                  ))}
                </TableBody>
              </Table>
              </div>
            )}
          </CardContent>
        </Card>

        {/* Evidence File Attachments */}
        <Card>
          <CardHeader>
            <CardTitle className="text-sm">{t('secvitals.controlDetailPage.attachments')}</CardTitle>
          </CardHeader>
          <CardContent>
            <EvidenceFileUpload controlId={controlId} />
          </CardContent>
        </Card>

        {/* Collaborative Tasks */}
        <TasksPanel entityType="control" entityId={controlId} />

        {/* Comments */}
        <Comments entityType="control" entityId={controlId} />

        {/* Change Log */}
        <Card>
          <CardHeader>
            <CardTitle className="text-sm flex items-center gap-2">
              <History className="w-4 h-4" aria-hidden="true" />
              Änderungen
            </CardTitle>
          </CardHeader>
          <CardContent>
            <ChangeLogTab controlId={controlId} />
          </CardContent>
        </Card>
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

      {/* Evidence History Dialog */}
      <EvidenceHistoryDialog
        evidenceId={historyEvidenceId}
        evidenceTitle={historyEvidenceTitle}
        open={historyOpen}
        onClose={() => setHistoryOpen(false)}
      />

      {/* 4-Augen: Approval Request Dialog */}
      <Dialog open={approvalDialogOpen} onOpenChange={(v) => { if (!v) { setApprovalDialogOpen(false); setApprovalComment(''); setPendingStatus('') } }}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>{t('secvitals.controlDetailPage.approvalDialogTitle')}</DialogTitle>
          </DialogHeader>
          <div className="space-y-4 py-2">
            <p className="text-sm text-secondary">
              {t('secvitals.controlDetailPage.approvalDesc')}
            </p>
            <div className="text-sm space-y-1">
              <p>
                <span className="font-medium text-primary">{t('secvitals.controlDetailPage.requestedStatus')}:</span>{' '}
                <span className="text-brand font-medium">
                  {pendingStatus === 'missing' ? t('secvitals.controlDetailPage.statusMissing')
                    : pendingStatus === 'in_progress' ? t('secvitals.controlDetailPage.statusInProgress')
                    : pendingStatus === 'implemented' ? t('secvitals.controlDetailPage.statusImplemented')
                    : pendingStatus === 'not_applicable' ? t('secvitals.controlDetailPage.statusNotApplicable')
                    : pendingStatus}
                </span>
              </p>
            </div>
            <div className="space-y-1.5">
              <Label className="text-xs">{t('secvitals.controlDetailPage.approvalComment')}</Label>
              <Textarea
                value={approvalComment}
                onChange={(e) => setApprovalComment(e.target.value)}
                placeholder={t('secvitals.controlDetailPage.approvalPlaceholder')}
                rows={3}
              />
            </div>
          </div>
          <DialogFooter>
            <Button
              variant="outline"
              onClick={() => { setApprovalDialogOpen(false); setApprovalComment(''); setPendingStatus('') }}
              disabled={requestApproval.isPending}
            >
              {t('common.cancel')}
            </Button>
            <Button
              onClick={handleSubmitApprovalRequest}
              disabled={requestApproval.isPending}
            >
              {requestApproval.isPending ? t('secvitals.controlDetailPage.submitting') : t('secvitals.controlDetailPage.submitApproval')}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      {/* Add Evidence Dialog */}
      <Dialog open={addOpen} onOpenChange={setAddOpen}>
        <DialogContent>
          <DialogHeader><DialogTitle>{t('secvitals.controlDetailPage.addEvidenceTitle')}</DialogTitle></DialogHeader>
          <form onSubmit={(e) => { void handleAdd(e) }}>
            <div className="py-4 space-y-4">
              <div className="space-y-1.5">
                <Label htmlFor="ev-title">{t('secvitals.controlDetailPage.evidenceLabelTitle')}</Label>
                <Input
                  id="ev-title"
                  value={title}
                  onChange={(e) => setTitle(e.target.value)}
                  placeholder={t('secvitals.controlDetailPage.evidencePlaceholderTitle')}
                  required
                />
              </div>
              <div className="space-y-1.5">
                <Label>{t('secvitals.controlDetailPage.evidenceLabelType')}</Label>
                <Select value={type} onValueChange={(v) => { setType(v as Evidence['type']); setFile(null) }}>
                  <SelectTrigger>
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent>
                    <SelectItem value="manual">{t('secvitals.controlDetailPage.evidenceTypeManual')}</SelectItem>
                    <SelectItem value="document">{t('secvitals.controlDetailPage.evidenceTypeDocument')}</SelectItem>
                    <SelectItem value="automated">{t('secvitals.controlDetailPage.evidenceTypeAutomated')}</SelectItem>
                  </SelectContent>
                </Select>
              </div>

              {type === 'document' && (
                <div className="space-y-1.5">
                  <Label htmlFor="ev-file">{t('secvitals.controlDetailPage.evidenceLabelFile')}</Label>
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
                    <p className="font-medium text-primary mb-1">{t('secvitals.controlDetailPage.automatedEvidenceTitle')}</p>
                    <p>{t('secvitals.controlDetailPage.automatedEvidenceDesc')}</p>
                    <Button
                      type="button"
                      size="sm"
                      variant="outline"
                      className="mt-2 h-6 text-xs"
                      onClick={() => collectEvidence.mutate()}
                      disabled={collectEvidence.isPending}
                    >
                      <RefreshCw className="w-3 h-3 mr-1" />
                      {collectEvidence.isPending ? t('secvitals.controlDetailPage.collecting') : t('secvitals.controlDetailPage.collectNow')}
                    </Button>
                  </div>
                </div>
              )}

              <div className="space-y-1.5">
                <Label htmlFor="ev-notes">{t('secvitals.controlDetailPage.evidenceLabelNotes')}</Label>
                <textarea
                  id="ev-notes"
                  rows={3}
                  className="w-full rounded-md border border-border bg-surface2 text-primary px-3 py-2 text-sm focus:outline-none focus:ring-1 focus:ring-brand"
                  value={notes}
                  onChange={(e) => setNotes(e.target.value)}
                  placeholder={t('secvitals.controlDetailPage.evidencePlaceholderNotes')}
                />
              </div>

              <div className="space-y-1.5">
                <Label htmlFor="ev-expires-at">{t('secvitals.controlDetailPage.evidenceLabelExpiry')}</Label>
                <Input
                  id="ev-expires-at"
                  type="date"
                  value={expiresAt}
                  onChange={(e) => setExpiresAt(e.target.value)}
                  min={new Date().toISOString().split('T')[0]}
                />
                <p className="text-xs text-secondary">
                  {t('secvitals.controlDetailPage.evidenceExpiryHint')}
                </p>
              </div>
            </div>
            <DialogFooter>
              <Button type="button" variant="outline" onClick={() => setAddOpen(false)}>{t('common.cancel')}</Button>
              <Button
                type="submit"
                disabled={
                  addEvidence.isPending ||
                  uploadEvidence.isPending ||
                  (type === 'document' && !file)
                }
              >
                {addEvidence.isPending || uploadEvidence.isPending ? t('secvitals.controlDetailPage.adding') : t('secvitals.controlDetailPage.addEvidenceSubmit')}
              </Button>
            </DialogFooter>
          </form>
        </DialogContent>
      </Dialog>

      {/* Review Dialog */}
      <Dialog open={reviewOpen} onOpenChange={setReviewOpen}>
        <DialogContent>
          <DialogHeader><DialogTitle>{t('secvitals.controlDetailPage.reviewTitle')}</DialogTitle></DialogHeader>
          <form onSubmit={(e) => { void handleReview(e) }}>
            <div className="py-4 space-y-4">
              <div className="space-y-1.5">
                <Label>{t('secvitals.controlDetailPage.reviewDecision')}</Label>
                <Select value={reviewStatus} onValueChange={(v) => setReviewStatus(v as 'approved' | 'rejected')}>
                  <SelectTrigger>
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent>
                    <SelectItem value="approved">{t('secvitals.controlDetailPage.reviewApprove')}</SelectItem>
                    <SelectItem value="rejected">{t('secvitals.controlDetailPage.reviewReject')}</SelectItem>
                  </SelectContent>
                </Select>
              </div>
              <div className="space-y-1.5">
                <Label htmlFor="review-notes">{t('secvitals.controlDetailPage.reviewLabelNotes')}</Label>
                <textarea
                  id="review-notes"
                  rows={3}
                  className="w-full rounded-md border border-border bg-surface2 text-primary px-3 py-2 text-sm focus:outline-none focus:ring-1 focus:ring-brand"
                  value={reviewNotes}
                  onChange={(e) => setReviewNotes(e.target.value)}
                  placeholder={t('secvitals.controlDetailPage.reviewPlaceholderNotes')}
                />
              </div>
            </div>
            <DialogFooter>
              <Button type="button" variant="outline" onClick={() => setReviewOpen(false)}>{t('common.cancel')}</Button>
              <Button type="submit" disabled={review.isPending}>
                {review.isPending ? t('secvitals.controlDetailPage.reviewSaving') : t('secvitals.controlDetailPage.reviewComplete')}
              </Button>
            </DialogFooter>
          </form>
        </DialogContent>
      </Dialog>

      {/* Exception (Waiver) Dialog */}
      <Dialog open={exceptionOpen} onOpenChange={(v) => { if (!v) resetExForm() }}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>Neue Ausnahme anlegen</DialogTitle>
          </DialogHeader>
          <form onSubmit={(e) => { void handleExceptionSubmit(e) }}>
            <div className="py-4 space-y-4">
              <div className="space-y-1.5">
                <Label htmlFor="ex-title">Titel <span className="text-red-500">*</span></Label>
                <Input
                  id="ex-title"
                  value={exForm.title}
                  onChange={(e) => setExForm((f) => ({ ...f, title: e.target.value }))}
                  placeholder="Kurze Beschreibung der Ausnahme"
                  required
                />
              </div>
              <div className="space-y-1.5">
                <Label htmlFor="ex-reason">Begründung <span className="text-red-500">*</span></Label>
                <textarea
                  id="ex-reason"
                  rows={3}
                  className="w-full rounded-md border border-border bg-surface2 text-primary px-3 py-2 text-sm focus:outline-none focus:ring-1 focus:ring-brand"
                  value={exForm.reason}
                  onChange={(e) => setExForm((f) => ({ ...f, reason: e.target.value }))}
                  placeholder="Warum wird diese Ausnahme beantragt?"
                  required
                />
              </div>
              <div className="space-y-1.5">
                <Label htmlFor="ex-risk">
                  <TermTooltip
                    term="Akzeptiertes Risiko"
                    explanation="Formale Risikoakzeptanz: Das verbleibende Risiko nach Maßnahmen wird bewusst in Kauf genommen"
                  />{' '}
                  <span className="text-red-500">*</span>
                </Label>
                <textarea
                  id="ex-risk"
                  rows={2}
                  className="w-full rounded-md border border-border bg-surface2 text-primary px-3 py-2 text-sm focus:outline-none focus:ring-1 focus:ring-brand"
                  value={exForm.risk_accepted}
                  onChange={(e) => setExForm((f) => ({ ...f, risk_accepted: e.target.value }))}
                  placeholder="Welches Risiko wird bewusst akzeptiert?"
                  required
                />
              </div>
              <div className="space-y-1.5">
                <Label htmlFor="ex-approved-by">Genehmigt von</Label>
                <Input
                  id="ex-approved-by"
                  value={exForm.approved_by ?? ''}
                  onChange={(e) => setExForm((f) => ({ ...f, approved_by: e.target.value }))}
                  placeholder="E-Mail oder Name des Genehmigenden"
                />
              </div>
              <div className="space-y-1.5">
                <Label htmlFor="ex-expires-at">Gültig bis</Label>
                <Input
                  id="ex-expires-at"
                  type="date"
                  value={typeof exForm.expires_at === 'string' ? exForm.expires_at : ''}
                  onChange={(e) => setExForm((f) => ({ ...f, expires_at: e.target.value || null }))}
                  min={new Date().toISOString().split('T')[0]}
                />
              </div>
            </div>
            <DialogFooter>
              <Button type="button" variant="outline" onClick={resetExForm}>Abbrechen</Button>
              <Button
                type="submit"
                disabled={
                  createException.isPending ||
                  !exForm.title.trim() ||
                  !exForm.reason.trim() ||
                  !exForm.risk_accepted.trim()
                }
              >
                {createException.isPending ? 'Wird gespeichert…' : 'Ausnahme speichern'}
              </Button>
            </DialogFooter>
          </form>
        </DialogContent>
      </Dialog>
    </div>
  )
}
