import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { Plus, ShieldCheck, AlertTriangle, Eye } from 'lucide-react'
import { Button } from '../../../components/ui/button'
import { Badge } from '../../../components/ui/badge'
import { Dialog, DialogContent, DialogHeader, DialogTitle, DialogFooter } from '../../../components/ui/dialog'
import { apiFetch } from '../../../api/client'
import { EmptyState } from '../../../shared/components/EmptyState'
import { ProGate } from '../../../shared/components/ProGate'
import { SkeletonTable } from '../../../shared/components/SkeletonLoaders'
import { PageHeader } from '../../../shared/components/PageHeader'
import type { AccessReview, AccessReviewDetail, AccessReviewItem, ReviewDecision } from '../types'

function ReviewRow({
  review,
  onOpen,
}: {
  review: AccessReview
  onOpen: (review: AccessReview) => void
}) {
  const isCompleted = review.status === 'completed'
  return (
    <div className="flex items-center justify-between px-4 py-3 bg-surface border border-border rounded-lg gap-3">
      <div className="flex items-center gap-3 min-w-0">
        <ShieldCheck className="w-4 h-4 text-secondary shrink-0" />
        <div className="min-w-0">
          <p className="text-sm font-medium">{review.period_label}</p>
          <p className="text-xs text-secondary">
            {review.total_entries} Secrets · {review.stale_entries} veraltet
            {isCompleted && ` · ${review.revoked_entries} widerrufen`}
          </p>
        </div>
      </div>
      <div className="flex items-center gap-2 shrink-0">
        <Badge variant={isCompleted ? 'success' : 'warning'} className="text-xs">
          {isCompleted ? 'Abgeschlossen' : 'Offen'}
        </Badge>
        {!isCompleted && (
          <Button size="sm" variant="outline" onClick={() => { onOpen(review); }}>
            <Eye className="w-3 h-3 mr-1" /> Review starten
          </Button>
        )}
      </div>
    </div>
  )
}

export default function AccessReviewsPage() {
  const { t } = useTranslation()
  const qc = useQueryClient()
  const [selectedReview, setSelectedReview] = useState<AccessReview | null>(null)
  const [decisions, setDecisions] = useState<Record<string, 'keep' | 'revoke'>>({})

  const { data: reviews, isLoading, isError, error } = useQuery<AccessReview[]>({
    queryKey: ['vault', 'access-reviews'],
    queryFn: () => apiFetch('/vaktvault/access-reviews'),
  })

  const { data: reviewDetail } = useQuery<AccessReviewDetail>({
    queryKey: ['vault', 'access-review', selectedReview?.id],
    queryFn: () => apiFetch(`/vaktvault/access-reviews/${selectedReview!.id}`),
    enabled: !!selectedReview,
  })

  const createMutation = useMutation({
    mutationFn: () => apiFetch('/vaktvault/access-reviews', { method: 'POST' }),
    onSuccess: () => qc.invalidateQueries({ queryKey: ['vault', 'access-reviews'] }),
  })

  const completeMutation = useMutation({
    mutationFn: ({ id, decs }: { id: string; decs: ReviewDecision[] }) =>
      apiFetch(`/vaktvault/access-reviews/${id}/complete`, {
        method: 'POST',
        body: JSON.stringify({ decisions: decs }),
      }),
    onSuccess: () => {
      void qc.invalidateQueries({ queryKey: ['vault', 'access-reviews'] })
      setSelectedReview(null)
      setDecisions({})
    },
  })

  const handleComplete = () => {
    if (!selectedReview || !reviewDetail) return
    const decs: ReviewDecision[] = reviewDetail.items.map((item: AccessReviewItem) => ({
      env_id: item.env_id,
      secret_key: item.secret_key,
      action: decisions[`${item.env_id}:${item.secret_key}`] ?? 'keep',
    }))
    completeMutation.mutate({ id: selectedReview.id, decs })
  }

  return (
    <ProGate error={isError ? error : null}>
    <div className="space-y-6">
      <PageHeader
        title="Quartalsweise Zugriffsreviews"
        description="Überprüfen Sie vierteljährlich, welche Secrets noch benötigt werden und widerrufen Sie veraltete Zugänge."
        actions={
          <Button onClick={() => { createMutation.mutate(); }} disabled={createMutation.isPending}>
            <Plus className="w-4 h-4 mr-2" />
            {createMutation.isPending ? 'Erstellen…' : 'Review starten'}
          </Button>
        }
      />

      {isLoading && <SkeletonTable rows={3} />}

      {!isLoading && !reviews?.length && (
        <EmptyState
          icon={ShieldCheck}
          title="Noch keine Access-Reviews"
          description="Starten Sie Ihren ersten quartalsweisen Zugriffsreview, um veraltete Secrets zu identifizieren."
        />
      )}

      {reviews?.map(r => (
        <ReviewRow key={r.id} review={r} onOpen={setSelectedReview} />
      ))}

      <Dialog open={!!selectedReview} onOpenChange={open => { if (!open) { setSelectedReview(null); setDecisions({}) } }}>
        <DialogContent className="max-w-2xl">
          <DialogHeader>
            <DialogTitle>Access Review: {selectedReview?.period_label}</DialogTitle>
          </DialogHeader>
          {reviewDetail && (
            <div className="space-y-3 mt-2 max-h-[50vh] overflow-y-auto pr-1">
              {reviewDetail.items.length === 0 && (
                <p className="text-sm text-secondary text-center py-4">Keine Secrets gefunden.</p>
              )}
              {reviewDetail.items.map(item => {
                const key = `${item.env_id}:${item.secret_key}`
                const decision = decisions[key] ?? 'keep'
                return (
                  <div
                    key={key}
                    className="flex items-center justify-between px-3 py-2 bg-surface border border-border rounded gap-2"
                  >
                    <div className="min-w-0">
                      <p className="text-sm font-mono font-medium truncate">{item.secret_key}</p>
                      <p className="text-xs text-secondary">
                        {item.project_name && <span>{item.project_name} · </span>}
                        {item.last_accessed_at
                          ? `Zuletzt: ${new Date(item.last_accessed_at).toLocaleDateString('de-DE')}`
                          : 'Nie zugegriffen'}
                        {item.is_stale && (
                          <span className="ml-1 text-warning">
                            <AlertTriangle className="inline w-3 h-3 -mt-0.5 mr-0.5" />veraltet
                          </span>
                        )}
                      </p>
                    </div>
                    <div className="flex gap-1 shrink-0">
                      <Button
                        size="sm"
                        variant={decision === 'keep' ? 'default' : 'outline'}
                        onClick={() => { setDecisions(d => ({ ...d, [key]: 'keep' })); }}
                      >
                        Behalten
                      </Button>
                      <Button
                        size="sm"
                        variant={decision === 'revoke' ? 'destructive' : 'outline'}
                        onClick={() => { setDecisions(d => ({ ...d, [key]: 'revoke' })); }}
                      >
                        Widerrufen
                      </Button>
                    </div>
                  </div>
                )
              })}
            </div>
          )}
          <DialogFooter>
            <Button variant="outline" onClick={() => { setSelectedReview(null); setDecisions({}) }}>{t('common.cancel')}</Button>
            <Button onClick={handleComplete} disabled={completeMutation.isPending}>
              {completeMutation.isPending ? 'Wird abgeschlossen…' : 'Review abschließen'}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </div>
    </ProGate>
  )
}
