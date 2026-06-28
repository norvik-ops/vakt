import { useState } from 'react'
import { useParams, Link } from 'react-router-dom'
import { CheckCircle, AlertTriangle } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { Button } from '../../../components/ui/button'
import { Badge } from '../../../components/ui/badge'
import { Card, CardContent, CardHeader, CardTitle } from '../../../components/ui/card'
import { Textarea } from '../../../components/ui/textarea'
import { useAssessmentAnswers, useReviewAnswer, useFinalizeAssessment } from '../hooks/useAssessments'
import type { AnswerWithReview } from '../types'

export function AssessmentReviewView() {
  const { t } = useTranslation()
  const { id: assessmentId = '' } = useParams<{ id: string }>()
  const { data: answers = [], isLoading } = useAssessmentAnswers(assessmentId)
  const reviewMutation = useReviewAnswer(assessmentId)
  const finalizeMutation = useFinalizeAssessment()

  const [reworkDialog, setReworkDialog] = useState<{ answerId: string; note: string } | null>(null)

  const allReviewed = answers.length > 0 && answers.every((a) => a.review_status != null)

  function handleAccept(answerId: string) {
    reviewMutation.mutate({ answerId, input: { review_status: 'accepted' } })
  }

  function handleReworkConfirm() {
    if (!reworkDialog) return
    reviewMutation.mutate({
      answerId: reworkDialog.answerId,
      input: { review_status: 'needs_rework', rework_note: reworkDialog.note },
    })
    setReworkDialog(null)
  }

  function handleFinalize() {
    finalizeMutation.mutate(assessmentId)
  }

  if (isLoading) return <p className="p-4 text-muted-foreground">{t('vaktcomply.assessmentReview.loading')}</p>

  return (
    <div className="p-6 space-y-4">
      <div className="flex items-center justify-between">
        <h1 className="text-xl font-bold">{t('vaktcomply.assessmentReview.reviewTitle')}</h1>
        {allReviewed && (
          <Button onClick={handleFinalize} disabled={finalizeMutation.isPending}>
            <CheckCircle className="mr-2 h-4 w-4" />
            {t('vaktcomply.assessmentReview.finalize')}
          </Button>
        )}
      </div>

      {answers.length === 0 && (
        <p className="text-muted-foreground">{t('vaktcomply.assessmentReview.noAnswers')}</p>
      )}

      {answers.map((answer: AnswerWithReview) => (
        <Card key={answer.id}>
          <CardHeader className="pb-2">
            <CardTitle className="text-sm font-medium">{answer.question_text}</CardTitle>
          </CardHeader>
          <CardContent className="space-y-2">
            <p className="text-sm">
              {answer.answer_text || (answer.file_url ? <a href={answer.file_url} className="underline text-blue-600" target="_blank" rel="noreferrer">{t('vaktcomply.assessmentReview.viewFile')}</a> : '—')}
            </p>

            <div className="flex items-center gap-2 flex-wrap">
              {answer.review_status == null ? (
                <>
                  <Button
                    size="sm"
                    variant="outline"
                    className="text-green-700 border-green-300"
                    onClick={() => { handleAccept(answer.id); }}
                    disabled={reviewMutation.isPending}
                  >
                    <CheckCircle className="mr-1 h-3 w-3" />
                    {t('vaktcomply.assessmentReview.accept')}
                  </Button>
                  <Button
                    size="sm"
                    variant="outline"
                    className="text-yellow-700 border-yellow-300"
                    onClick={() => { setReworkDialog({ answerId: answer.id, note: '' }); }}
                  >
                    <AlertTriangle className="mr-1 h-3 w-3" />
                    {t('vaktcomply.assessmentReview.needsRework')}
                  </Button>
                </>
              ) : (
                <ReviewStatusBadge status={answer.review_status} />
              )}

              {answer.evidence_id && (
                <Link to={`/vaktcomply/controls/${answer.control_id}`}>
                  <Badge variant="outline" className="text-blue-600 border-blue-300">
                    {t('vaktcomply.assessmentReview.evidenceCreated')}
                  </Badge>
                </Link>
              )}
            </div>

            {answer.rework_note && (
              <p className="text-xs text-yellow-700 bg-yellow-50 p-2 rounded">{answer.rework_note}</p>
            )}
          </CardContent>
        </Card>
      ))}

      {reworkDialog && (
        <div className="fixed inset-0 bg-black/40 flex items-center justify-center z-50">
          <div className="bg-white rounded-lg p-6 w-full max-w-md space-y-4">
            <h2 className="font-semibold">{t('vaktcomply.assessmentReview.reworkRequired')}</h2>
            <Textarea
              placeholder={t('vaktcomply.assessmentReview.reworkNotePlaceholder')}
              value={reworkDialog.note}
              onChange={(e) => { setReworkDialog({ ...reworkDialog, note: e.target.value }); }}
            />
            <div className="flex justify-end gap-2">
              <Button variant="ghost" onClick={() => { setReworkDialog(null); }}>{t('common.cancel')}</Button>
              <Button onClick={handleReworkConfirm} disabled={reviewMutation.isPending}>
                {t('vaktcomply.assessmentReview.confirmRework')}
              </Button>
            </div>
          </div>
        </div>
      )}
    </div>
  )
}

function ReviewStatusBadge({ status }: { status: string }) {
  const { t } = useTranslation()
  if (status === 'accepted') {
    return <Badge className="bg-green-100 text-green-800 border-green-300">{t('vaktcomply.assessmentReview.accepted')}</Badge>
  }
  return <Badge className="bg-yellow-100 text-yellow-800 border-yellow-300">{t('vaktcomply.assessmentReview.needsRework')}</Badge>
}
