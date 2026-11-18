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
import { useRisk, useSaveRiskDetail, useAcceptRisk } from '../hooks/useRisks'
import { useRiskNarrative } from '../hooks/useAIInsights'
import { useAIStatus } from '../hooks/useAIAdvisor'
import { AIDisclaimer } from '../../../shared/components/AIDisclaimer'
import RiskTreatmentPanel, { type TreatmentFields } from '../components/RiskTreatmentPanel'
import type { Risk } from '../types'
import { useFormatDate } from '../../../shared/hooks/useFormatDate'
import { TermTooltip } from '../../../shared/components/TermTooltip'
import { riskLevel, RISK_LEVEL_BADGE } from '../../../shared/utils/riskLevel'
import { ReadOnlyHint } from '../../../shared/components/ReadOnlyHint'
import { WriteFieldset } from '../../../shared/components/WriteFieldset'
import { useCanWrite } from '../../../shared/hooks/useCanWrite'
import { riskToEditForm, residualScoreOf, type RiskEditForm } from '../utils/riskEditForm'

const SCORE_COLOR = (score: number) => RISK_LEVEL_BADGE[riskLevel(score)]

function AIRiskNarrativePanel({ riskId, existingNarrative }: { riskId: string; existingNarrative: string | null }) {
  const generate = useRiskNarrative(riskId)
  const { t } = useTranslation()
  const { data: aiStatus } = useAIStatus()
  const aiAvailable = aiStatus?.available === true

  // Ohne KI-Anbieter sind die /ai/*-Routen nicht registriert; der Knopf würde nur
  // einen Fehler erzeugen (UX-24). Eine früher erzeugte Narrative bleibt lesbar.
  if (!aiAvailable && !existingNarrative) return null

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
        {aiAvailable && (
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
        )}
      </CardContent>
    </Card>
  )
}

/** Formal acceptance (ISO 27001 6.1.3 / 8.3), only when the register status is 'accepted'. */
function RiskAcceptance({ risk }: { risk: Risk }) {
  const { t } = useTranslation()
  const { formatDate } = useFormatDate()
  const acceptRisk = useAcceptRisk(risk.id)
  const [showAcceptDialog, setShowAcceptDialog] = useState(false)
  const [justification, setJustification] = useState('')

  // This gate was always the right one; the backend precondition read
  // treatment_status and could never be satisfied — R1-14c-12.
  if (risk.status !== 'accepted') return null

  return (
    <div className="border-t pt-3 space-y-2">
      <p className="text-xs font-medium">{t('vaktcomply.riskDetail.formalAcceptance')}</p>
      {risk.risk_accepted_at ? (
        <div className="text-xs text-muted-foreground space-y-1">
          <p>{t('vaktcomply.riskDetail.acceptedAt')} {formatDate(risk.risk_accepted_at)}</p>
          {risk.risk_acceptance_justification && (
            <p className="text-primary whitespace-pre-wrap">{risk.risk_acceptance_justification}</p>
          )}
        </div>
      ) : !showAcceptDialog ? (
        <Button size="sm" variant="outline" onClick={() => { setShowAcceptDialog(true); }}>
          {t('vaktcomply.riskDetail.formalAcceptBtn')}
        </Button>
      ) : (
        <div className="space-y-2">
          <Label className="text-xs" htmlFor="risk-accept-justification">{t('vaktcomply.riskDetail.justificationLabel')}</Label>
          <Textarea
            id="risk-accept-justification"
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
    </div>
  )
}

/**
 * Risk detail. UX-26: every value is asked once — strategy, measures, owner and
 * residual risk used to appear in two or three blocks with their own save buttons.
 * One form, one save (header button and the button under the treatment section do
 * the same); see utils/riskEditForm.ts for the column pairs it keeps in step.
 * UX-27: the residual score stands next to the gross score at the top.
 */
export default function RiskDetailPage() {
  const { id } = useParams<{ id: string }>()
  const navigate = useNavigate()
  const canWrite = useCanWrite()
  const { formatDate } = useFormatDate()
  const { t } = useTranslation()
  const { data: risk, isLoading, isError } = useRisk(id ?? '')
  const save = useSaveRiskDetail(id ?? '')

  const [form, setForm] = useState<RiskEditForm | null>(null)
  const [dirty, setDirty] = useState(false)

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
    if (risk && !form) setForm(riskToEditForm(risk))
  }, [risk, form])

  function set<K extends keyof RiskEditForm>(key: K, value: RiskEditForm[K]) {
    setForm((f) => f ? { ...f, [key]: value } : f)
    setDirty(true)
  }

  function handleSave() {
    if (!form) return
    save.mutate(form, { onSuccess: () => { setDirty(false); } })
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
  const residualScore = form
    ? residualScoreOf(form.residualLikelihood, form.residualImpact)
    : (risk.residual_score ?? null)

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
            {canWrite && (
              <Button onClick={handleSave} disabled={!dirty || save.isPending}>
                <Save className="w-4 h-4 mr-1" />
                {save.isPending ? t('vaktcomply.riskDetail.saving') : t('vaktcomply.riskDetail.save')}
              </Button>
            )}
          </div>
        }
      />

      {form && (
        <div className="flex-1 p-6 space-y-6">
          <ReadOnlyHint />
          {save.isError && (
            <p role="alert" className="text-sm text-red-400">{save.error?.message ?? t('common.error')}</p>
          )}
          <WriteFieldset>
          <div className="grid grid-cols-1 lg:grid-cols-3 gap-6">
          {/* Left: basic data */}
          <div className="lg:col-span-2 space-y-4">
            <Card>
              <CardHeader><CardTitle className="text-sm">{t('vaktcomply.riskDetail.basicData')}</CardTitle></CardHeader>
              <CardContent className="space-y-4">
                <div className="space-y-1.5">
                  <Label htmlFor="risk-title">{t('vaktcomply.riskDetail.labelTitle')}</Label>
                  <Input id="risk-title" value={form.title} onChange={(e) => { set('title', e.target.value); }} />
                </div>
                <div className="space-y-1.5">
                  <Label htmlFor="risk-category">{t('vaktcomply.riskDetail.labelCategory')}</Label>
                  <Input id="risk-category" value={form.category} placeholder={t('vaktcomply.riskDetail.placeholderCategory')} onChange={(e) => { set('category', e.target.value); }} />
                </div>
                <div className="space-y-1.5">
                  <Label htmlFor="risk-description">{t('vaktcomply.riskDetail.labelDescription')}</Label>
                  <Textarea id="risk-description" rows={3} value={form.description} onChange={(e) => { set('description', e.target.value); }} />
                </div>
                <div className="space-y-1.5">
                  <Label htmlFor="risk-owner">{t('vaktcomply.riskDetail.labelOwner')}</Label>
                  <Input id="risk-owner" value={form.owner} placeholder="z.B. Max Mustermann" onChange={(e) => { set('owner', e.target.value); }} />
                  <p className="text-xs text-muted-foreground">{t('vaktcomply.riskDetail.ownerHint')}</p>
                </div>
              </CardContent>
            </Card>
          </div>

          {/* Right: assessment (gross and residual) + status */}
          <div className="space-y-4">
            <Card>
              <CardHeader><CardTitle className="text-sm">{t('vaktcomply.riskDetail.riskAssessment')}</CardTitle></CardHeader>
              <CardContent className="space-y-4">
                <div className="flex items-center justify-between">
                  <span className="text-sm text-muted-foreground">{t('vaktcomply.riskDetail.riskScore')}</span>
                  <Badge className={SCORE_COLOR(previewScore)}>{previewScore} / 25</Badge>
                </div>
                <div className="flex items-center justify-between" data-testid="risk-residual-summary">
                  <span className="text-sm text-muted-foreground">
                    <TermTooltip term="Restrisiko" glossaryKey="Residualrisiko">{t('vaktcomply.riskDetail.residualRisk')}</TermTooltip>
                  </span>
                  {residualScore != null
                    ? <Badge className={SCORE_COLOR(residualScore)}>{residualScore} / 25</Badge>
                    : <span className="text-xs text-muted-foreground">{t('vaktcomply.riskDetail.residualNotSet')}</span>}
                </div>
                <div className="space-y-1.5">
                  <Label htmlFor="risk-likelihood">{t('vaktcomply.riskDetail.labelLikelihood')}</Label>
                  <Input id="risk-likelihood" type="number" min={1} max={5} value={form.likelihood}
                    onChange={(e) => { set('likelihood', Math.min(5, Math.max(1, parseInt(e.target.value, 10) || 1))); }} />
                </div>
                <div className="space-y-1.5">
                  <Label htmlFor="risk-impact">{t('vaktcomply.riskDetail.labelImpact')}</Label>
                  <Input id="risk-impact" type="number" min={1} max={5} value={form.impact}
                    onChange={(e) => { set('impact', Math.min(5, Math.max(1, parseInt(e.target.value, 10) || 1))); }} />
                </div>
              </CardContent>
            </Card>

            <Card>
              <CardHeader><CardTitle className="text-sm">{t('vaktcomply.riskDetail.statusCard')}</CardTitle></CardHeader>
              <CardContent className="space-y-3">
                <Select value={form.status} onValueChange={(v) => { set('status', v as Risk['status']); }}>
                  <SelectTrigger aria-label={t('vaktcomply.riskDetail.statusCard')}><SelectValue /></SelectTrigger>
                  <SelectContent>
                    {(Object.keys(STATUS_LABELS) as Risk['status'][]).map((k) => (
                      <SelectItem key={k} value={k}>{STATUS_LABELS[k]}</SelectItem>
                    ))}
                  </SelectContent>
                </Select>
                <RiskAcceptance risk={risk} />
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

          {/* The one treatment block: strategy, plan, due date, progress, residual risk, controls. */}
          <div>
            <h2 className="text-sm font-semibold mb-3">{t('vaktcomply.riskDetail.treatmentSection')}</h2>
            <RiskTreatmentPanel
              riskId={id ?? ''}
              grossScore={previewScore}
              value={form satisfies TreatmentFields}
              onChange={(patch) => { setForm((f) => f ? { ...f, ...patch } : f); setDirty(true) }}
              onSave={handleSave}
              saving={save.isPending}
              dirty={dirty}
            />
          </div>
          </WriteFieldset>
        </div>
      )}

    </div>
  )
}
