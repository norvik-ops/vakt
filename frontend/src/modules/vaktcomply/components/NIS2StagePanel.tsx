import { useState } from 'react'
import { CheckCircle2, Clock, AlertTriangle, ChevronDown, ChevronRight } from 'lucide-react'
import { Card, CardContent, CardHeader, CardTitle } from '../../../components/ui/card'
import { Button } from '../../../components/ui/button'
import { Badge } from '../../../components/ui/badge'
import { Textarea } from '../../../components/ui/textarea'
import { Input } from '../../../components/ui/input'
import { Label } from '../../../components/ui/label'
import { Spinner } from '../../../components/Spinner'
import { useNIS2Status, useNIS2AssessReportability, useNIS2SubmitStage } from '../hooks/useNIS2Reporting'
import { toast } from '../../../shared/hooks/useToast'
import type { NIS2ReportInput } from '../types'

const STAGE_LABELS: Record<string, string> = {
  none: 'Kein Workflow aktiv',
  early_warning: 'Frühwarnung (24h)',
  full_report: '72h-Meldung',
  final_report: '30-Tage-Abschlussbericht',
}

function formatDeadline(iso?: string | null): string {
  if (!iso) return '—'
  return new Date(iso).toLocaleString('de-DE', {
    day: '2-digit', month: '2-digit', year: 'numeric',
    hour: '2-digit', minute: '2-digit',
  })
}

function hoursLeft(iso?: string | null): number | null {
  if (!iso) return null
  return (new Date(iso).getTime() - Date.now()) / 3_600_000
}

function DeadlineItem({
  label,
  deadline,
  isCompleted,
}: {
  label: string
  deadline?: string | null
  isCompleted: boolean
}) {
  const hours = hoursLeft(deadline)
  const overdue = hours !== null && hours < 0
  const urgent = hours !== null && hours >= 0 && hours < 2

  return (
    <div className="flex items-center justify-between py-2 border-b border-border last:border-0">
      <div className="flex items-center gap-2">
        {isCompleted
          ? <CheckCircle2 className="w-4 h-4 text-green-400" />
          : overdue
            ? <AlertTriangle className="w-4 h-4 text-red-400" />
            : <Clock className={`w-4 h-4 ${urgent ? 'text-amber-400' : 'text-muted-foreground'}`} />
        }
        <div>
          <p className="text-sm font-medium">{label}</p>
          <p className="text-xs text-muted-foreground">{formatDeadline(deadline)}</p>
        </div>
      </div>
      {isCompleted
        ? <Badge className="text-xs bg-green-500/20 text-green-400 border-green-500/30">Eingereicht</Badge>
        : overdue
          ? <Badge className="text-xs bg-red-500/20 text-red-400 border-red-500/30">Überfällig</Badge>
          : urgent
            ? <Badge className="text-xs bg-amber-500/20 text-amber-400 border-amber-500/30">Dringend</Badge>
            : <Badge variant="outline" className="text-xs">Offen</Badge>
      }
    </div>
  )
}

type Stage = 'early_warning' | 'full_report' | 'final_report'

function StageForm({
  incidentId,
  stage,
  onSuccess,
}: {
  incidentId: string
  stage: Stage
  onSuccess: () => void
}) {
  const submit = useNIS2SubmitStage(incidentId)
  const [form, setForm] = useState<NIS2ReportInput>({
    affected_services: '',
    initial_assessment: '',
    root_cause: '',
    measures_taken: '',
    full_root_cause_analysis: '',
    permanent_measures: '',
    effectiveness_evidence: '',
  })

  function handleSubmit() {
    submit.mutate(
      { stage, input: form },
      {
        onSuccess: () => {
          toast(`${STAGE_LABELS[stage]} wurde gespeichert.`)
          onSuccess()
        },
        onError: (err) => {
          toast(err.message, 'error')
        },
      },
    )
  }

  return (
    <div className="space-y-3 pt-3 border-t border-border">
      <div className="space-y-1">
        <Label className="text-xs">Betroffene Dienste</Label>
        <Input
          value={form.affected_services}
          onChange={(e) => { setForm((f) => ({ ...f, affected_services: e.target.value })); }}
          placeholder="z.B. Online-Banking, VPN"
        />
      </div>
      <div className="space-y-1">
        <Label className="text-xs">Erste Einschätzung</Label>
        <Textarea
          rows={2}
          value={form.initial_assessment}
          onChange={(e) => { setForm((f) => ({ ...f, initial_assessment: e.target.value })); }}
        />
      </div>
      {(stage === 'full_report' || stage === 'final_report') && (
        <>
          <div className="space-y-1">
            <Label className="text-xs">Ursachen-Hypothese</Label>
            <Textarea
              rows={2}
              value={form.root_cause}
              onChange={(e) => { setForm((f) => ({ ...f, root_cause: e.target.value })); }}
            />
          </div>
          <div className="space-y-1">
            <Label className="text-xs">Ergriffene Maßnahmen</Label>
            <Textarea
              rows={2}
              value={form.measures_taken}
              onChange={(e) => { setForm((f) => ({ ...f, measures_taken: e.target.value })); }}
            />
          </div>
        </>
      )}
      {stage === 'final_report' && (
        <>
          <div className="space-y-1">
            <Label className="text-xs">Vollständige Root-Cause-Analyse</Label>
            <Textarea
              rows={3}
              value={form.full_root_cause_analysis}
              onChange={(e) => { setForm((f) => ({ ...f, full_root_cause_analysis: e.target.value })); }}
            />
          </div>
          <div className="space-y-1">
            <Label className="text-xs">Dauerhafte Maßnahmen</Label>
            <Textarea
              rows={2}
              value={form.permanent_measures}
              onChange={(e) => { setForm((f) => ({ ...f, permanent_measures: e.target.value })); }}
            />
          </div>
          <div className="space-y-1">
            <Label className="text-xs">Wirksamkeitsnachweis</Label>
            <Textarea
              rows={2}
              value={form.effectiveness_evidence}
              onChange={(e) => { setForm((f) => ({ ...f, effectiveness_evidence: e.target.value })); }}
            />
          </div>
        </>
      )}
      <Button
        size="sm"
        onClick={handleSubmit}
        disabled={submit.isPending || !form.affected_services || !form.initial_assessment}
      >
        {submit.isPending ? 'Einreichen …' : 'Stufe einreichen'}
      </Button>
    </div>
  )
}

export function NIS2StagePanel({ incidentId }: { incidentId: string }) {
  const { data: status, isLoading, isError } = useNIS2Status(incidentId)
  const assess = useNIS2AssessReportability(incidentId)
  const [openStage, setOpenStage] = useState<Stage | null>(null)
  const [assessCheck, setAssessCheck] = useState({
    causes_significant_disruption: false,
    affects_third_parties: false,
    causes_financial_damage: false,
  })

  if (isLoading) return (
    <div className="flex items-center justify-center h-24">
      <Spinner size="sm" color="primary" />
    </div>
  )
  if (isError) return null

  function handleAssess() {
    assess.mutate(
      {
        detected_at: new Date().toISOString(),
        check: assessCheck,
      },
      {
        onSuccess: () => {
          toast('NIS2-Meldepflicht bewertet')
        },
        onError: (err) => {
          toast(err.message, 'error')
        },
      },
    )
  }

  const stages: { key: Stage; label: string; deadline?: string | null }[] = [
    { key: 'early_warning', label: 'Frühwarnung (24h)', deadline: status?.deadlines.early_warning },
    { key: 'full_report', label: '72h-Meldung', deadline: status?.deadlines.full_report },
    { key: 'final_report', label: '30-Tage-Abschluss', deadline: status?.deadlines.final_report },
  ]

  return (
    <Card className="border-primary/20" data-testid="nis2-stage-panel">
      <CardHeader>
        <CardTitle className="text-sm flex items-center gap-2">
          <AlertTriangle className="w-4 h-4 text-amber-400" />
          NIS2 Art.23 Meldepflicht-Workflow
          {status?.is_reportable && (
            <Badge className="text-xs bg-red-500/20 text-red-400 border-red-500/30 ml-auto">
              Meldepflichtig
            </Badge>
          )}
        </CardTitle>
      </CardHeader>
      <CardContent className="space-y-4">
        {!status?.is_reportable && (
          <div className="space-y-3">
            <p className="text-xs text-muted-foreground">
              Prüfen Sie, ob dieser Vorfall nach NIS2 Art.23 meldepflichtig ist:
            </p>
            {[
              { key: 'causes_significant_disruption', label: 'Verursacht erhebliche Betriebsunterbrechungen' },
              { key: 'affects_third_parties', label: 'Beeinträchtigt Dritte erheblich' },
              { key: 'causes_financial_damage', label: 'Verursacht finanziellen Schaden' },
            ].map(({ key, label }) => (
              <label key={key} className="flex items-center gap-2 text-sm cursor-pointer">
                <input
                  type="checkbox"
                  checked={assessCheck[key as keyof typeof assessCheck]}
                  onChange={(e) => {
                    setAssessCheck((c) => ({ ...c, [key]: e.target.checked }))
                  }}
                  className="rounded"
                />
                {label}
              </label>
            ))}
            <Button
              size="sm"
              variant="outline"
              onClick={handleAssess}
              disabled={assess.isPending}
              data-testid="nis2-assess-btn"
            >
              {assess.isPending ? 'Prüfe …' : 'Meldepflicht bewerten'}
            </Button>
          </div>
        )}

        {status?.is_reportable && (
          <>
            <p className="text-xs text-muted-foreground">
              Aktueller Stand:{' '}
              <span className="font-medium text-foreground">
                {STAGE_LABELS[status.reporting_stage] ?? status.reporting_stage}
              </span>
            </p>

            <div className="divide-y divide-border">
              {stages.map(({ key, label, deadline }) => (
                <DeadlineItem
                  key={key}
                  label={label}
                  deadline={deadline}
                  isCompleted={status.completed_stages.includes(key)}
                />
              ))}
            </div>

            <div className="space-y-2">
              {stages.map(({ key, label }) => {
                const done = status.completed_stages.includes(key)
                if (done) return null
                const isOpen = openStage === key
                return (
                  <div key={key} className="border border-border rounded">
                    <button
                      type="button"
                      className="w-full flex items-center justify-between px-3 py-2 text-sm hover:bg-muted/30"
                      onClick={() => { setOpenStage(isOpen ? null : key); }}
                    >
                      <span>{label} ausfüllen</span>
                      {isOpen ? <ChevronDown className="w-4 h-4" /> : <ChevronRight className="w-4 h-4" />}
                    </button>
                    {isOpen && (
                      <div className="px-3 pb-3">
                        <StageForm
                          incidentId={incidentId}
                          stage={key}
                          onSuccess={() => { setOpenStage(null); }}
                        />
                      </div>
                    )}
                  </div>
                )
              })}
            </div>
          </>
        )}
      </CardContent>
    </Card>
  )
}
