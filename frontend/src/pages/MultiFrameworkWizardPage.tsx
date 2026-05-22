import { useEffect, useState } from 'react'
import { Shield, ChevronRight, ChevronLeft, AlertTriangle, CheckCircle2, Lock } from 'lucide-react'
import { Spinner } from '../components/Spinner'
import { apiFetch, FeatureLockedError } from '../api/client'

// Sprint 28 / S28-4: Multi-Framework-Assessment-Wizard (NIS2 + ISO27001 + DSGVO-TOM).
// Route: /nis2-check/multi
// ProGate: FeatureNIS2Reporting — Start des Runs erfordert Pro-Lizenz.
// Die Fragen-Liste ist public abrufbar (kein Gate).
//
// Flow:
//   1. GET /api/v1/public/nis2-assessment/multi/questions (public)
//   2. POST /api/v1/secvitals/nis2-assessment/multi/start (authenticated, ProGate)
//   3. POST /api/v1/secvitals/nis2-assessment/multi/:token/answer (authenticated)
//   4. GET  /api/v1/secvitals/nis2-assessment/multi/:token/result (authenticated)

interface MultiFrameworkQuestion {
  id: string
  framework: string
  cross_frameworks?: string[]
  area: string
  text: string
  help_text: string
  weight: number
  ref: string
}

interface AnswerEntry {
  value: number
  comment?: string
}

interface MultiGap {
  framework: string
  area: string
  area_title: string
  score: number
}

interface MultiFrameworkScore {
  nis2_score: number
  iso27001_score: number
  dsgvo_score: number
  overall_score: number
  by_framework: Record<string, number>
  top_gaps: MultiGap[]
}

interface MultiRun {
  token: string
  answers: Record<string, AnswerEntry>
  score?: MultiFrameworkScore
  completed_at?: string
  expires_at: string
}

const VALUE_LABELS = [
  'Nicht implementiert',
  'In Planung',
  'Teilweise umgesetzt',
  'Weitgehend umgesetzt',
  'Vollständig + getestet',
]

const FRAMEWORK_LABELS: Record<string, string> = {
  nis2:      'NIS2',
  iso27001:  'ISO 27001',
  dsgvo_tom: 'DSGVO-TOM',
}

const FRAMEWORK_COLORS: Record<string, string> = {
  nis2:      'text-indigo-600',
  iso27001:  'text-emerald-600',
  dsgvo_tom: 'text-violet-600',
}

const FRAMEWORK_BG: Record<string, string> = {
  nis2:      'bg-indigo-50 border-indigo-200',
  iso27001:  'bg-emerald-50 border-emerald-200',
  dsgvo_tom: 'bg-violet-50 border-violet-200',
}

const FRAMEWORK_PROGRESS: Record<string, string> = {
  nis2:      'bg-indigo-600',
  iso27001:  'bg-emerald-600',
  dsgvo_tom: 'bg-violet-600',
}

function scoreColor(score: number): string {
  if (score >= 70) return 'text-green-600'
  if (score >= 40) return 'text-amber-600'
  return 'text-red-600'
}

export default function MultiFrameworkWizardPage() {
  const [questions, setQuestions] = useState<MultiFrameworkQuestion[]>([])
  const [run, setRun] = useState<MultiRun | null>(null)
  const [stepIdx, setStepIdx] = useState(0)
  const [comment, setComment] = useState('')
  const [loading, setLoading] = useState(true)
  const [finished, setFinished] = useState(false)
  const [result, setResult] = useState<MultiRun | null>(null)
  const [proGateError, setProGateError] = useState(false)
  const [error, setError] = useState<string | null>(null)

  // Initialer Load: Fragen abrufen + Run starten.
  useEffect(() => {
    const init = async () => {
      try {
        // 1. Fragen laden (public)
        const qResp = await fetch('/api/v1/public/nis2-assessment/multi/questions')
        if (!qResp.ok) throw new Error('Fragen konnten nicht geladen werden.')
        const qData = await qResp.json()
        setQuestions(qData.questions ?? [])

        // 2. Laufenden Run aus sessionStorage wiederverwenden.
        const existing = sessionStorage.getItem('vakt_multi_token')
        if (existing) {
          try {
            const rResp = await apiFetch<MultiRun>(
              `/secvitals/nis2-assessment/multi/${existing}/result`
            )
            setRun(rResp)
            if (rResp.completed_at) {
              setFinished(true)
              setResult(rResp)
            } else {
              const firstUnanswered = (qData.questions as MultiFrameworkQuestion[]).findIndex(
                (q) => !rResp.answers[q.id]
              )
              setStepIdx(firstUnanswered >= 0 ? firstUnanswered : 0)
            }
            return
          } catch {
            sessionStorage.removeItem('vakt_multi_token')
          }
        }

        // 3. Neuen Run starten (authenticated, ProGate).
        const startResp = await apiFetch<MultiRun>('/secvitals/nis2-assessment/multi/start', {
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify({ referrer: document.referrer || '' }),
        })
        sessionStorage.setItem('vakt_multi_token', startResp.token)
        setRun(startResp)
      } catch (err) {
        if (err instanceof FeatureLockedError) {
          setProGateError(true)
        } else {
          setError(err instanceof Error ? err.message : 'Unbekannter Fehler')
        }
      } finally {
        setLoading(false)
      }
    }
    void init()
  }, [])

  const submitAnswer = async (value: number) => {
    if (!run) return
    const q = questions[stepIdx]
    try {
      const updated = await apiFetch<MultiRun>(
        `/secvitals/nis2-assessment/multi/${run.token}/answer`,
        {
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify({ question_id: q.id, value, comment }),
        }
      )
      setRun(updated)
      setComment('')

      if (updated.completed_at) {
        const finalResult = await apiFetch<MultiRun>(
          `/secvitals/nis2-assessment/multi/${run.token}/result`
        )
        setResult(finalResult)
        setFinished(true)
      } else {
        setStepIdx((i) => Math.min(i + 1, questions.length - 1))
      }
    } catch {
      // ignore — User kann nochmal klicken
    }
  }

  if (loading || questions.length === 0) {
    return (
      <div className="min-h-screen flex items-center justify-center bg-gray-50">
        <Spinner size="lg" />
      </div>
    )
  }

  if (proGateError) {
    return <ProGateView />
  }

  if (error) {
    return (
      <div className="min-h-screen flex items-center justify-center bg-gray-50">
        <div className="bg-white border rounded-xl p-8 max-w-md text-center space-y-3">
          <AlertTriangle className="w-8 h-8 text-amber-500 mx-auto" />
          <p className="text-gray-700 text-sm">{error}</p>
          <button
            onClick={() => window.location.reload()}
            className="text-sm text-indigo-600 hover:underline"
          >
            Neu laden
          </button>
        </div>
      </div>
    )
  }

  if (!run) {
    return (
      <div className="min-h-screen flex items-center justify-center bg-gray-50">
        <Spinner size="lg" />
      </div>
    )
  }

  if (finished && result) {
    return <ResultView result={result} />
  }

  const q = questions[stepIdx]
  const answered = Object.keys(run.answers).length
  const total = questions.length
  const progressPct = Math.round((answered / total) * 100)

  return (
    <div className="min-h-screen bg-gray-50">
      <header className="bg-white border-b">
        <div className="max-w-2xl mx-auto px-6 py-4 flex items-center gap-3">
          <Shield className="w-6 h-6 text-indigo-600" />
          <div>
            <h1 className="text-lg font-semibold text-gray-900">
              Multi-Framework-Assessment
            </h1>
            <p className="text-xs text-gray-500">
              NIS2 + ISO 27001 + DSGVO-TOM · {total} Fragen · ~25 Minuten
            </p>
          </div>
        </div>
      </header>

      <main className="max-w-2xl mx-auto px-6 py-8">
        {/* Fortschritt */}
        <div className="mb-6">
          <div className="flex justify-between text-xs text-gray-500 mb-1">
            <span>Frage {stepIdx + 1} von {total}</span>
            <span>{progressPct}% beantwortet</span>
          </div>
          <div className="w-full bg-gray-200 rounded-full h-1.5">
            <div
              className="bg-indigo-600 h-1.5 rounded-full transition-all"
              style={{ width: `${progressPct}%` }}
            />
          </div>
        </div>

        {/* Frage */}
        <div className="bg-white border rounded-xl p-6 space-y-4">
          <div className="flex items-center gap-2 flex-wrap">
            <span className={`text-xs uppercase font-semibold ${FRAMEWORK_COLORS[q.framework] ?? 'text-gray-600'}`}>
              {FRAMEWORK_LABELS[q.framework] ?? q.framework}
            </span>
            {(q.cross_frameworks ?? []).map((cf) => (
              <span
                key={cf}
                className={`text-xs px-1.5 py-0.5 rounded border font-medium ${FRAMEWORK_COLORS[cf] ?? 'text-gray-500'} ${FRAMEWORK_BG[cf] ?? 'bg-gray-50 border-gray-200'}`}
              >
                +{FRAMEWORK_LABELS[cf] ?? cf}
              </span>
            ))}
            <span className="text-xs text-gray-400 ml-auto">{q.ref}</span>
          </div>

          <h2 className="text-xl font-semibold text-gray-900">{q.text}</h2>
          {q.help_text && <p className="text-sm text-gray-600">{q.help_text}</p>}

          <div className="space-y-2">
            {VALUE_LABELS.map((label, value) => {
              const isAnswered = run.answers[q.id]?.value === value
              return (
                <button
                  key={value}
                  onClick={() => void submitAnswer(value)}
                  className={`w-full text-left px-4 py-3 rounded-lg border transition-colors flex items-center justify-between ${
                    isAnswered
                      ? 'border-indigo-400 bg-indigo-50'
                      : 'border-gray-200 hover:border-indigo-300 hover:bg-indigo-50'
                  }`}
                >
                  <span className="text-sm font-medium text-gray-900">
                    <span className="inline-block w-6 text-indigo-600 font-bold">{value}</span>
                    {' '}{label}
                  </span>
                  <ChevronRight className="w-4 h-4 text-gray-400 shrink-0" />
                </button>
              )
            })}
          </div>

          <textarea
            placeholder="Kommentar (optional)"
            value={comment}
            onChange={(e) => setComment(e.target.value)}
            className="w-full mt-2 text-sm border border-gray-200 rounded-md px-3 py-2 focus:outline-none focus:ring-1 focus:ring-indigo-400"
            rows={2}
          />
        </div>

        <div className="flex justify-between mt-6">
          <button
            onClick={() => setStepIdx((i) => Math.max(i - 1, 0))}
            disabled={stepIdx === 0}
            className="text-sm text-gray-600 disabled:text-gray-300 inline-flex items-center gap-1"
          >
            <ChevronLeft className="w-4 h-4" /> Zurück
          </button>
          <span className="text-xs text-gray-400">
            Antworten werden sicher gespeichert · 7 Tage Lebensdauer
          </span>
        </div>
      </main>
    </div>
  )
}

// ── Result-Screen ─────────────────────────────────────────────────────────────

function FrameworkScoreBar({
  framework,
  score,
}: {
  framework: string
  score: number
}) {
  const label = FRAMEWORK_LABELS[framework] ?? framework
  const color = scoreColor(score)
  const barColor = FRAMEWORK_PROGRESS[framework] ?? 'bg-indigo-600'

  return (
    <div className="space-y-1">
      <div className="flex justify-between text-sm">
        <span className="font-medium text-gray-800">{label}</span>
        <span className={`font-bold ${color}`}>{score}%</span>
      </div>
      <div className="w-full bg-gray-200 rounded-full h-2">
        <div
          className={`${barColor} h-2 rounded-full transition-all`}
          style={{ width: `${score}%` }}
        />
      </div>
    </div>
  )
}

function ResultView({ result }: { result: MultiRun }) {
  const score = result.score
  if (!score) {
    return (
      <div className="min-h-screen flex items-center justify-center bg-gray-50">
        <p className="text-gray-500 text-sm">Ergebnis wird berechnet…</p>
      </div>
    )
  }

  const overall = score.overall_score
  const overallColor = scoreColor(overall)

  return (
    <div className="min-h-screen bg-gray-50">
      <header className="bg-white border-b">
        <div className="max-w-2xl mx-auto px-6 py-4 flex items-center gap-3">
          <Shield className="w-6 h-6 text-indigo-600" />
          <h1 className="text-lg font-semibold text-gray-900">
            Multi-Framework-Ergebnis
          </h1>
        </div>
      </header>

      <main className="max-w-2xl mx-auto px-6 py-8 space-y-6">
        {/* Gesamt-Score */}
        <div className="bg-white border rounded-xl p-8 text-center">
          <div className={`text-6xl font-bold ${overallColor}`}>{overall}</div>
          <div className="text-sm text-gray-500 mt-2">Gesamt-Reifegrad (0–100)</div>
          <p className="text-sm text-gray-700 mt-4">
            {overall >= 70 ? (
              <>
                <CheckCircle2 className="w-4 h-4 inline mr-1 text-green-500" />
                Solide Basis. Optimierung in den hervorgehobenen Bereichen empfohlen.
              </>
            ) : overall >= 40 ? (
              <>
                <AlertTriangle className="w-4 h-4 inline mr-1 text-amber-500" />
                Wichtige Lücken — strukturierte Compliance-Roadmap erforderlich.
              </>
            ) : (
              <>
                <AlertTriangle className="w-4 h-4 inline mr-1 text-red-500" />
                Akuter Handlungsbedarf in mehreren Frameworks.
              </>
            )}
          </p>
        </div>

        {/* Scores per Framework */}
        <div className="bg-white border rounded-xl p-6 space-y-4">
          <h2 className="text-base font-semibold text-gray-900">Scores pro Framework</h2>
          <FrameworkScoreBar framework="nis2"      score={score.nis2_score} />
          <FrameworkScoreBar framework="iso27001"  score={score.iso27001_score} />
          <FrameworkScoreBar framework="dsgvo_tom" score={score.dsgvo_score} />
        </div>

        {/* Top-Gaps */}
        {score.top_gaps.length > 0 && (
          <div className="bg-white border rounded-xl p-6">
            <h2 className="text-base font-semibold text-gray-900 mb-3">
              Top-{score.top_gaps.length} Verbesserungsbereiche
            </h2>
            <ul className="space-y-2">
              {score.top_gaps.map((gap, i) => (
                <li key={`${gap.framework}-${gap.area}`} className="flex items-start justify-between gap-2 text-sm">
                  <div className="flex items-start gap-2">
                    <span className="text-gray-400 font-mono shrink-0 w-4">{i + 1}.</span>
                    <div>
                      <span className="text-gray-700">{gap.area_title}</span>
                      <span className={`ml-2 text-xs font-medium ${FRAMEWORK_COLORS[gap.framework] ?? 'text-gray-500'}`}>
                        {FRAMEWORK_LABELS[gap.framework] ?? gap.framework}
                      </span>
                    </div>
                  </div>
                  <span className={`font-semibold shrink-0 ${gap.score < 40 ? 'text-red-600' : 'text-amber-600'}`}>
                    {gap.score}%
                  </span>
                </li>
              ))}
            </ul>
          </div>
        )}

        {/* Cross-Mapping-Hinweis */}
        <div className="bg-amber-50 border border-amber-200 rounded-xl p-5">
          <h3 className="text-sm font-semibold text-amber-900 mb-1">
            Cross-Framework-Synergie
          </h3>
          <p className="text-xs text-amber-800 leading-relaxed">
            Viele Maßnahmen aus diesem Assessment decken gleichzeitig NIS2, ISO 27001 und DSGVO ab.
            Priorisieren Sie Bereiche mit mehreren Framework-Tags — ein Fix schließt mehrere Lücken.
          </p>
        </div>

        {/* CTA */}
        <div className="bg-indigo-50 border border-indigo-200 rounded-xl p-6 space-y-3">
          <h3 className="text-base font-semibold text-indigo-900">Nächste Schritte</h3>
          <p className="text-sm text-indigo-800">
            Übernehmen Sie Ihr Ergebnis in Vakt Comply und schließen Sie Lücken
            systematisch — mit Roadmap, Evidence-Tracking und Audit-Export.
          </p>
          <a
            href="/secvitals"
            className="inline-block bg-indigo-600 text-white text-sm font-medium px-5 py-2 rounded-lg hover:bg-indigo-700"
          >
            Zu Vakt Comply →
          </a>
          <p className="text-xs text-indigo-700">
            Antworten werden bei Anmeldung automatisch als initialer Compliance-Status übernommen.
          </p>
        </div>
      </main>
    </div>
  )
}

// ── Pro-Gate-View ─────────────────────────────────────────────────────────────

function ProGateView() {
  return (
    <div className="min-h-screen bg-gray-50">
      <header className="bg-white border-b">
        <div className="max-w-2xl mx-auto px-6 py-4 flex items-center gap-3">
          <Shield className="w-6 h-6 text-indigo-600" />
          <h1 className="text-lg font-semibold text-gray-900">
            Multi-Framework-Assessment
          </h1>
        </div>
      </header>
      <main className="max-w-2xl mx-auto px-6 py-12">
        <div className="bg-white border rounded-xl p-8 text-center space-y-4">
          <div className="mx-auto w-12 h-12 rounded-full bg-indigo-100 flex items-center justify-center">
            <Lock className="w-6 h-6 text-indigo-600" />
          </div>
          <h2 className="text-xl font-semibold text-gray-900">Pro-Feature</h2>
          <p className="text-sm text-gray-600 max-w-md mx-auto leading-relaxed">
            Das kombinierte NIS2 + ISO 27001 + DSGVO-TOM Assessment ist in der
            Community Edition nicht verfügbar. Es erfordert eine Vakt-Pro-Lizenz mit
            dem Feature <span className="font-mono text-xs bg-gray-100 px-1.5 py-0.5 rounded">nis2_reporting</span>.
          </p>
          <div className="flex gap-3 justify-center pt-2">
            <a
              href="/settings/license"
              className="inline-block bg-indigo-600 text-white text-sm font-medium px-5 py-2 rounded-lg hover:bg-indigo-700"
            >
              Lizenz aktivieren
            </a>
            <a
              href="/nis2-check"
              className="inline-block bg-white border text-gray-700 text-sm font-medium px-5 py-2 rounded-lg hover:bg-gray-50"
            >
              NIS2-Only-Assessment (kostenlos)
            </a>
          </div>
        </div>
      </main>
    </div>
  )
}
