import { useState, useEffect } from 'react'
import { useParams, useNavigate } from 'react-router-dom'
import { ArrowLeft, Save } from 'lucide-react'
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
import { useRisk, useUpdateRisk } from '../hooks/useRisks'
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
export default function RiskDetailPage() {
  const { id } = useParams<{ id: string }>()
  const navigate = useNavigate()
  const { formatDate } = useFormatDate()
  const { data: risk, isLoading, isError } = useRisk(id ?? '')
  const update = useUpdateRisk(id ?? '')

  const [form, setForm] = useState<UpdateRiskInput | null>(null)
  const [dirty, setDirty] = useState(false)

  useEffect(() => {
    if (risk) trackPage(`/secvitals/risks/${id}`, risk.title, '⚠️')
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
        { label: 'SecVitals', href: '/secvitals' },
        { label: 'Risiken', href: '/secvitals/risks' },
        { label: risk.title },
      ]} />
      <PageHeader
        title={risk.title}
        description={risk.category || 'Risikodetails'}
        actions={
          <div className="flex gap-2">
            <Button variant="outline" onClick={() => { navigate('/secvitals/risks'); }}>
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

          {/* Treatment workflow — full width section below the main grid */}
          <div>
            <h2 className="text-sm font-semibold mb-3">Risikobehandlung (ISO 27001 Clause 6)</h2>
            <RiskTreatmentPanel riskId={id ?? ''} risk={risk} />
          </div>
        </div>
      )}

    </div>
  )
}
