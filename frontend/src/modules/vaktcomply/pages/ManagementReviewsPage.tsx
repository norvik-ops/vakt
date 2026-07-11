// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { Plus, ChevronDown, ChevronUp, CheckCircle2, Clock } from 'lucide-react'
import { Spinner } from '../../../components/Spinner'
import { PageHeader } from '../../../shared/components/PageHeader'
import { EmptyState } from '../../../shared/components/EmptyState'
import { Button } from '../../../components/ui/button'
import { Badge } from '../../../components/ui/badge'
import { Card, CardContent, CardHeader, CardTitle } from '../../../components/ui/card'
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
  DialogFooter,
} from '../../../components/ui/dialog'
import { Input } from '../../../components/ui/input'
import { Label } from '../../../components/ui/label'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '../../../components/ui/select'
import { toast } from '../../../shared/hooks/useToast'
import {
  useManagementReviews,
  useCreateManagementReview,
  useApproveManagementReview,
  useUpdateManagementReviewInputs,
  useUpdateManagementReviewOutputs,
} from '../hooks/useManagementReviews'
import type {
  ManagementReview,
  CreateManagementReviewInput,
  UpdateManagementReviewInputsInput,
  UpdateManagementReviewOutputsInput,
} from '../types'

// ─── Constants ────────────────────────────────────────────────────────────────

const STATUS_CLASS: Record<ManagementReview['status'], string> = {
  draft: 'bg-secondary text-secondary-foreground',
  approved: 'bg-green-500/20 text-green-400 border-green-500/30',
}

const TYPE_CLASS: Record<ManagementReview['review_type'], string> = {
  annual: 'bg-blue-500/20 text-blue-400 border-blue-500/30',
  extraordinary: 'bg-amber-500/20 text-amber-400 border-amber-500/30',
}

// ─── Review Detail ─────────────────────────────────────────────────────────────

function ReviewDetail({ review }: { review: ManagementReview }) {
  const { t } = useTranslation()
  const [open, setOpen] = useState(false)
  const [inputsOpen, setInputsOpen] = useState(false)
  const [outputsOpen, setOutputsOpen] = useState(false)
  const approve = useApproveManagementReview(review.id)
  const updateInputs = useUpdateManagementReviewInputs(review.id)
  const updateOutputs = useUpdateManagementReviewOutputs(review.id)

  const [inputsForm, setInputsForm] = useState<UpdateManagementReviewInputsInput>({
    audit_findings_summary: review.audit_findings_summary,
    incident_summary: review.incident_summary,
    risk_status_summary: review.risk_status_summary,
    previous_actions_status: review.previous_actions_status,
    context_changes: review.context_changes,
    customer_feedback: review.customer_feedback,
  })

  const [outputsForm, setOutputsForm] = useState<UpdateManagementReviewOutputsInput>({
    improvement_decisions: review.improvement_decisions,
    resource_decisions: review.resource_decisions,
    isms_changes: review.isms_changes,
    next_review_date: review.next_review_date,
  })

  function handleApprove() {
    approve.mutate(undefined, {
      onSuccess: () => toast(t('vaktcomply.managementReviews.toastApproved'), 'success'),
      onError: (e) => toast(t('vaktcomply.managementReviews.toastError', { message: e.message }), 'error'),
    })
  }

  function handleSaveInputs() {
    updateInputs.mutate(inputsForm, {
      onSuccess: () => {
        setInputsOpen(false)
        toast(t('vaktcomply.managementReviews.toastInputsSaved'), 'success')
      },
      onError: (e) => toast(t('vaktcomply.managementReviews.toastError', { message: e.message }), 'error'),
    })
  }

  function handleSaveOutputs() {
    updateOutputs.mutate(outputsForm, {
      onSuccess: () => {
        setOutputsOpen(false)
        toast(t('vaktcomply.managementReviews.toastOutputsSaved'), 'success')
      },
      onError: (e) => toast(t('vaktcomply.managementReviews.toastError', { message: e.message }), 'error'),
    })
  }

  async function handleExportPDF() {
    try {
      const res = await fetch(`/api/v1/vaktcomply/management-reviews/${review.id}/export-pdf`, {
        credentials: 'include',
      })
      if (!res.ok) {
        toast(t('vaktcomply.managementReviews.toastError', { message: `HTTP ${res.status}` }), 'error')
        return
      }
      const blob = await res.blob()
      const url = URL.createObjectURL(blob)
      const a = document.createElement('a')
      a.href = url
      a.download = `vakt-management-review-${review.review_date}.pdf`
      document.body.appendChild(a)
      a.click()
      document.body.removeChild(a)
      URL.revokeObjectURL(url)
    } catch (e) {
      toast(t('vaktcomply.managementReviews.toastError', { message: String(e) }), 'error')
    }
  }

  const inputFields: [keyof UpdateManagementReviewInputsInput, string][] = [
    ['audit_findings_summary', t('vaktcomply.managementReviews.inputAuditFindings')],
    ['incident_summary', t('vaktcomply.managementReviews.inputIncidentSummary')],
    ['risk_status_summary', t('vaktcomply.managementReviews.inputRiskStatus')],
    ['previous_actions_status', t('vaktcomply.managementReviews.inputPreviousActions')],
    ['context_changes', t('vaktcomply.managementReviews.inputContextChanges')],
    ['customer_feedback', t('vaktcomply.managementReviews.inputCustomerFeedback')],
  ]

  return (
    <Card className="mb-3">
      <CardHeader className="pb-2">
        <div className="flex items-center justify-between">
          <div className="flex items-center gap-2">
            <CardTitle className="text-base">{review.review_date}</CardTitle>
            <Badge className={TYPE_CLASS[review.review_type]}>
              {review.review_type === 'annual'
                ? t('vaktcomply.managementReviews.typeAnnual')
                : t('vaktcomply.managementReviews.typeExtraordinary')}
            </Badge>
            <Badge className={STATUS_CLASS[review.status]}>
              {review.status === 'approved'
                ? t('vaktcomply.managementReviews.statusApproved')
                : t('vaktcomply.managementReviews.statusDraft')}
            </Badge>
            {review.participant_ids.length > 0 && (
              <span className="text-xs text-muted-foreground">
                {t('vaktcomply.managementReviews.participants', { count: review.participant_ids.length })}
              </span>
            )}
          </div>
          <div className="flex gap-2">
            {review.status === 'draft' && (
              <Button size="sm" variant="outline" onClick={handleApprove} disabled={approve.isPending}>
                <CheckCircle2 className="w-3 h-3 mr-1" />
                {t('vaktcomply.managementReviews.approve')}
              </Button>
            )}
            <Button size="sm" variant="ghost" onClick={() => { void handleExportPDF() }}>
              {t('vaktcomply.managementReviews.exportPdf')}
            </Button>
            <button
              className="flex items-center gap-1 text-xs text-muted-foreground hover:text-foreground"
              onClick={() => { setOpen((v) => !v); }}
            >
              {open ? <ChevronUp className="w-4 h-4" /> : <ChevronDown className="w-4 h-4" />}
            </button>
          </div>
        </div>
      </CardHeader>

      {open && (
        <CardContent className="space-y-4">
          {/* Inputs Section */}
          <div>
            <div className="flex items-center justify-between mb-2">
              <h4 className="text-sm font-medium">{t('vaktcomply.managementReviews.sectionInputs')}</h4>
              <Button size="sm" variant="outline" onClick={() => { setInputsOpen(true); }}>
                {t('vaktcomply.managementReviews.edit')}
              </Button>
            </div>
            <div className="grid grid-cols-2 gap-2 text-sm">
              <div>
                <span className="text-muted-foreground">{t('vaktcomply.managementReviews.inputAuditFindings')}:</span>
                <p className="mt-0.5">{review.audit_findings_summary || '—'}</p>
              </div>
              <div>
                <span className="text-muted-foreground">{t('vaktcomply.managementReviews.inputIncidentSummary')}:</span>
                <p className="mt-0.5">{review.incident_summary || '—'}</p>
              </div>
              <div>
                <span className="text-muted-foreground">{t('vaktcomply.managementReviews.inputRiskStatus')}:</span>
                <p className="mt-0.5">{review.risk_status_summary || '—'}</p>
              </div>
              <div>
                <span className="text-muted-foreground">{t('vaktcomply.managementReviews.inputPreviousActions')}:</span>
                <p className="mt-0.5">{review.previous_actions_status || '—'}</p>
              </div>
              <div>
                <span className="text-muted-foreground">{t('vaktcomply.managementReviews.inputContextChanges')}:</span>
                <p className="mt-0.5">{review.context_changes || '—'}</p>
              </div>
              <div>
                <span className="text-muted-foreground">{t('vaktcomply.managementReviews.inputCustomerFeedback')}:</span>
                <p className="mt-0.5">{review.customer_feedback || '—'}</p>
              </div>
            </div>
          </div>

          {/* Outputs Section */}
          <div>
            <div className="flex items-center justify-between mb-2">
              <h4 className="text-sm font-medium">{t('vaktcomply.managementReviews.sectionOutputs')}</h4>
              <Button size="sm" variant="outline" onClick={() => { setOutputsOpen(true); }}>
                {t('vaktcomply.managementReviews.edit')}
              </Button>
            </div>
            <div className="grid grid-cols-2 gap-2 text-sm">
              <div>
                <span className="text-muted-foreground">{t('vaktcomply.managementReviews.outputImprovementDecisions')}:</span>
                <p className="mt-0.5">
                  {review.improvement_decisions.length > 0
                    ? t('vaktcomply.managementReviews.outputDecisionsCount', { count: review.improvement_decisions.length })
                    : '—'}
                </p>
              </div>
              <div>
                <span className="text-muted-foreground">{t('vaktcomply.managementReviews.outputResourceDecisions')}:</span>
                <p className="mt-0.5">{review.resource_decisions || '—'}</p>
              </div>
              <div>
                <span className="text-muted-foreground">{t('vaktcomply.managementReviews.outputIsmsChanges')}:</span>
                <p className="mt-0.5">{review.isms_changes || '—'}</p>
              </div>
              {review.next_review_date && (
                <div>
                  <span className="text-muted-foreground">{t('vaktcomply.managementReviews.outputNextReview')}:</span>
                  <p className="mt-0.5">{review.next_review_date}</p>
                </div>
              )}
            </div>
          </div>
        </CardContent>
      )}

      {/* Inputs Edit Dialog */}
      <Dialog open={inputsOpen} onOpenChange={setInputsOpen}>
        <DialogContent className="max-w-lg">
          <DialogHeader>
            <DialogTitle>{t('vaktcomply.managementReviews.inputsDialogTitle')}</DialogTitle>
          </DialogHeader>
          <div className="space-y-3">
            {inputFields.map(([field, label]) => (
              <div key={field}>
                <Label>{label}</Label>
                <Input
                  value={(inputsForm[field] as string) || ''}
                  onChange={(e) => { setInputsForm((f) => ({ ...f, [field]: e.target.value })); }}
                />
              </div>
            ))}
          </div>
          <DialogFooter>
            <Button variant="outline" onClick={() => { setInputsOpen(false); }}>
              {t('vaktcomply.managementReviews.cancel')}
            </Button>
            <Button onClick={handleSaveInputs} disabled={updateInputs.isPending}>
              {t('vaktcomply.managementReviews.save')}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      {/* Outputs Edit Dialog */}
      <Dialog open={outputsOpen} onOpenChange={setOutputsOpen}>
        <DialogContent className="max-w-lg">
          <DialogHeader>
            <DialogTitle>{t('vaktcomply.managementReviews.outputsDialogTitle')}</DialogTitle>
          </DialogHeader>
          <div className="space-y-3">
            <div>
              <Label>{t('vaktcomply.managementReviews.outputResourceDecisions')}</Label>
              <Input
                value={outputsForm.resource_decisions}
                onChange={(e) => { setOutputsForm((f) => ({ ...f, resource_decisions: e.target.value })); }}
              />
            </div>
            <div>
              <Label>{t('vaktcomply.managementReviews.outputIsmsChanges')}</Label>
              <Input
                value={outputsForm.isms_changes}
                onChange={(e) => { setOutputsForm((f) => ({ ...f, isms_changes: e.target.value })); }}
              />
            </div>
            <div>
              <Label>{t('vaktcomply.managementReviews.outputNextReview')}</Label>
              <Input
                type="date"
                value={outputsForm.next_review_date ?? ''}
                onChange={(e) => { setOutputsForm((f) => ({ ...f, next_review_date: e.target.value || undefined })); }}
              />
            </div>
          </div>
          <DialogFooter>
            <Button variant="outline" onClick={() => { setOutputsOpen(false); }}>
              {t('vaktcomply.managementReviews.cancel')}
            </Button>
            <Button onClick={handleSaveOutputs} disabled={updateOutputs.isPending}>
              {t('vaktcomply.managementReviews.save')}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </Card>
  )
}

// ─── Main Page ─────────────────────────────────────────────────────────────────

export default function ManagementReviewsPage() {
  const { t } = useTranslation()
  const { data: reviews, isLoading } = useManagementReviews()
  const createReview = useCreateManagementReview()

  const [createOpen, setCreateOpen] = useState(false)
  const [createForm, setCreateForm] = useState<CreateManagementReviewInput>({
    review_date: new Date().toISOString().split('T')[0],
    review_type: 'annual',
  })

  function handleCreate() {
    createReview.mutate(createForm, {
      onSuccess: () => {
        setCreateOpen(false)
        setCreateForm({ review_date: new Date().toISOString().split('T')[0], review_type: 'annual' })
        toast(t('vaktcomply.managementReviews.toastCreated'), 'success')
      },
      onError: (e) => toast(t('vaktcomply.managementReviews.toastError', { message: e.message }), 'error'),
    })
  }

  if (isLoading) {
    return (
      <div className="flex justify-center py-12">
        <Spinner />
      </div>
    )
  }

  return (
    <div className="space-y-6">
      <PageHeader
        title={t('vaktcomply.managementReviews.pageTitle')}
        description={t('vaktcomply.managementReviews.pageDescription')}
        actions={
          <Button onClick={() => { setCreateOpen(true); }}>
            <Plus className="w-4 h-4 mr-1" />
            {t('vaktcomply.managementReviews.newReview')}
          </Button>
        }
      />

      {(!reviews || reviews.length === 0) ? (
        <EmptyState
          icon={Clock}
          title={t('vaktcomply.managementReviews.emptyTitle')}
          description={t('vaktcomply.managementReviews.emptyDescription')}
          action={
            <Button onClick={() => { setCreateOpen(true); }}>
              <Plus className="w-4 h-4 mr-1" />
              {t('vaktcomply.managementReviews.newReview')}
            </Button>
          }
        />
      ) : (
        <div>
          {reviews.map((review) => (
            <ReviewDetail key={review.id} review={review} />
          ))}
        </div>
      )}

      {/* Create Dialog */}
      <Dialog open={createOpen} onOpenChange={setCreateOpen}>
        <DialogContent className="max-w-md">
          <DialogHeader>
            <DialogTitle>{t('vaktcomply.managementReviews.createDialogTitle')}</DialogTitle>
          </DialogHeader>
          <div className="space-y-4">
            <div>
              <Label htmlFor="review_date">{t('vaktcomply.managementReviews.labelDate')}</Label>
              <Input
                id="review_date"
                type="date"
                value={createForm.review_date}
                onChange={(e) => { setCreateForm((f) => ({ ...f, review_date: e.target.value })); }}
              />
            </div>
            <div>
              <Label htmlFor="review_type">{t('vaktcomply.managementReviews.labelType')}</Label>
              <Select
                value={createForm.review_type}
                onValueChange={(v) => {
                  setCreateForm((f) => ({ ...f, review_type: v as 'annual' | 'extraordinary' }))
                }}
              >
                <SelectTrigger id="review_type">
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="annual">{t('vaktcomply.managementReviews.typeAnnualPlanned')}</SelectItem>
                  <SelectItem value="extraordinary">{t('vaktcomply.managementReviews.typeExtraordinaryOption')}</SelectItem>
                </SelectContent>
              </Select>
            </div>
          </div>
          <DialogFooter>
            <Button variant="outline" onClick={() => { setCreateOpen(false); }}>
              {t('vaktcomply.managementReviews.cancel')}
            </Button>
            <Button onClick={handleCreate} disabled={createReview.isPending || !createForm.review_date}>
              {t('vaktcomply.managementReviews.startReview')}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </div>
  )
}
