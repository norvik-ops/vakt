import { useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import {
  Shield,
  ChevronRight,
  ChevronLeft,
  TrendingUp,
  TrendingDown,
  Minus,
  RefreshCw,
  Clock,
  CheckCircle2,
  AlertTriangle,
} from 'lucide-react'
import { apiFetch, FeatureLockedError } from '../../../api/client'
import { ProGate } from '../../../shared/components/ProGate'
import { PageHeader } from '../../../shared/components/PageHeader'
import { Button } from '../../../components/ui/button'
import { Card, CardContent, CardHeader, CardTitle } from '../../../components/ui/card'
import { Badge } from '../../../components/ui/badge'

// Sprint 28 / S28-3: Re-Assessment-History — zeigt alle vergangenen NIS2-
// Re-Assessment-Runs einer Org mit Diff-View (Δ-Spalte) pro Bereich.
//
// ProGate: FeatureNIS2Reporting. Kein Run vorhanden → "Neu bewerten"-Button.
// Letzter Run < 90 Tage → Button disabled mit Hinweis wann wieder möglich.

// ─── Types ──────────────────────────────────────────────────────────────────

interface AnswerEntry {
  value: number
  comment?: string
}

interface AssessmentRun {
  id: string
  org_id: string
  run_number: number
  answers: Record<string, AnswerEntry>
  overall_score?: number
  score_by_area?: Record<string, number>
  top_gaps?: Gap[]
  completed_at?: string
  created_at: string
}

interface Gap {
  area: string
  area_title: string
  score: number
}

interface HistoryResponse {
  runs: AssessmentRun[]
  total: number
}

interface Question {
  id: string
  area: string
  title: string
  help: string
  nis2_ref: string
  weight: number
}

const AREA_LABELS: Record<string, string> = {
  governance: 'Governance & Verantwortlichkeit',
  risk_management: 'Risikomanagement',
  incident_response: 'Incident-Response',
  business_continuity: 'Business Continuity',
  supply_chain: 'Lieferketten-Sicherheit',
  crypto: 'Kryptographie',
  access_control: 'Zugriffskontrolle',
  asset_management: 'Asset- & Vulnerability-Mgmt',
}

const VALUE_LABELS = [
  'Nicht implementiert',
  'In Planung',
  'Teilweise umgesetzt',
  'Weitgehend umgesetzt',
  'Vollständig + getestet',
]

const COOLDOWN_DAYS = 90

// ─── Helper ──────────────────────────────────────────────────────────────────

function scoreColor(score: number): string {
  if (score >= 70) return 'text-green-600'
  if (score >= 40) return 'text-amber-600'
  return 'text-red-600'
}

function deltaBadge(delta: number | null) {
  if (delta === null) return <span className="text-gray-400 text-xs">—</span>
  if (delta > 0)
    return (
      <span className="inline-flex items-center gap-0.5 text-green-600 text-xs font-semibold">
        <TrendingUp className="w-3 h-3" /> +{delta}
      </span>
    )
  if (delta < 0)
    return (
      <span className="inline-flex items-center gap-0.5 text-red-600 text-xs font-semibold">
        <TrendingDown className="w-3 h-3" /> {delta}
      </span>
    )
  return (
    <span className="inline-flex items-center gap-0.5 text-gray-500 text-xs">
      <Minus className="w-3 h-3" /> 0
    </span>
  )
}

function formatDate(iso: string): string {
  return new Date(iso).toLocaleDateString('de-DE', {
    day: '2-digit',
    month: '2-digit',
    year: 'numeric',
  })
}

// ─── Sub-components ───────────────────────────────────────────────────────────

function HistoryTable({ runs }: { runs: AssessmentRun[] }) {
  const [expanded, setExpanded] = useState<string | null>(null)

  // Für den Diff brauchen wir die Runs in chronologischer Reihenfolge.
  const chronological = [...runs].reverse()

  return (
    <div className="space-y-3">
      {runs.map((run, idx) => {
        // Vorheriger Run (älterer) aus der chronologischen Liste.
        const chronIdx = chronological.findIndex((r) => r.id === run.id)
        const prevRun = chronIdx > 0 ? chronological[chronIdx - 1] : null

        const isExpanded = expanded === run.id
        const areas = Object.keys(AREA_LABELS)

        return (
          <Card key={run.id} className="overflow-hidden">
            <CardHeader
              className="cursor-pointer select-none py-3 px-4 hover:bg-gray-50 transition-colors"
              onClick={() => setExpanded(isExpanded ? null : run.id)}
            >
              <div className="flex items-center justify-between">
                <div className="flex items-center gap-3">
                  <Badge variant="secondary">Run #{run.run_number}</Badge>
                  <span className="text-sm text-gray-500">{formatDate(run.created_at)}</span>
                  {run.completed_at && (
                    <span className="inline-flex items-center gap-1 text-xs text-green-600">
                      <CheckCircle2 className="w-3 h-3" /> Abgeschlossen
                    </span>
                  )}
                  {!run.completed_at && (
                    <span className="inline-flex items-center gap-1 text-xs text-amber-600">
                      <Clock className="w-3 h-3" /> In Bearbeitung
                    </span>
                  )}
                </div>
                <div className="flex items-center gap-4">
                  {run.overall_score !== undefined && (
                    <span className={`text-2xl font-bold ${scoreColor(run.overall_score)}`}>
                      {run.overall_score}
                    </span>
                  )}
                  {prevRun?.overall_score !== undefined && run.overall_score !== undefined
                    ? deltaBadge(run.overall_score - prevRun.overall_score)
                    : idx > 0
                    ? deltaBadge(null)
                    : null}
                  {isExpanded ? (
                    <ChevronLeft className="w-4 h-4 text-gray-400 rotate-90" />
                  ) : (
                    <ChevronRight className="w-4 h-4 text-gray-400 rotate-90" />
                  )}
                </div>
              </div>
            </CardHeader>

            {isExpanded && run.score_by_area && (
              <CardContent className="pt-0 pb-4 px-4">
                <div className="overflow-x-auto">
                  <table className="w-full text-sm">
                    <thead>
                      <tr className="border-b text-left">
                        <th className="pb-2 pr-4 font-medium text-gray-600">Bereich</th>
                        <th className="pb-2 pr-4 font-medium text-gray-600 text-right">Score</th>
                        <th className="pb-2 font-medium text-gray-600 text-right">Δ vs. Vorherig</th>
                      </tr>
                    </thead>
                    <tbody>
                      {areas.map((area) => {
                        const score = run.score_by_area?.[area]
                        const prevScore = prevRun?.score_by_area?.[area]
                        const delta =
                          score !== undefined && prevScore !== undefined
                            ? score - prevScore
                            : null

                        return (
                          <tr key={area} className="border-b last:border-0">
                            <td className="py-2 pr-4 text-gray-700">{AREA_LABELS[area]}</td>
                            <td className="py-2 pr-4 text-right">
                              {score !== undefined ? (
                                <span className={`font-semibold ${scoreColor(score)}`}>{score}</span>
                              ) : (
                                <span className="text-gray-400">—</span>
                              )}
                            </td>
                            <td className="py-2 text-right">{deltaBadge(delta)}</td>
                          </tr>
                        )
                      })}
                    </tbody>
                  </table>
                </div>

                {run.top_gaps && run.top_gaps.length > 0 && (
                  <div className="mt-4 p-3 bg-amber-50 border border-amber-200 rounded-lg">
                    <p className="text-xs font-semibold text-amber-800 mb-2">Top-Lücken</p>
                    <ul className="space-y-1">
                      {run.top_gaps.map((gap) => (
                        <li key={gap.area} className="flex justify-between text-xs text-amber-700">
                          <span>{gap.area_title}</span>
                          <span className="font-semibold">{gap.score}%</span>
                        </li>
                      ))}
                    </ul>
                  </div>
                )}
              </CardContent>
            )}
          </Card>
        )
      })}
    </div>
  )
}

// ─── Wizard-Schicht für den Re-Assessment-Flow ───────────────────────────────

interface WizardProps {
  runId: string
  onComplete: () => void
  onCancel: () => void
}

function ReassessmentWizard({ runId, onComplete, onCancel }: WizardProps) {
  const [questions, setQuestions] = useState<Question[]>([])
  const [stepIdx, setStepIdx] = useState(0)
  const [comment, setComment] = useState('')
  const [answers, setAnswers] = useState<Record<string, number>>({})
  const [loading, setLoading] = useState(true)
  const [submitting, setSubmitting] = useState(false)

  useState(() => {
    fetch('/api/v1/public/nis2-assessment/questions')
      .then((r) => r.json())
      .then((d: { questions: Question[] }) => {
        setQuestions(d.questions)
        setLoading(false)
      })
      .catch(() => setLoading(false))
  })

  const submitAnswer = async (value: number) => {
    if (submitting) return
    const q = questions[stepIdx]
    setSubmitting(true)
    try {
      await apiFetch(`/secvitals/reassess/${runId}/answer`, {
        method: 'POST',
        body: JSON.stringify({ question_id: q.id, value, comment }),
      })
      setAnswers((prev) => ({ ...prev, [q.id]: value }))
      setComment('')
      if (stepIdx + 1 >= questions.length) {
        onComplete()
      } else {
        setStepIdx((i) => i + 1)
      }
    } catch {
      // ignore — User kann nochmal klicken
    } finally {
      setSubmitting(false)
    }
  }

  if (loading || questions.length === 0) {
    return (
      <div className="py-12 text-center text-gray-500 text-sm">Fragen werden geladen…</div>
    )
  }

  const q = questions[stepIdx]
  const answered = Object.keys(answers).length
  const total = questions.length
  const progressPct = Math.round((answered / total) * 100)

  return (
    <div className="space-y-4">
      <div className="flex items-center justify-between text-xs text-gray-500 mb-1">
        <span>Frage {stepIdx + 1} von {total}</span>
        <span>{progressPct}% beantwortet</span>
      </div>
      <div className="w-full bg-gray-200 rounded-full h-1.5">
        <div
          className="bg-indigo-600 h-1.5 rounded-full transition-all"
          style={{ width: `${progressPct}%` }}
        />
      </div>

      <div className="bg-white border rounded-xl p-6 space-y-4">
        <div className="text-xs uppercase text-indigo-600 font-medium">
          {q.area} · {q.nis2_ref}
        </div>
        <h3 className="text-base font-semibold text-gray-900">{q.title}</h3>
        {q.help && <p className="text-sm text-gray-600">{q.help}</p>}

        <div className="space-y-2">
          {VALUE_LABELS.map((label, value) => (
            <button
              key={value}
              onClick={() => void submitAnswer(value)}
              disabled={submitting}
              className="w-full text-left px-4 py-3 rounded-lg border border-gray-200 hover:border-indigo-300 hover:bg-indigo-50 transition-colors flex items-center justify-between disabled:opacity-50"
            >
              <span className="text-sm font-medium text-gray-900">
                <span className="inline-block w-6 text-indigo-600 font-bold">{value}</span> {label}
              </span>
              <ChevronRight className="w-4 h-4 text-gray-400" />
            </button>
          ))}
        </div>

        <textarea
          placeholder="Kommentar (optional)"
          value={comment}
          onChange={(e) => setComment(e.target.value)}
          className="w-full mt-2 text-sm border border-gray-200 rounded-md px-3 py-2"
          rows={2}
        />
      </div>

      <div className="flex justify-between mt-2">
        <button
          onClick={() => setStepIdx((i) => Math.max(i - 1, 0))}
          disabled={stepIdx === 0}
          className="text-sm text-gray-600 disabled:text-gray-300 inline-flex items-center gap-1"
        >
          <ChevronLeft className="w-4 h-4" /> Zurück
        </button>
        <button
          onClick={onCancel}
          className="text-sm text-gray-500 hover:text-gray-700"
        >
          Abbrechen
        </button>
      </div>
    </div>
  )
}

// ─── Hauptseite ───────────────────────────────────────────────────────────────

export default function NIS2ReassessmentPage() {
  const navigate = useNavigate()
  const queryClient = useQueryClient()
  const [activeRunId, setActiveRunId] = useState<string | null>(null)
  const [cooldownMsg, setCooldownMsg] = useState<string | null>(null)

  const {
    data: historyData,
    error: historyError,
    isLoading,
  } = useQuery<HistoryResponse>({
    queryKey: ['nis2-history'],
    queryFn: () => apiFetch<HistoryResponse>('/secvitals/history'),
  })

  const startMutation = useMutation({
    mutationFn: () =>
      apiFetch<{ run_id: string }>('/secvitals/reassess', { method: 'POST', body: '{}' }),
    onSuccess: (data) => {
      setCooldownMsg(null)
      setActiveRunId(data.run_id)
    },
    onError: (err: unknown) => {
      if (err instanceof Error && err.message.includes('REASSESSMENT_COOLDOWN')) {
        setCooldownMsg(err.message)
      } else if (err instanceof Error && err.message.includes('cooldown:')) {
        setCooldownMsg(err.message)
      }
    },
  })

  const handleComplete = () => {
    setActiveRunId(null)
    void queryClient.invalidateQueries({ queryKey: ['nis2-history'] })
  }

  if (historyError instanceof FeatureLockedError) {
    return (
      <div className="p-6">
        <PageHeader
          title="NIS2-Re-Assessment History"
          description="Verfolgen Sie die Entwicklung Ihrer NIS2-Compliance über mehrere Bewertungen."
        />
        <ProGate error={historyError}>
          <div />
        </ProGate>
      </div>
    )
  }

  const runs = historyData?.runs ?? []
  const latestRun = runs[0]

  // Cooldown: letzter Run existiert + completed_at vorhanden?
  const lastCompleted =
    latestRun?.completed_at ? new Date(latestRun.completed_at) : null
  const cooldownExpires = lastCompleted
    ? new Date(lastCompleted.getTime() + COOLDOWN_DAYS * 24 * 60 * 60 * 1000)
    : null
  const inCooldown = cooldownExpires ? cooldownExpires > new Date() : false

  // Aktiver Wizard-Flow.
  if (activeRunId) {
    return (
      <div className="max-w-2xl mx-auto px-6 py-8">
        <div className="flex items-center gap-3 mb-6">
          <Shield className="w-5 h-5 text-indigo-600" />
          <h1 className="text-lg font-semibold text-gray-900">NIS2-Re-Assessment</h1>
          <Badge variant="secondary">Run #{(latestRun?.run_number ?? 0) + 1}</Badge>
        </div>
        <ReassessmentWizard
          runId={activeRunId}
          onComplete={handleComplete}
          onCancel={() => setActiveRunId(null)}
        />
      </div>
    )
  }

  return (
    <div className="p-6 max-w-4xl mx-auto space-y-6">
      <PageHeader
        title="NIS2-Re-Assessment History"
        description="Verfolgen Sie die Entwicklung Ihrer NIS2-Compliance über mehrere Bewertungen. Alle 90 Tage empfehlen wir eine neue Bewertung."
        action={
          <Button
            onClick={() => {
              if (!inCooldown) {
                startMutation.mutate()
              }
            }}
            disabled={inCooldown || startMutation.isPending}
            className="gap-2"
          >
            <RefreshCw className={`w-4 h-4 ${startMutation.isPending ? 'animate-spin' : ''}`} />
            Neu bewerten
          </Button>
        }
      />

      {/* Cooldown-Hinweis */}
      {inCooldown && cooldownExpires && (
        <div className="flex items-start gap-3 p-4 bg-amber-50 border border-amber-200 rounded-xl">
          <Clock className="w-4 h-4 text-amber-600 mt-0.5 shrink-0" />
          <p className="text-sm text-amber-800">
            Die nächste Bewertung ist frühestens am{' '}
            <strong>{formatDate(cooldownExpires.toISOString())}</strong> möglich.
            Zwischen zwei Re-Assessments müssen mindestens {COOLDOWN_DAYS} Tage liegen.
          </p>
        </div>
      )}

      {/* Cooldown-Fehler aus Mutation (Server-seitig) */}
      {cooldownMsg && !inCooldown && (
        <div className="flex items-start gap-3 p-4 bg-amber-50 border border-amber-200 rounded-xl">
          <AlertTriangle className="w-4 h-4 text-amber-600 mt-0.5 shrink-0" />
          <p className="text-sm text-amber-800">{cooldownMsg}</p>
        </div>
      )}

      {/* Leer-Zustand */}
      {!isLoading && runs.length === 0 && (
        <Card>
          <CardContent className="py-12 text-center space-y-3">
            <Shield className="w-10 h-10 text-gray-300 mx-auto" />
            <p className="text-sm font-medium text-gray-600">Noch keine Re-Assessments</p>
            <p className="text-xs text-gray-400">
              Starten Sie Ihr erstes Re-Assessment, um Ihre NIS2-Compliance zu messen.
            </p>
            <Button
              size="sm"
              onClick={() => startMutation.mutate()}
              disabled={startMutation.isPending}
              className="gap-2"
            >
              <RefreshCw className={`w-4 h-4 ${startMutation.isPending ? 'animate-spin' : ''}`} />
              Erstes Assessment starten
            </Button>
          </CardContent>
        </Card>
      )}

      {/* Übersichts-Score-Karte (letzter Run) */}
      {latestRun?.overall_score !== undefined && latestRun.completed_at && (
        <div className="grid grid-cols-1 sm:grid-cols-3 gap-4">
          <Card>
            <CardContent className="py-4 text-center">
              <div className={`text-4xl font-bold ${scoreColor(latestRun.overall_score)}`}>
                {latestRun.overall_score}
              </div>
              <div className="text-xs text-gray-500 mt-1">Letzter Gesamt-Score</div>
            </CardContent>
          </Card>
          <Card>
            <CardContent className="py-4 text-center">
              <div className="text-4xl font-bold text-gray-800">{runs.length}</div>
              <div className="text-xs text-gray-500 mt-1">Assessments gesamt</div>
            </CardContent>
          </Card>
          {runs.length >= 2 && runs[0].overall_score !== undefined && runs[1].overall_score !== undefined && (
            <Card>
              <CardContent className="py-4 text-center">
                <div className="flex items-center justify-center gap-1 mt-1">
                  {deltaBadge(runs[0].overall_score - runs[1].overall_score)}
                  <span className="text-xs text-gray-500 ml-1">vs. vorheriger Run</span>
                </div>
                <div className="text-xs text-gray-500 mt-1">Score-Entwicklung</div>
              </CardContent>
            </Card>
          )}
        </div>
      )}

      {/* History-Liste */}
      {runs.length > 0 && (
        <div className="space-y-2">
          <h2 className="text-sm font-semibold text-gray-700">Alle Bewertungen</h2>
          <HistoryTable runs={runs} />
        </div>
      )}

      {/* Link zum öffentlichen Wizard (für neuen anonymen Check) */}
      <div className="text-center">
        <button
          onClick={() => navigate('/nis2-check')}
          className="text-xs text-gray-400 hover:text-gray-600 underline"
        >
          Zum öffentlichen NIS2-Wizard →
        </button>
      </div>
    </div>
  )
}
