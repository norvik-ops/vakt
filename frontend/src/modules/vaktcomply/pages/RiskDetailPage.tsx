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
import RiskTreatmentPanel from '../components/RiskTreatmentPanel'
import type { Risk, UpdateRiskInput } from '../types'
import { useFormatDate } from '../../../shared/hooks/useFormatDate'

const SCORE_COLOR = (score: number) => {
  if (score >= 15) return 'bg-red-500/20 text-red-400 border-red-500/30'
  if (score >= 9)  return 'bg-amber-500/20 text-amber-400 border-amber-500/30'
  if (score >= 4)  return 'bg-yellow-500/20 text-yellow-400 border-yellow-500/30'
  return 'bg-green-500/20 text-green-400 border-green-500/30'
}

const STATUS_LABELS: Record<Risk['status'], string> = {
  open: 'Offen', mitigated: 'Gemindert', accepted: 'Akzeptiert', closed: 'Geschlossen',
}
const TREATMENT_LABELS: Record<Risk['treatment'], string> = {
  avoid: 'Vermeiden', mitigate: 'Mindern', transfer: 'Übertragen', accept: 'Akzeptieren',
}
function AIRiskNarrativePanel({ riskId, existingNarrative }: { riskId: string; existingNarrative: string | null }) {
  const generate = useRiskNarrative(riskId)
  const { t } = useTranslation()

  return (
    <Card>
      <CardHeader>
        <CardTitle className="text-sm flex items-center gap-2">
          <Sparkles className="w-4 h-4 text-brand" />KI-Risikonarrative
          {existingNarrative && (
            <span className="ml-auto text-xs font-normal text-secondary/70 bg-amber-500/10 border border-amber-500/20 rounded px-1.5 py-0.5">
              {t('ai.disclaimer')}
            </span>
          )}
        </CardTitle>
      </CardHeader>
      <CardContent className="space-y-3">
        {existingNarrative && (
          <p className="text-xs text-primary leading-relaxed whitespace-pre-wrap">{existingNarrative}</p>
        )}
        {generate.isError && (
          <p className="text-xs text-red-400">{generate.error?.message ?? 'Fehler beim Generieren.'}</p>
        )}
        <button
          onClick={() => { generate.mutate(); }}
          disabled={generate.isPending}
          className="inline-flex items-center gap-1.5 text-xs text-brand hover:text-brand/80 disabled:opacity-50 disabled:cursor-not-allowed transition-colors"
        >
          {generate.isPending
            ? <><Loader2 className="w-3 h-3 animate-spin" /> Generiert…</>
            : <><Sparkles className="w-3 h-3" />{existingNarrative ? 'Neu generieren' : 'KI-Narrative generieren'}</>
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
    <div className="p-6 text-sm text-red-400">Risiko nicht gefunden.</div>
  )

  const previewScore = form ? form.likelihood * form.impact : risk.risk_score

  return (
    <div className="flex flex-col h-full">
      <Breadcrumbs items={[
        { label: 'Vakt Comply', href: '/vaktcomply' },
        { label: 'Risiken', href: '/vaktcomply/risks' },
        { label: risk.title },
      ]} />
      <PageHeader
        title={risk.title}
        description={risk.category || 'Risikodetails'}
        actions={
          <div className="flex gap-2">
            <Button variant="outline" onClick={() => { navigate('/vaktcomply/risks'); }}>
              <ArrowLeft className="w-4 h-4 mr-1" />
              Zurück
            </Button>
            <Button onClick={handleSave} disabled={!dirty || update.isPending}>
              <Save className="w-4 h-4 mr-1" />
              {update.isPending ? 'Speichern …' : 'Speichern'}
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
              <CardHeader><CardTitle className="text-sm">Grunddaten</CardTitle></CardHeader>
              <CardContent className="space-y-4">
                <div className="space-y-1.5">
                  <Label>Bezeichnung</Label>
                  <Input value={form.title} onChange={(e) => { set('title', e.target.value); }} />
                </div>
                <div className="space-y-1.5">
                  <Label>Kategorie</Label>
                  <Input value={form.category ?? ''} placeholder="z.B. Cyber, Compliance" onChange={(e) => { set('category', e.target.value); }} />
                </div>
                <div className="space-y-1.5">
                  <Label>Beschreibung</Label>
                  <Textarea rows={3} value={form.description ?? ''} onChange={(e) => { set('description', e.target.value); }} />
                </div>
                <div className="space-y-1.5">
                  <Label>Verantwortlicher</Label>
                  <Input value={form.owner ?? ''} onChange={(e) => { set('owner', e.target.value); }} />
                </div>
              </CardContent>
            </Card>

            <Card>
              <CardHeader><CardTitle className="text-sm">Behandlung</CardTitle></CardHeader>
              <CardContent className="space-y-4">
                <div className="space-y-1.5">
                  <Label>Strategie</Label>
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
                  <Label>Maßnahmen</Label>
                  <Textarea rows={3} value={form.treatment_notes ?? ''} onChange={(e) => { set('treatment_notes', e.target.value); }} />
                </div>
              </CardContent>
            </Card>
          </div>

          {/* Right: score + status */}
          <div className="space-y-4">
            <Card>
              <CardHeader><CardTitle className="text-sm">Risikobewertung</CardTitle></CardHeader>
              <CardContent className="space-y-4">
                <div className="flex items-center justify-between">
                  <span className="text-sm text-muted-foreground">Risiko-Score</span>
                  <Badge className={SCORE_COLOR(previewScore)}>{previewScore} / 25</Badge>
                </div>
                <div className="space-y-1.5">
                  <Label>Wahrscheinlichkeit (1–5)</Label>
                  <Input type="number" min={1} max={5} value={form.likelihood}
                    onChange={(e) => { set('likelihood', Math.min(5, Math.max(1, parseInt(e.target.value, 10) || 1))); }} />
                </div>
                <div className="space-y-1.5">
                  <Label>Auswirkung (1–5)</Label>
                  <Input type="number" min={1} max={5} value={form.impact}
                    onChange={(e) => { set('impact', Math.min(5, Math.max(1, parseInt(e.target.value, 10) || 1))); }} />
                </div>
              </CardContent>
            </Card>

            <Card>
              <CardHeader><CardTitle className="text-sm">Status</CardTitle></CardHeader>
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
                <p>Erstellt: {formatDate(risk.created_at)}</p>
                <p>Geändert: {formatDate(risk.updated_at)}</p>
              </CardContent>
            </Card>
          </div>
          </div>

          {/* S52-3: AI Risk Narrative */}
          <AIRiskNarrativePanel riskId={id ?? ''} existingNarrative={risk.ai_narrative ?? null} />

          {/* Treatment workflow — full width section below the main grid */}
          <div>
            <h2 className="text-sm font-semibold mb-3">Risikobehandlung (ISO 27001 Clause 6)</h2>
            <RiskTreatmentPanel riskId={id ?? ''} risk={risk} />
          </div>

          {/* S61-4: Residualrisiko-Berechnung */}
          <Card>
            <CardHeader>
              <CardTitle className="text-sm">Residualrisiko-Berechnung (ISO 27001 Clause 8.3)</CardTitle>
            </CardHeader>
            <CardContent className="space-y-5">
              {/* Score indicators */}
              <div className="flex items-center gap-4 flex-wrap">
                <div className="flex items-center gap-2">
                  <span className="text-xs text-muted-foreground">Bruttorisiko:</span>
                  <Badge className={residualScoreColor(risk.inherent_score)}>
                    {risk.inherent_score !== undefined ? `${risk.inherent_score} / 25` : '–'}
                  </Badge>
                </div>
                {risk.inherent_score !== undefined && risk.residual_score !== undefined && (
                  <span className="text-muted-foreground text-xs">→</span>
                )}
                <div className="flex items-center gap-2">
                  <span className="text-xs text-muted-foreground">Nettorisiko:</span>
                  <Badge className={residualScoreColor(risk.residual_score)}>
                    {risk.residual_score !== undefined ? `${risk.residual_score} / 25` : '–'}
                  </Badge>
                </div>
              </div>

              {/* Edit form */}
              <div className="grid grid-cols-2 md:grid-cols-4 gap-3">
                {(
                  [
                    { key: 'inherent_likelihood' as const, label: 'Brutto-Wahrscheinlichkeit' },
                    { key: 'inherent_impact' as const, label: 'Brutto-Auswirkung' },
                    { key: 'residual_likelihood' as const, label: 'Netto-Wahrscheinlichkeit' },
                    { key: 'residual_impact' as const, label: 'Netto-Auswirkung' },
                  ] as const
                ).map(({ key, label }) => (
                  <div key={key} className="space-y-1">
                    <Label className="text-xs">{label} (1–5)</Label>
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
                {updateResidual.isPending ? 'Speichern …' : 'Werte speichern'}
              </Button>

              {/* Risk acceptance section (only when treatment_status = accepted) */}
              {risk.status === 'accepted' && (
                <div className="border-t pt-4 space-y-2">
                  <p className="text-xs font-medium">Formale Risikoakzeptanz</p>
                  {risk.risk_accepted_at ? (
                    <div className="text-xs text-muted-foreground space-y-1">
                      <p>Akzeptiert am: {formatDate(risk.risk_accepted_at)}</p>
                      {risk.risk_acceptance_justification && (
                        <p className="text-primary whitespace-pre-wrap">{risk.risk_acceptance_justification}</p>
                      )}
                    </div>
                  ) : (
                    <>
                      {!showAcceptDialog ? (
                        <Button size="sm" variant="outline" onClick={() => { setShowAcceptDialog(true); }}>
                          Risiko formal akzeptieren
                        </Button>
                      ) : (
                        <div className="space-y-2">
                          <Label className="text-xs">Begründung (Pflicht)</Label>
                          <Textarea
                            rows={3}
                            placeholder="Begründung für die Risikoakzeptanz…"
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
                              {acceptRisk.isPending ? 'Akzeptieren …' : 'Bestätigen'}
                            </Button>
                            <Button size="sm" variant="ghost" onClick={() => { setShowAcceptDialog(false); }}>
                              Abbrechen
                            </Button>
                          </div>
                          {acceptRisk.isError && (
                            <p className="text-xs text-red-400">{acceptRisk.error?.message ?? 'Fehler'}</p>
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
