import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { Users, Plus, Pencil, Trash2, AlertTriangle, Download, ShieldCheck, CheckCircle2 } from 'lucide-react'
import { Spinner } from '../../../components/Spinner'
import { Button } from '../../../components/ui/button'
import { Card, CardContent } from '../../../components/ui/card'
import { Badge } from '../../../components/ui/badge'
import { Dialog, DialogContent, DialogHeader, DialogTitle, DialogFooter } from '../../../components/ui/dialog'
import { AlertDialog, AlertDialogContent, AlertDialogHeader, AlertDialogTitle, AlertDialogDescription, AlertDialogFooter, AlertDialogCancel, AlertDialogAction } from '../../../components/ui/alert-dialog'
import { Input } from '../../../components/ui/input'
import { Label } from '../../../components/ui/label'
import { Textarea } from '../../../components/ui/textarea'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '../../../components/ui/select'
import { PageHeader } from '../../../shared/components/PageHeader'
import { EmptyState } from '../../../shared/components/EmptyState'
import { InfoBanner } from '../../../shared/components/InfoBanner'
import { TermTooltip } from '../../../shared/components/TermTooltip'
import { useDSRs, useCreateDSR, useUpdateDSR, useDeleteDSR, useDSRSummary, useResolveDSR } from '../hooks/useDSRs'
import { ComplianceTooltip } from '../../../shared/components/ComplianceTooltip'
import type { DSR, DSRType, DSRStatus, CreateDSRInput, UpdateDSRInput, ResolveDSRInput } from '../types'
import { useFormatDate } from '../../../shared/hooks/useFormatDate'

const STATUS_CLASS: Record<DSRStatus, string> = {
  open: 'bg-yellow-500/20 text-yellow-400 border-yellow-500/30',
  in_progress: 'bg-blue-500/20 text-blue-400 border-blue-500/30',
  completed: 'bg-green-500/20 text-green-400 border-green-500/30',
  rejected: 'bg-secondary text-secondary-foreground',
  extended: 'bg-purple-500/20 text-purple-400 border-purple-500/30',
  overdue: 'bg-red-500/20 text-red-400 border-red-500/30',
}

function isOverdue(dueDate?: string): boolean {
  if (!dueDate) return false
  return new Date(dueDate) < new Date()
}

function daysUntil(dateStr?: string): number | null {
  if (!dateStr) return null
  const diff = new Date(dateStr).getTime() - Date.now()
  return Math.ceil(diff / (1000 * 60 * 60 * 24))
}

interface CreateFormState {
  requester_name: string
  requester_email: string
  type: DSRType
  description: string
  notes: string
  channel: string
  reference_id: string
}

interface EditFormState {
  status: DSRStatus
  notes: string
}

function emptyCreateForm(): CreateFormState {
  return {
    requester_name: '',
    requester_email: '',
    type: 'access',
    description: '',
    notes: '',
    channel: '',
    reference_id: '',
  }
}

function DSRCard({
  dsr,
  onEdit,
  onDelete,
  onErasure,
  onResolve,
}: {
  dsr: DSR
  onEdit: (d: DSR) => void
  onDelete: (id: string) => void
  onErasure?: (id: string) => void
  onResolve: (d: DSR) => void
}) {
  const { t } = useTranslation()
  const { formatDate } = useFormatDate()
  const overdue = isOverdue(dsr.due_date) && dsr.status !== 'completed' && dsr.status !== 'rejected' && dsr.status !== 'extended'
  const days = daysUntil(dsr.due_date)
  const receivedDate = formatDate(dsr.received_at, { year: 'numeric', month: 'short', day: 'numeric' })
  const getDSRTypeLabel = (type: DSRType) => t(`vaktprivacy.dsrPage.type${type.charAt(0).toUpperCase() + type.slice(1).replace(/_([a-z])/g, (_, c: string) => c.toUpperCase())}`, { defaultValue: type })
  const getDSRStatusLabel = (status: DSRStatus) => t(`vaktprivacy.dsrPage.status${status.charAt(0).toUpperCase() + status.slice(1).replace(/_([a-z])/g, (_, c: string) => c.toUpperCase())}`, { defaultValue: status })

  return (
    <Card className={overdue || dsr.status === 'overdue' ? 'border-red-500/30' : ''}>
      <CardContent className="pt-5 space-y-2">
        <div className="flex items-start justify-between gap-2">
          <div className="min-w-0">
            <p className="font-medium text-sm truncate">{dsr.requester_name}</p>
            <p className="text-xs text-muted-foreground truncate">{dsr.requester_email}</p>
          </div>
          <Badge className={STATUS_CLASS[dsr.status]}>{getDSRStatusLabel(dsr.status)}</Badge>
        </div>

        <div className="flex flex-wrap gap-1.5">
          <Badge variant="outline" className="text-xs font-normal">{getDSRTypeLabel(dsr.type)}</Badge>
          {dsr.channel && <Badge variant="outline" className="text-xs font-normal text-gray-400">{dsr.channel}</Badge>}
        </div>

        {dsr.description && (
          <p className="text-xs text-muted-foreground line-clamp-2">{dsr.description}</p>
        )}

        <div className="flex flex-wrap gap-3 text-xs text-muted-foreground">
          <span>{t('vaktprivacy.dsrPage.cardReceived')} {receivedDate}</span>
          {dsr.due_date && (
            <span className={overdue ? 'text-red-400 font-medium' : days !== null && days <= 7 ? 'text-amber-500 font-medium' : ''}>
              {overdue ? (
                <><AlertTriangle className="w-3 h-3 inline mr-0.5" />{t('vaktprivacy.dsrPage.cardDeadlineExpired')}</>
              ) : days !== null ? (
                <>{t('vaktprivacy.dsrPage.cardDeadline')} {days > 0 ? t('vaktprivacy.dsrPage.cardDaysLeft', { days }) : t('vaktprivacy.dsrPage.cardToday')} ({formatDate(dsr.due_date)})</>
              ) : null}
            </span>
          )}
          {dsr.extension_due_at && dsr.status === 'extended' && (
            <span className="text-purple-400">
              {t('vaktprivacy.dsrPage.cardExtension')} {formatDate(dsr.extension_due_at)}
            </span>
          )}
        </div>

        {dsr.notes && (
          <p className="text-xs text-muted-foreground italic line-clamp-1">{dsr.notes}</p>
        )}

        <div className="flex justify-end gap-1 pt-1 flex-wrap">
          {dsr.type === 'erasure' && dsr.status !== 'completed' && dsr.status !== 'rejected' && onErasure && (
            <Button
              size="sm"
              variant="outline"
              className="h-7 text-xs gap-1 text-green-400 border-green-500/30 hover:bg-green-500/10"
              onClick={() => { onErasure(dsr.id); }}
            >
              <ShieldCheck className="w-3.5 h-3.5" />
              {t('vaktprivacy.dsrPage.buttonErasure')}
            </Button>
          )}
          {(dsr.status === 'open' || dsr.status === 'in_progress' || dsr.status === 'overdue') && (
            <Button
              size="sm"
              variant="outline"
              className="h-7 text-xs gap-1"
              onClick={() => { onResolve(dsr); }}
            >
              <CheckCircle2 className="w-3.5 h-3.5" />
              {t('vaktprivacy.dsrPage.buttonResolve')}
            </Button>
          )}
          <Button size="icon" variant="ghost" className="h-7 w-7" aria-label={t('vaktprivacy.dsrPage.ariaEdit')} onClick={() => { onEdit(dsr); }}>
            <Pencil className="w-3.5 h-3.5" />
          </Button>
          <Button
            size="icon"
            variant="ghost"
            className="h-7 w-7 text-destructive hover:text-destructive"
            aria-label={t('vaktprivacy.dsrPage.ariaDelete')}
            onClick={() => { onDelete(dsr.id); }}
          >
            <Trash2 className="w-3.5 h-3.5" />
          </Button>
        </div>
      </CardContent>
    </Card>
  )
}

export default function DSRPage() {
  const { t } = useTranslation()
  const [dialogMode, setDialogMode] = useState<'create' | 'edit' | null>(null)
  const [editId, setEditId] = useState<string | null>(null)
  const [createForm, setCreateForm] = useState<CreateFormState>(emptyCreateForm())
  const [editForm, setEditForm] = useState<EditFormState>({ status: 'open', notes: '' })
  const [deleteId, setDeleteId] = useState<string | null>(null)
  const [erasureId, setErasureId] = useState<string | null>(null)
  const [erasureNote, setErasureNote] = useState('')
  const [resolveTarget, setResolveTarget] = useState<DSR | null>(null)
  const [resolveForm, setResolveForm] = useState<ResolveDSRInput>({ resolution_type: 'completed', resolution_notes: '', extension_reason: '' })

  const { data: dsrs, isLoading, isError } = useDSRs()
  const { data: summary } = useDSRSummary()
  const createDSR = useCreateDSR()
  const updateDSR = useUpdateDSR()
  const deleteDSR = useDeleteDSR()
  const resolveDSR = useResolveDSR()

  function openCreate() {
    setCreateForm(emptyCreateForm())
    setEditId(null)
    setDialogMode('create')
  }

  function openEdit(dsr: DSR) {
    setEditForm({ status: dsr.status, notes: dsr.notes ?? '' })
    setEditId(dsr.id)
    setDialogMode('edit')
  }

  function openResolve(dsr: DSR) {
    setResolveTarget(dsr)
    setResolveForm({ resolution_type: 'completed', resolution_notes: '', extension_reason: '' })
  }

  function handleErasureOpen(id: string) {
    setErasureId(id)
    setErasureNote('')
  }

  function handleErasureConfirm() {
    if (!erasureId) return
    const id = erasureId
    setErasureId(null)
    updateDSR.mutate({
      id,
      input: { status: 'completed', notes: erasureNote || 'Löschung ausgeführt (Art. 17 DSGVO).' },
    })
  }

  function confirmDelete() {
    if (deleteId) deleteDSR.mutate(deleteId)
    setDeleteId(null)
  }

  function handleSubmit() {
    if (dialogMode === 'create') {
      const payload: CreateDSRInput = {
        requester_name: createForm.requester_name,
        requester_email: createForm.requester_email,
        type: createForm.type,
        description: createForm.description || undefined,
        notes: createForm.notes || undefined,
        channel: createForm.channel || undefined,
        reference_id: createForm.reference_id || undefined,
      }
      createDSR.mutate(payload, { onSuccess: () => { setDialogMode(null); } })
    } else if (dialogMode === 'edit' && editId) {
      const payload: UpdateDSRInput = {
        status: editForm.status,
        notes: editForm.notes || undefined,
      }
      updateDSR.mutate({ id: editId, input: payload }, { onSuccess: () => { setDialogMode(null); } })
    }
  }

  function handleResolve() {
    if (!resolveTarget) return
    resolveDSR.mutate(
      { id: resolveTarget.id, input: resolveForm },
      { onSuccess: () => { setResolveTarget(null); } },
    )
  }

  function handlePDFExport() {
    const a = document.createElement('a')
    a.href = '/api/v1/vaktprivacy/dsr/export?format=pdf'
    a.download = `dsr-auditlog-${new Date().toISOString().slice(0, 10)}.pdf`
    document.body.appendChild(a)
    a.click()
    a.remove()
  }

  const isPending = createDSR.isPending || updateDSR.isPending
  const canSubmitCreate = createForm.requester_name && createForm.requester_email && !isPending

  const openDSRs = dsrs?.filter((d) => d.status === 'open' || d.status === 'in_progress' || d.status === 'overdue' || d.status === 'extended') ?? []
  const closedDSRs = dsrs?.filter((d) => d.status === 'completed' || d.status === 'rejected') ?? []
  const overdueCount = summary?.overdue_count ?? openDSRs.filter(d => isOverdue(d.due_date)).length

  return (
    <div className="flex flex-col h-full">
      <PageHeader
        title={t('vaktprivacy.dsrPage.title')}
        description={t('vaktprivacy.dsrPage.description')}
        actions={
          <div className="flex items-center gap-2">
            <Button variant="outline" onClick={handlePDFExport}>
              <Download className="w-4 h-4 mr-1" />
              {t('vaktprivacy.dsrPage.exportAuditPDF')}
            </Button>
            <Button variant="outline" onClick={() => {
              void fetch('/api/v1/vaktprivacy/dsr/export?format=csv', { credentials: 'include' })
                .then(res => res.blob())
                .then(blob => {
                  const url = URL.createObjectURL(blob)
                  const a = document.createElement('a')
                  a.href = url
                  a.download = `dsr-export-${new Date().toISOString().slice(0, 10)}.csv`
                  document.body.appendChild(a)
                  a.click()
                  a.remove()
                  URL.revokeObjectURL(url)
                })
            }}>
              <Download className="w-4 h-4 mr-1" />
              CSV
            </Button>
            <Button onClick={openCreate}>
              <Plus className="w-4 h-4 mr-1" />
              {t('vaktprivacy.dsrPage.createDSR')}
            </Button>
          </div>
        }
      />

      <InfoBanner icon={Users} title={t('vaktprivacy.dsrPage.infoBannerTitle')}>
        <p>
          <TermTooltip term="DSR" explanation={t('vaktprivacy.dsrPage.bannerTooltipDSR')}>{t('vaktprivacy.dsrPage.bannerTooltipLabel')}</TermTooltip>
          {t('vaktprivacy.dsrPage.bannerDesc1')}
        </p>
      </InfoBanner>

      {/* Summary stats */}
      {summary && (
        <div className="px-6 py-3 grid grid-cols-4 gap-3">
          <div className="bg-white border rounded-lg p-3 text-center">
            <div className="text-xl font-bold">{summary.open_count}</div>
            <div className="text-xs text-gray-500">{t('vaktprivacy.dsrPage.statOpen')}</div>
          </div>
          <div className={`border rounded-lg p-3 text-center ${summary.overdue_count > 0 ? 'bg-red-50 border-red-200' : 'bg-white'}`}>
            <div className={`text-xl font-bold ${summary.overdue_count > 0 ? 'text-red-600' : ''}`}>{summary.overdue_count}</div>
            <div className="text-xs text-gray-500">{t('vaktprivacy.dsrPage.statOverdue')}</div>
          </div>
          <div className="bg-white border rounded-lg p-3 text-center">
            <div className="text-xl font-bold">{summary.fulfilled_last_12m}</div>
            <div className="text-xs text-gray-500">{t('vaktprivacy.dsrPage.statFulfilled12M')}</div>
          </div>
          <div className="bg-white border rounded-lg p-3 text-center">
            <div className="text-xl font-bold">{summary.on_time_rate_pct}%</div>
            <div className="text-xs text-gray-500">{t('vaktprivacy.dsrPage.statOnTime')}</div>
          </div>
        </div>
      )}

      <div className="flex-1 p-6 space-y-6">
        {isLoading && (
          <div className="flex items-center justify-center h-48">
            <Spinner size="lg" color="primary" />
          </div>
        )}

        {isError && (
          <div className="text-sm text-red-400 p-4 bg-red-500/10 rounded-lg">
            {t('vaktprivacy.dsrPage.errorLoading')}
          </div>
        )}

        {!isLoading && !isError && dsrs?.length === 0 && (
          <EmptyState
            icon={Users}
            title={t('vaktprivacy.dsrPage.emptyTitle')}
            description={t('vaktprivacy.dsrPage.emptyDesc')}
            action={
              <Button onClick={openCreate}>
                <Plus className="w-4 h-4 mr-1" />
                {t('vaktprivacy.dsrPage.createDSR')}
              </Button>
            }
          />
        )}

        {!isLoading && !isError && dsrs && dsrs.length > 0 && (
          <>
            {overdueCount > 0 && (
              <div className="flex items-start gap-3 p-4 bg-red-500/10 border border-red-500/30 rounded-lg">
                <AlertTriangle className="w-5 h-5 text-red-500 shrink-0 mt-0.5" />
                <div>
                  <p className="text-sm font-semibold text-red-500">
                    {t(overdueCount > 1 ? 'vaktprivacy.dsrPage.overdueAlertPlural' : 'vaktprivacy.dsrPage.overdueAlert', { count: overdueCount })}
                  </p>
                  <p className="text-xs text-secondary mt-0.5">
                    {t('vaktprivacy.dsrPage.overdueHint')}
                  </p>
                </div>
              </div>
            )}

            {openDSRs.length > 0 && (
              <div className="space-y-3">
                <h2 className="text-sm font-semibold text-secondary">{t('vaktprivacy.dsrPage.openSection', { count: openDSRs.length })}</h2>
                <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 gap-4">
                  {openDSRs.map((d) => (
                    <DSRCard key={d.id} dsr={d} onEdit={openEdit} onDelete={(id) => { setDeleteId(id); }} onErasure={handleErasureOpen} onResolve={openResolve} />
                  ))}
                </div>
              </div>
            )}

            {closedDSRs.length > 0 && (
              <div className="space-y-3">
                <h2 className="text-sm font-semibold text-muted-foreground">{t('vaktprivacy.dsrPage.closedSection')}</h2>
                <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 gap-4">
                  {closedDSRs.map((d) => (
                    <DSRCard key={d.id} dsr={d} onEdit={openEdit} onDelete={(id) => { setDeleteId(id); }} onErasure={handleErasureOpen} onResolve={openResolve} />
                  ))}
                </div>
              </div>
            )}
          </>
        )}
      </div>

      {/* Erasure Dialog */}
      <Dialog open={erasureId !== null} onOpenChange={(open) => { if (!open) { setErasureId(null); } }}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>{t('vaktprivacy.dsrPage.erasureDialogTitle')}</DialogTitle>
          </DialogHeader>
          <p className="text-sm text-muted-foreground">
            {t('vaktprivacy.dsrPage.erasureConfirmDesc')}
          </p>
          <div className="space-y-2">
            <Label htmlFor="erasure-note">{t('vaktprivacy.dsrPage.erasureLabelNote')}</Label>
            <Textarea
              id="erasure-note"
              placeholder="z.B. Kundendatensätze in DB gelöscht, Backups werden nach 30 Tagen überschrieben."
              value={erasureNote}
              onChange={(e) => { setErasureNote(e.target.value); }}
              rows={3}
            />
          </div>
          <DialogFooter>
            <Button variant="ghost" onClick={() => { setErasureId(null); }}>{t('common.cancel')}</Button>
            <Button onClick={handleErasureConfirm} className="bg-green-600 hover:bg-green-700 text-white">
              <ShieldCheck className="w-4 h-4 mr-1.5" />
              {t('vaktprivacy.dsrPage.erasureConfirmBtn')}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      {/* Resolve Dialog */}
      <Dialog open={resolveTarget !== null} onOpenChange={(open) => { if (!open) { setResolveTarget(null); } }}>
        <DialogContent className="max-w-lg">
          <DialogHeader>
            <DialogTitle>{t('vaktprivacy.dsrPage.resolveDialogTitle')} — {resolveTarget?.requester_name}</DialogTitle>
          </DialogHeader>
          <div className="space-y-4 py-2">
            <div className="space-y-1.5">
              <Label>{t('vaktprivacy.dsrPage.resolveLabelResult')}</Label>
              <Select
                value={resolveForm.resolution_type}
                onValueChange={(v) => { setResolveForm(f => ({ ...f, resolution_type: v as DSRStatus })); }}
              >
                <SelectTrigger><SelectValue /></SelectTrigger>
                <SelectContent>
                  <SelectItem value="completed">{t('vaktprivacy.dsrPage.resolveCompleted')}</SelectItem>
                  <SelectItem value="rejected">{t('vaktprivacy.dsrPage.resolveRejected')}</SelectItem>
                  <SelectItem value="extended">{t('vaktprivacy.dsrPage.resolveExtended')}</SelectItem>
                </SelectContent>
              </Select>
            </div>
            {resolveForm.resolution_type === 'extended' && (
              <div className="space-y-1.5">
                <Label>{t('vaktprivacy.dsrPage.resolveLabelExtensionReason')}</Label>
                <Textarea
                  rows={2}
                  placeholder="Begründung (Pflicht bei Verlängerung nach Art. 12 Abs. 3 DSGVO)"
                  value={resolveForm.extension_reason}
                  onChange={(e) => { setResolveForm(f => ({ ...f, extension_reason: e.target.value })); }}
                />
              </div>
            )}
            <div className="space-y-1.5">
              <Label>{t('vaktprivacy.dsrPage.resolveLabelNotes')}</Label>
              <Textarea
                rows={3}
                placeholder="Maßnahmen, Kommentare …"
                value={resolveForm.resolution_notes}
                onChange={(e) => { setResolveForm(f => ({ ...f, resolution_notes: e.target.value })); }}
              />
            </div>
          </div>
          <DialogFooter>
            <Button variant="outline" onClick={() => { setResolveTarget(null); }}>{t('common.cancel')}</Button>
            <Button
              onClick={handleResolve}
              disabled={resolveDSR.isPending || (resolveForm.resolution_type === 'extended' && !resolveForm.extension_reason?.trim())}
            >
              {resolveDSR.isPending ? t('vaktprivacy.dsrPage.savingPending') : t('vaktprivacy.dsrPage.resolveBtn')}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      <AlertDialog open={deleteId !== null} onOpenChange={(open) => { if (!open) { setDeleteId(null); } }}>
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>{t('vaktprivacy.dsrPage.deleteTitle')}</AlertDialogTitle>
            <AlertDialogDescription>{t('vaktprivacy.dsrPage.deleteDesc')}</AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel onClick={() => { setDeleteId(null); }}>{t('common.cancel')}</AlertDialogCancel>
            <AlertDialogAction onClick={confirmDelete} className="bg-destructive text-destructive-foreground hover:bg-destructive/90">{t('common.delete')}</AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>

      {/* Create Dialog */}
      <Dialog open={dialogMode === 'create'} onOpenChange={(open) => { if (!open) { setDialogMode(null); } }}>
        <DialogContent className="max-w-lg max-h-[90vh] overflow-y-auto">
          <DialogHeader>
            <DialogTitle><ComplianceTooltip term="dsr">{t('vaktprivacy.dsrPage.createDialogTitle')}</ComplianceTooltip></DialogTitle>
          </DialogHeader>
          <div className="space-y-4 py-2">
            <div className="p-3 rounded-lg bg-blue-500/10 text-blue-400 text-xs">
              {t('vaktprivacy.dsrPage.createHint')}
            </div>
            <div className="space-y-1.5">
              <Label>{t('vaktprivacy.dsrPage.labelRequesterName')}</Label>
              <Input
                placeholder="z.B. Max Mustermann"
                value={createForm.requester_name}
                onChange={(e) => { setCreateForm((f) => ({ ...f, requester_name: e.target.value })); }}
              />
            </div>
            <div className="space-y-1.5">
              <Label>{t('vaktprivacy.dsrPage.labelEmail')}</Label>
              <Input
                type="email"
                placeholder="max@example.com"
                value={createForm.requester_email}
                onChange={(e) => { setCreateForm((f) => ({ ...f, requester_email: e.target.value })); }}
              />
            </div>
            <div className="grid grid-cols-2 gap-3">
              <div className="space-y-1.5">
                <Label>{t('vaktprivacy.dsrPage.labelType')}</Label>
                <Select
                  value={createForm.type}
                  onValueChange={(v) => { setCreateForm((f) => ({ ...f, type: v as DSRType })); }}
                >
                  <SelectTrigger><SelectValue /></SelectTrigger>
                  <SelectContent>
                    <SelectItem value="access">{t('vaktprivacy.dsrPage.typeAccess')}</SelectItem>
                    <SelectItem value="erasure">{t('vaktprivacy.dsrPage.typeErasure')}</SelectItem>
                    <SelectItem value="portability">{t('vaktprivacy.dsrPage.typePortability')}</SelectItem>
                    <SelectItem value="objection">{t('vaktprivacy.dsrPage.typeObjection')}</SelectItem>
                    <SelectItem value="rectification">{t('vaktprivacy.dsrPage.typeRectification')}</SelectItem>
                    <SelectItem value="restriction">{t('vaktprivacy.dsrPage.typeRestriction')}</SelectItem>
                    <SelectItem value="no_profiling">{t('vaktprivacy.dsrPage.typeNoProfiling')}</SelectItem>
                  </SelectContent>
                </Select>
              </div>
              <div className="space-y-1.5">
                <Label>{t('vaktprivacy.dsrPage.labelChannel')}</Label>
                <Input
                  placeholder="z.B. E-Mail, Portal"
                  value={createForm.channel}
                  onChange={(e) => { setCreateForm((f) => ({ ...f, channel: e.target.value })); }}
                />
              </div>
            </div>
            <div className="space-y-1.5">
              <Label>{t('vaktprivacy.dsrPage.labelReferenceID')}</Label>
              <Input
                placeholder="Ticket-Nr., Fallnummer …"
                value={createForm.reference_id}
                onChange={(e) => { setCreateForm((f) => ({ ...f, reference_id: e.target.value })); }}
              />
            </div>
            <div className="space-y-1.5">
              <Label>{t('vaktprivacy.dsrPage.labelDescription')}</Label>
              <Textarea
                placeholder="Inhalt der Anfrage …"
                rows={3}
                value={createForm.description}
                onChange={(e) => { setCreateForm((f) => ({ ...f, description: e.target.value })); }}
              />
            </div>
            <div className="space-y-1.5">
              <Label>{t('vaktprivacy.dsrPage.labelInternalNotes')}</Label>
              <Textarea
                placeholder="Interne Anmerkungen …"
                rows={2}
                value={createForm.notes}
                onChange={(e) => { setCreateForm((f) => ({ ...f, notes: e.target.value })); }}
              />
            </div>
          </div>
          <DialogFooter>
            <Button variant="outline" onClick={() => { setDialogMode(null); }}>{t('common.cancel')}</Button>
            <Button onClick={handleSubmit} disabled={!canSubmitCreate}>
              {isPending ? t('vaktprivacy.dsrPage.savingPending') : t('vaktprivacy.dsrPage.createDSR')}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      {/* Edit Dialog */}
      <Dialog open={dialogMode === 'edit'} onOpenChange={(open) => { if (!open) { setDialogMode(null); } }}>
        <DialogContent className="max-w-lg">
          <DialogHeader>
            <DialogTitle>{t('vaktprivacy.dsrPage.editDialogTitle')}</DialogTitle>
          </DialogHeader>
          <div className="space-y-4 py-2">
            <div className="space-y-1.5">
              <Label>{t('vaktprivacy.dsrPage.editLabelStatus')}</Label>
              <Select
                value={editForm.status}
                onValueChange={(v) => { setEditForm((f) => ({ ...f, status: v as DSRStatus })); }}
              >
                <SelectTrigger><SelectValue /></SelectTrigger>
                <SelectContent>
                  <SelectItem value="open">{t('vaktprivacy.dsrPage.statusOpen')}</SelectItem>
                  <SelectItem value="in_progress">{t('vaktprivacy.dsrPage.statusInProgress')}</SelectItem>
                  <SelectItem value="completed">{t('vaktprivacy.dsrPage.statusCompleted')}</SelectItem>
                  <SelectItem value="rejected">{t('vaktprivacy.dsrPage.statusRejected')}</SelectItem>
                  <SelectItem value="extended">{t('vaktprivacy.dsrPage.statusExtended')}</SelectItem>
                  <SelectItem value="overdue">{t('vaktprivacy.dsrPage.statusOverdue')}</SelectItem>
                </SelectContent>
              </Select>
            </div>
            <div className="space-y-1.5">
              <Label>{t('vaktprivacy.dsrPage.labelInternalNotes')}</Label>
              <Textarea
                placeholder="Begründung, Maßnahmen, Kommentare …"
                rows={3}
                value={editForm.notes}
                onChange={(e) => { setEditForm((f) => ({ ...f, notes: e.target.value })); }}
              />
            </div>
          </div>
          <DialogFooter>
            <Button variant="outline" onClick={() => { setDialogMode(null); }}>{t('common.cancel')}</Button>
            <Button onClick={handleSubmit} disabled={isPending}>
              {isPending ? t('vaktprivacy.dsrPage.savingPending') : t('vaktprivacy.dsrPage.saveBtn')}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </div>
  )
}
