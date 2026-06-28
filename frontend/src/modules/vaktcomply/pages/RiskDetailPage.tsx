import { useState, useEffect } from 'react'
import { useParams, useNavigate } from 'react-router-dom'
import { ArrowLeft, Save, Sparkles, Loader2 } from 'lucide-react'
import { Spinner } from '../../../components/Spinner'
import { PageHeader } from '../../../shared/components/PageHeader'
import { Breadcrumbs } from '../../../shared/components/Breadcrumbs'
import { trackPage } from '../../../shared/hooks/useRecentPages'
import { Button } from '../../../components/ui/button'
import { Badge } from '../../../components/ui/badge'
import { Card, CardContent, CardHeader, CardTitle } from '../../../components/ui/card'
import { Input } from '../../../components/ui/input'
import { Label } from '../../../components/ui/label'
import { Textarea } from '../../../components/ui/textarea'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '../../../components/ui/select'
import { useTranslation } from 'react-i18next'
import { useRisk, useUpdateRisk, useUpdateRiskResidual, useAcceptRisk } from '../hooks/useRisks'
import { useRiskNarrative } from '../hooks/useAIInsights'
import { AIDisclaimer } from '../../../shared/components/AIDisclaimer'
import RiskTreatmentPanel from '../components/RiskTreatmentPanel'
import type { Risk, UpdateRiskInput } from '../types'
import { useFormatDate } from '../../../shared/hooks/useFormatDate'
import { TermTooltip } from '../../../shared/components/TermTooltip'

const SCORE_COLOR = (score: number) => {
  if (score >= 15) return 'bg-red-500/20 text-red-400 border-red-500/30'
  if (score >= 9)  return 'bg-amber-500/20 text-amber-400 border-amber-500/30'
  if (score >= 4)  return 'bg-yellow-500/20 text-yellow-400 border-yellow-500/30'
  return 'bg-green-500/20 text-green-400 border-green-500/30'
}

function AIRiskNarrativePanel({ riskId, existingNarrative }: { riskId: string; existingNarrative: string | null }) {
  const generate = useRiskNarrative(riskId)
  const { t } = useTranslation()

  return (
    <Card>
      <CardHeader>
        <CardTitle className="text-sm flex items-center gap-2">
          <Sparkles className="w-4 h-4 text-brand" />{t('vaktcomply.riskDetail.aiNarrative')}
        </CardTitle>
      </CardHeader>
      <CardContent className="space-y-3">
        {existingNarrative && (
          <>
            <AIDisclaimer />
            <p className="text-xs text-primary leading-relaxed whitespace-pre-wrap">{existingNarrative}</p>
          </>
        )}
        {generate.isError && (
          <p className="text-xs text-red-400">{generate.error?.message ?? t('vaktcomply.riskDetail.generateError')}</p>
        )}
        <button
          onClick={() => { generate.mutate(); }}
          disabled={generate.isPending}
          className="inline-flex items-center gap-1.5 text-xs text-brand hover:text-brand/80 disabled:opacity-50 disabled:cursor-not-allowed transition-colors"
        >
          {generate.isPending
            ? <><Loader2 className="w-3 h-3 animate-spin" /> {t('vaktcomply.riskDetail.generating')}</>
            : <><Sparkles className="w-3 h-3" />{existingNarrative ? t('vaktcomply.riskDetail.regenerateBtn') : t('vaktcomply.riskDetail.generateBtn')}</>
          }
        </button>
      </CardContent>
    </Card>
  )
}

function residualScoreColor(score: number | undefined): string {
  if (score === undefined) return 'bg-muted text-muted-foreground border-border'
  if (score > 12) return 'bg-red-500/20 text-red-400 border-red-500/30'
  if (score >= 6)  return 'bg-amber-500/20 text-amber-400 border-amber-500/30'
  return 'bg-green-500/20 text-green-400 border-green-500/30'
}

export default function RiskDetailPage() {
  const { id } = useParams<{ id: string }>()
  const navigate = useNavigate()
  const { formatDate } = useFormatDate()
  const { t } = useTranslation()
  const { data: risk, isLoading, isError } = useRisk(id ?? '')
  const update = useUpdateRisk(id ?? '')
  const updateResidual = useUpdateRiskResidual(id ?? '')
  const acceptRisk = useAcceptRisk(id ?? '')

  const [form, setForm] = useState<UpdateRiskInput | null>(null)
  const [dirty, setDirty] = useState(false)

  // Residual form state
  const [residualForm, setResidualForm] = useState<{
    inherent_likelihood: string; inherent_impact: string;
    residual_likelihood: string; residual_impact: string;
  }>({ inherent_likelihood: '', inherent_impact: '', residual_likelihood: '', residual_impact: '' })
  const [showAcceptDialog, setShowAcceptDialog] = useState(false)
  const [justification, setJustification] = useState('')

  const TREATMENT_LABELS: Record<Risk['treatment'], string> = {
    avoid: t('vaktcomply.riskDetail.treatmentAvoid'),
    mitigate: t('vaktcomply.riskDetail.treatmentMitigate'),
    transfer: t('vaktcomply.riskDetail.treatmentTransfer'),
    accept: t('vaktcomply.riskDetail.treatmentAccept'),
  }
  const STATUS_LABELS: Record<Risk['status'], string> = {
    open: t('vaktcomply.risksPage.statusOpen'),
    mitigated: t('vaktcomply.risksPage.statusMitigated'),
    accepted: t('vaktcomply.risksPage.statusAccepted'),
    closed: t('vaktcomply.risksPage.statusClosed'),
  }

  useEffect(() => {
    if (risk) trackPage(`/vaktcomply/risks/${id}`, risk.title, '⚠️')
  }, [risk?.id])

  useEffect(() => {
    if (risk && !form) {
      setForm({
        title: risk.title,
        description: risk.description ?? '',
        category: risk.category ?? '',
        likelihood: risk.likelihood,
        impact: risk.impact,
        owner: risk.owner ?? '',
        status: risk.status,
        treatment: risk.treatment,
        treatment_notes: risk.treatment_notes ?? '',
      })
      setResidualForm({
        inherent_likelihood: risk.inherent_likelihood !== undefined ? String(risk.inherent_likelihood) : '',
        inherent_impact: risk.inherent_impact !== undefined ? String(risk.inherent_impact) : '',
        residual_likelihood: risk.residual_likelihood !== null && risk.residual_likelihood !== undefined ? String(risk.residual_likelihood) : '',
        residual_impact: risk.residual_impact !== null && risk.residual_impact !== undefined ? String(risk.residual_impact) : '',
      })
    }
  }, [risk, form])

  function set<K extends keyof UpdateRiskInput>(key: K, value: UpdateRiskInput[K]) {
    setForm((f) => f ? { ...f, [key]: value } : f)
    setDirty(true)
  }

  function handleSave() {
    if (!form) return
    update.mutate(form, { onSuccess: () => { setDirty(false); } })
  }

  if (isLoading) return (
    <div className="flex items-center justify-center h-48">
      <Spinner size="lg" color="primary" />
    </div>
  )
  if (isError || !risk) return (
    <div className="p-6 text-sm text-red-400">{t('vaktcomply.riskDetail.notFound')}</div>
  )

  const previewScore = form ? form.likelihood * form.impact : risk.risk_score

  return (
    <div className="flex flex-col h-full">
      <Breadcrumbs items={[
        { label: 'Vakt Comply', href: '/vaktcomply' },
        { label: t('vaktcomply.riskDetail.breadcrumbRisks'), href: '/vaktcomply/risks' },
        { label: risk.title },
      ]} />
      <PageHeader
        title={risk.title}
        description={risk.category || t('vaktcomply.riskDetail.riskDetails')}
        actions={
          <div className="flex gap-2">
            <Button variant="outline" onClick={() => { navigate('/vaktcomply/risks'); }}>
              <ArrowLeft className="w-4 h-4 mr-1" />
              {t('vaktcomply.riskDetail.back')}
            </Button>
            <Button onClick={handleSave} disabled={!dirty || update.isPending}>
              <Save className="w-4 h-4 mr-1" />
              {update.isPending ? t('vaktcomply.riskDetail.saving') : t('vaktcomply.riskDetail.save')}
            </Button>
          </div>
        }
      />

      {form && (
        <div className="flex-1 p-6 space-y-6">
          <div className="grid grid-cols-1 lg:grid-cols-3 gap-6">
          {/* Left: edit form */}
          <div className="lg:col-span-2 space-y-4">
            <Card>
              <CardHeader><CardTitle className="text-sm">{t('vaktcomply.riskDetail.basicData')}</CardTitle></CardHeader>
              <CardContent className="space-y-4">
                <div className="space-y-1.5">
                  <Label>{t('vaktcomply.riskDetail.labelTitle')}</Label>
                  <Input value={form.title} onChange={(e) => { set('title', e.target.value); }} />
                </div>
                <div className="space-y-1.5">
                  <Label>{t('vaktcomply.riskDetail.labelCategory')}</Label>
                  <Input value={form.category ?? ''} placeholder={t('vaktcomply.riskDetail.placeholderCategory')} onChange={(e) => { set('category', e.target.value); }} />
                </div>
                <div className="space-y-1.5">
                  <Label>{t('vaktcomply.riskDetail.labelDescription')}</Label>
                  <Textarea rows={3} value={form.description ?? ''} onChange={(e) => { set('description', e.target.value); }} />
                </div>
                <div className="space-y-1.5">
                  <Label>{t('vaktcomply.riskDetail.labelOwner')}</Label>
                  <Input value={form.owner ?? ''} onChange={(e) => { set('owner', e.target.value); }} />
                </div>
              </CardContent>
            </Card>

            <Card>
              <CardHeader><CardTitle className="text-sm">{t('vaktcomply.riskDetail.treatment')}</CardTitle></CardHeader>
              <CardContent className="space-y-4">
                <div className="space-y-1.5">
                  <Label>{t('vaktcomply.riskDetail.labelStrategy')}</Label>
                  <Select value={form.treatment} onValueChange={(v) => { set('treatment', v as Risk['treatment']); }}>
                    <SelectTrigger><SelectValue /></SelectTrigger>
                    <SelectContent>
                      {(Object.keys(TREATMENT_LABELS) as Risk['treatment'][]).map((k) => (
                        <SelectItem key={k} value={k}>{TREATMENT_LABELS[k]}</SelectItem>
                      ))}
                    </SelectContent>
                  </Select>
                </div>
                <div className="space-y-1.5">
                  <Label>{t('vaktcomply.riskDetail.labelMeasures')}</Label>
                  <Textarea rows={3} value={form.treatment_notes ?? ''} onChange={(e) => { set('treatment_notes', e.target.value); }} />
                </div>
              </CardContent>
            </Card>
          </div>

          {/* Right: score + status */}
          <div className="space-y-4">
            <Card>
              <CardHeader><CardTitle className="text-sm">{t('vaktcomply.riskDetail.riskAssessment')}</CardTitle></CardHeader>
              <CardContent className="space-y-4">
                <div className="flex items-center justify-between">
                  <span className="text-sm text-muted-foreground">{t('vaktcomply.riskDetail.riskScore')}</span>
                  <Badge className={SCORE_COLOR(previewScore)}>{previewScore} / 25</Badge>
                </div>
                <div className="space-y-1.5">
                  <Label>{t('vaktcomply.riskDetail.labelLikelihood')}</Label>
                  <Input type="number" min={1} max={5} value={form.likelihood}
                    onChange={(e) => { set('likelihood', Math.min(5, Math.max(1, parseInt(e.target.value, 10) || 1))); }} />
                </div>
                <div className="space-y-1.5">
                  <Label>{t('vaktcomply.riskDetail.labelImpact')}</Label>
                  <Input type="number" min={1} max={5} value={form.impact}
                    onChange={(e) => { set('impact', Math.min(5, Math.max(1, parseInt(e.target.value, 10) || 1))); }} />
                </div>
              </CardContent>
            </Card>

            <Card>
              <CardHeader><CardTitle className="text-sm">{t('vaktcomply.riskDetail.statusCard')}</CardTitle></CardHeader>
              <CardContent>
                <Select value={form.status} onValueChange={(v) => { set('status', v as Risk['status']); }}>
                  <SelectTrigger><SelectValue /></SelectTrigger>
                  <SelectContent>
                    {(Object.keys(STATUS_LABELS) as Risk['status'][]).map((k) => (
                      <SelectItem key={k} value={k}>{STATUS_LABELS[k]}</SelectItem>
                    ))}
                  </SelectContent>
                </Select>
              </CardContent>
            </Card>

            <Card>
              <CardContent className="pt-4 space-y-1 text-xs text-muted-foreground">
                <p>{t('vaktcomply.riskDetail.createdAt')} {formatDate(risk.created_at)}</p>
                <p>{t('vaktcomply.riskDetail.updatedAt')} {formatDate(risk.updated_at)}</p>
              </CardContent>
            </Card>
          </div>
          </div>

          {/* S52-3: AI Risk Narrative */}
          <AIRiskNarrativePanel riskId={id ?? ''} existingNarrative={risk.ai_narrative ?? null} />

          {/* Treatment workflow — full width section below the main grid */}
          <div>
            <h2 className="text-sm font-semibold mb-3">{t('vaktcomply.riskDetail.treatmentSection')}</h2>
            <RiskTreatmentPanel riskId={id ?? ''} risk={risk} />
          </div>

          {/* S61-4: Residualrisiko-Berechnung */}
          <Card>
            <CardHeader>
              <CardTitle className="text-sm"><TermTooltip term="Residualrisiko" glossaryKey="Residualrisiko">Residualrisiko</TermTooltip>-{t('vaktcomply.riskDetail.residualTitle')}</CardTitle>
            </CardHeader>
            <CardContent className="space-y-5">
              {/* Score indicators */}
              <div className="flex items-center gap-4 flex-wrap">
                <div className="flex items-center gap-2">
                  <span className="text-xs text-muted-foreground">{t('vaktcomply.riskDetail.grossRisk')}</span>
                  <Badge className={residualScoreColor(risk.inherent_score)}>
                    {risk.inherent_score !== undefined ? `${risk.inherent_score} / 25` : '–'}
                  </Badge>
                </div>
                {risk.inherent_score !== undefined && risk.residual_score !== undefined && (
                  <span className="text-muted-foreground text-xs">→</span>
                )}
                <div className="flex items-center gap-2">
                  <span className="text-xs text-muted-foreground">{t('vaktcomply.riskDetail.netRisk')}</span>
                  <Badge className={residualScoreColor(risk.residual_score)}>
                    {risk.residual_score !== undefined ? `${risk.residual_score} / 25` : '–'}
                  </Badge>
                </div>
              </div>

              {/* Edit form */}
              <div className="grid grid-cols-2 md:grid-cols-4 gap-3">
                {(
                  [
                    { key: 'inherent_likelihood' as const, labelKey: 'grossLikelihood' },
                    { key: 'inherent_impact' as const, labelKey: 'grossImpact' },
                    { key: 'residual_likelihood' as const, labelKey: 'netLikelihood' },
                    { key: 'residual_impact' as const, labelKey: 'netImpact' },
                  ] as const
                ).map(({ key, labelKey }) => (
                  <div key={key} className="space-y-1">
                    <Label className="text-xs">{t(`vaktcomply.riskDetail.${labelKey}`)} (1–5)</Label>
                    <Input
                      type="number" min={1} max={5} placeholder="–"
                      value={residualForm[key]}
                      onChange={(e) => { setResidualForm((f) => ({ ...f, [key]: e.target.value })); }}
                    />
                  </div>
                ))}
              </div>
              <Button
                size="sm"
                variant="outline"
                disabled={updateResidual.isPending}
                onClick={() => {
                  const parse = (v: string) => { const n = parseInt(v, 10); return isNaN(n) ? undefined : Math.min(5, Math.max(1, n)); }
                  updateResidual.mutate({
                    inherent_likelihood: parse(residualForm.inherent_likelihood),
                    inherent_impact: parse(residualForm.inherent_impact),
                    residual_likelihood: parse(residualForm.residual_likelihood),
                    residual_impact: parse(residualForm.residual_impact),
                  })
                }}
              >
                {updateResidual.isPending ? t('vaktcomply.riskDetail.saving') : t('vaktcomply.riskDetail.saveValues')}
              </Button>

              {/* Risk acceptance section (only when treatment_status = accepted) */}
              {risk.status === 'accepted' && (
                <div className="border-t pt-4 space-y-2">
                  <p className="text-xs font-medium">{t('vaktcomply.riskDetail.formalAcceptance')}</p>
                  {risk.risk_accepted_at ? (
                    <div className="text-xs text-muted-foreground space-y-1">
                      <p>{t('vaktcomply.riskDetail.acceptedAt')} {formatDate(risk.risk_accepted_at)}</p>
                      {risk.risk_acceptance_justification && (
                        <p className="text-primary whitespace-pre-wrap">{risk.risk_acceptance_justification}</p>
                      )}
                    </div>
                  ) : (
                    <>
                      {!showAcceptDialog ? (
                        <Button size="sm" variant="outline" onClick={() => { setShowAcceptDialog(true); }}>
                          {t('vaktcomply.riskDetail.formalAcceptBtn')}
                        </Button>
                      ) : (
                        <div className="space-y-2">
                          <Label className="text-xs">{t('vaktcomply.riskDetail.justificationLabel')}</Label>
                          <Textarea
                            rows={3}
                            placeholder={t('vaktcomply.riskDetail.justificationPlaceholder')}
                            value={justification}
                            onChange={(e) => { setJustification(e.target.value); }}
                          />
                          <div className="flex gap-2">
                            <Button
                              size="sm"
                              disabled={!justification.trim() || acceptRisk.isPending}
                              onClick={() => {
                                acceptRisk.mutate(
                                  { justification },
                                  { onSuccess: () => { setShowAcceptDialog(false); setJustification(''); } },
                                )
                              }}
                            >
                              {acceptRisk.isPending ? t('vaktcomply.riskDetail.acceptingBtn') : t('vaktcomply.riskDetail.confirmBtn')}
                            </Button>
                            <Button size="sm" variant="ghost" onClick={() => { setShowAcceptDialog(false); }}>
                              {t('common.cancel')}
                            </Button>
                          </div>
                          {acceptRisk.isError && (
                            <p className="text-xs text-red-400">{acceptRisk.error?.message ?? t('common.error')}</p>
                          )}
                        </div>
                      )}
                    </>
                  )}
                </div>
              )}
            </CardContent>
          </Card>
        </div>
      )}

    </div>
  )
}
