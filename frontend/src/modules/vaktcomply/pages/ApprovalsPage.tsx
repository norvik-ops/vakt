import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { CheckCircle2, XCircle, ShieldCheck } from 'lucide-react'
import { Spinner } from '../../../components/Spinner'
import { PageHeader } from '../../../shared/components/PageHeader'
import { EmptyState } from '../../../shared/components/EmptyState'
import { Button } from '../../../components/ui/button'
import { Badge } from '../../../components/ui/badge'
import { Card } from '../../../components/ui/card'
import { Dialog, DialogContent, DialogHeader, DialogTitle, DialogFooter } from '../../../components/ui/dialog'
import { Label } from '../../../components/ui/label'
import { Textarea } from '../../../components/ui/textarea'
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '../../../components/ui/table'
import { useAuthStore } from '../../../shared/stores/auth'
import { toast } from '../../../shared/hooks/useToast'
import { handleApiError } from '../../../shared/utils/errorMessages'
import { useFormatDate } from '../../../shared/hooks/useFormatDate'
import {
  usePendingApprovals, useApproveApproval, useRejectApproval,
  type ApprovalWithDetails,
} from '../hooks/useApprovals'

// ─── Status label helpers ─────────────────────────────────────────────────────

function useStatusLabel() {
  const { t } = useTranslation()
  const STATUS_LABEL: Record<string, string> = {
    missing:        t('vaktcomply.approvalsPage.statusMissing'),
    in_progress:    t('vaktcomply.approvalsPage.statusInProgress'),
    implemented:    t('vaktcomply.approvalsPage.statusImplemented'),
    not_applicable: t('vaktcomply.approvalsPage.statusNotApplicable'),
    covered:        t('vaktcomply.approvalsPage.statusCovered'),
    partial:        t('vaktcomply.approvalsPage.statusPartial'),
  }
  return (s: string) => STATUS_LABEL[s] ?? s
}

// ─── Review dialog ────────────────────────────────────────────────────────────

interface ReviewDialogProps {
  approval: ApprovalWithDetails | null
  mode: 'approve' | 'reject'
  onClose: () => void
}

function ReviewDialog({ approval, mode, onClose }: ReviewDialogProps) {
  const { t } = useTranslation()
  const [comment, setComment] = useState('')
  const approve = useApproveApproval()
  const reject = useRejectApproval()
  const statusLabel = useStatusLabel()

  if (!approval) return null

  function handleSubmit() {
    if (!approval) return
    const mutation = mode === 'approve' ? approve : reject
    mutation.mutate(
      { id: approval.id, comment },
      {
        onSuccess: () => {
          toast(
            mode === 'approve' ? t('vaktcomply.approvalsPage.approvedToast') : t('vaktcomply.approvalsPage.rejectedToast'),
            mode === 'approve' ? 'success' : 'error',
          )
          setComment('')
          onClose()
        },
        onError: (err) => toast(handleApiError(err), 'error'),
      },
    )
  }

  const isPending = approve.isPending || reject.isPending

  return (
    <Dialog open onOpenChange={(v) => { if (!v) onClose() }}>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>
            {mode === 'approve' ? t('vaktcomply.approvalsPage.approveTitle') : t('vaktcomply.approvalsPage.rejectTitle')}
          </DialogTitle>
        </DialogHeader>
        <div className="space-y-4 py-2">
          <div className="text-sm text-secondary space-y-1">
            <p><span className="font-medium text-primary">Control:</span> {approval.control_ref} — {approval.control_title}</p>
            <p><span className="font-medium text-primary">{t('vaktcomply.approvalsPage.requestedBy')}:</span> {approval.requester_name || approval.requester_email}</p>
            <p>
              <span className="font-medium text-primary">{t('vaktcomply.approvalsPage.statusChange')}:</span>{' '}
              <span className="text-secondary">{statusLabel(approval.current_status)}</span>
              {' '}&rarr;{' '}
              <span className="text-primary font-medium">{statusLabel(approval.requested_status)}</span>
            </p>
            {approval.comment && (
              <p><span className="font-medium text-primary">{t('vaktcomply.approvalsPage.reason')}:</span> {approval.comment}</p>
            )}
          </div>
          <div className="space-y-1.5">
            <Label className="text-xs">{t('vaktcomply.approvalsPage.commentLabel')}</Label>
            <Textarea
              value={comment}
              onChange={(e) => { setComment(e.target.value); }}
              placeholder={mode === 'approve' ? t('vaktcomply.approvalsPage.commentPlaceholderApprove') : t('vaktcomply.approvalsPage.commentPlaceholderReject')}
              rows={3}
            />
          </div>
        </div>
        <DialogFooter>
          <Button variant="outline" onClick={onClose} disabled={isPending}>{t('common.cancel')}</Button>
          <Button
            variant={mode === 'approve' ? 'default' : 'destructive'}
            onClick={handleSubmit}
            disabled={isPending}
          >
            {isPending
              ? t('vaktcomply.approvalsPage.saving')
              : mode === 'approve' ? t('vaktcomply.approvalsPage.approve') : t('vaktcomply.approvalsPage.reject')}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}

// ─── Main page ────────────────────────────────────────────────────────────────

export default function ApprovalsPage() {
  const { t } = useTranslation()
  const { user } = useAuthStore()
  const { formatDate } = useFormatDate()
  const isAdmin = user?.roles?.includes('Admin') ?? false
  const statusLabel = useStatusLabel()

  const { data: approvals = [], isLoading } = usePendingApprovals()
  const [selected, setSelected] = useState<ApprovalWithDetails | null>(null)
  const [reviewMode, setReviewMode] = useState<'approve' | 'reject'>('approve')

  if (!isAdmin) {
    return (
      <div className="p-6">
        <EmptyState
          icon={ShieldCheck}
          title={t('vaktcomply.approvalsPage.noPermissionTitle')}
          description={t('vaktcomply.approvalsPage.noPermissionDescription')}
        />
      </div>
    )
  }

  function openReview(approval: ApprovalWithDetails, mode: 'approve' | 'reject') {
    setSelected(approval)
    setReviewMode(mode)
  }

  return (
    <div className="p-6 space-y-6">
      <PageHeader
        title={t('vaktcomply.approvalsPage.title')}
        description={t('vaktcomply.approvalsPage.description')}
      />

      {isLoading ? (
        <div className="flex justify-center py-12">
          <Spinner size="lg" />
        </div>
      ) : approvals.length === 0 ? (
        <EmptyState
          icon={CheckCircle2}
          title={t('vaktcomply.approvalsPage.noRequestsTitle')}
          description={t('vaktcomply.approvalsPage.noRequestsDescription')}
        />
      ) : (
        <Card>
          <div className="overflow-x-auto">
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>Control</TableHead>
                <TableHead>{t('vaktcomply.approvalsPage.colRequestedBy')}</TableHead>
                <TableHead>{t('vaktcomply.approvalsPage.colCurrentStatus')}</TableHead>
                <TableHead>{t('vaktcomply.approvalsPage.colRequestedStatus')}</TableHead>
                <TableHead>{t('vaktcomply.approvalsPage.colReason')}</TableHead>
                <TableHead>{t('vaktcomply.approvalsPage.colDate')}</TableHead>
                <TableHead className="text-right">{t('common.actions')}</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {approvals.map((a) => (
                <TableRow key={a.id}>
                  <TableCell>
                    <div className="space-y-0.5">
                      <div className="text-xs text-secondary font-mono">{a.control_ref}</div>
                      <div className="text-sm font-medium text-primary max-w-[220px] truncate" title={a.control_title}>
                        {a.control_title}
                      </div>
                    </div>
                  </TableCell>
                  <TableCell>
                    <div className="text-sm">{a.requester_name || a.requester_email}</div>
                  </TableCell>
                  <TableCell>
                    <Badge variant="secondary">{statusLabel(a.current_status)}</Badge>
                  </TableCell>
                  <TableCell>
                    <Badge variant="outline" className="border-brand/40 text-brand">
                      {statusLabel(a.requested_status)}
                    </Badge>
                  </TableCell>
                  <TableCell>
                    <span className="text-sm text-secondary line-clamp-2 max-w-[180px]">
                      {a.comment || '—'}
                    </span>
                  </TableCell>
                  <TableCell>
                    <span className="text-xs text-secondary">
                      {formatDate(a.created_at)}
                    </span>
                  </TableCell>
                  <TableCell className="text-right">
                    <div className="flex items-center justify-end gap-2">
                      <Button
                        size="sm"
                        variant="outline"
                        className="text-green-600 border-green-600/30 hover:bg-green-600/10"
                        onClick={() => { openReview(a, 'approve'); }}
                      >
                        <CheckCircle2 className="w-3.5 h-3.5 mr-1" />
                        {t('vaktcomply.approvalsPage.approve')}
                      </Button>
                      <Button
                        size="sm"
                        variant="outline"
                        className="text-red-500 border-red-500/30 hover:bg-red-500/10"
                        onClick={() => { openReview(a, 'reject'); }}
                      >
                        <XCircle className="w-3.5 h-3.5 mr-1" />
                        {t('vaktcomply.approvalsPage.reject')}
                      </Button>
                    </div>
                  </TableCell>
                </TableRow>
              ))}
            </TableBody>
          </Table>
          </div>
        </Card>
      )}

      <ReviewDialog
        approval={selected}
        mode={reviewMode}
        onClose={() => { setSelected(null); }}
      />
    </div>
  )
}
