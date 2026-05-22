import { useEffect, useState } from 'react'
import { Shield, ChevronRight, ChevronLeft, AlertTriangle, CheckCircle2 } from 'lucide-react'

// Sprint 19 / S19-4 + S19-5: Public-Wizard-Page für NIS2-Self-Assessment.
// Lebt unter /nis2-check (kein Layout-Wrapper, eigenes leichtgewichtiges
// Branding). Mobile-first.
//
// Flow:
//   1. Beim Mount → /api/v1/public/nis2-assessment/start (anonym, holt Token)
//   2. Pro Frage → /api/v1/public/nis2-assessment/answer (speichert + Live-Score)
//   3. Nach 30. Frage → Result-Section mit Top-3-Gaps + CTA "Sign up"
//
// CE-Schicht (S19-4..7). Pro-Schicht (S19-8..10) wird nach Sign-up im
// authentifizierten Layout-Wrapper geliefert.

interface Question {
  id: string
  area: string
  title: string
  help: string
  nis2_ref: string
  weight: number
}

interface AnswerEntry {
  value: number
  comment?: string
}

interface Run {
  token: string
  answers: Record<string, AnswerEntry>
  score?: number
  score_by_area?: Record<string, number>
  completed_at?: string
  expires_at: string
}

interface Gap {
  area: string
  area_title: string
  score: number
}

interface ResultResponse extends Run {
  top_gaps: Gap[]
}

const VALUE_LABELS = [
  'Nicht implementiert',
  'In Planung',
  'Teilweise umgesetzt',
  'Weitgehend umgesetzt',
  'Vollständig + getestet',
]

export default function NIS2WizardPage() {
  const [questions, setQuestions] = useState<Question[]>([])
  const [run, setRun] = useState<Run | null>(null)
  const [stepIdx, setStepIdx] = useState(0)
  const [comment, setComment] = useState('')
  const [loading, setLoading] = useState(true)
  const [finished, setFinished] = useState(false)
  const [result, setResult] = useState<ResultResponse | null>(null)

  // Initialer Load: Questions + Run-Token (sofern noch nicht in localStorage).
  useEffect(() => {
    const init = async () => {
      try {
        const qResp = await fetch('/api/v1/public/nis2-assessment/questions')
        const qData = await qResp.json()
        setQuestions(qData.questions)

        // Token aus localStorage wiederverwenden (für Wiederbesuch).
        const existing = localStorage.getItem('vakt_nis2_token')
        if (existing) {
          const rResp = await fetch(`/api/v1/public/nis2-assessment/result?token=${existing}`)
          if (rResp.ok) {
            const r: ResultResponse = await rResp.json()
            setRun(r)
            // Springe zur ersten unbeantworteten Frage.
            const firstUnanswered = qData.questions.findIndex((q: Question) => !r.answers[q.id])
            setStepIdx(firstUnanswered >= 0 ? firstUnanswered : qData.questions.length - 1)
            if (r.completed_at) {
              setFinished(true)
              setResult(r)
            }
            setLoading(false)
            return
          }
        }

        const sResp = await fetch('/api/v1/public/nis2-assessment/start', {
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify({ referrer: document.referrer || '' }),
        })
        const sData: Run = await sResp.json()
        localStorage.setItem('vakt_nis2_token', sData.token)
        setRun(sData)
      } catch {
        // Stream-Fehler unkritisch — User sieht Loading-State + Retry-Button.
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
      const resp = await fetch('/api/v1/public/nis2-assessment/answer', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ token: run.token, question_id: q.id, value, comment }),
      })
      const updated: Run = await resp.json()
      setRun(updated)
      setComment('')
      if (updated.completed_at) {
        // Final fetch with top_gaps
        const r = await fetch(`/api/v1/public/nis2-assessment/result?token=${run.token}`)
        const rData: ResultResponse = await r.json()
        setResult(rData)
        setFinished(true)
      } else {
        setStepIdx((i) => Math.min(i + 1, questions.length - 1))
      }
    } catch {
      // ignore — User kann nochmal klicken
    }
  }

  if (loading || !run || questions.length === 0) {
    return (
      <div className="min-h-screen flex items-center justify-center bg-gray-50">
        <div className="animate-spin rounded-full h-8 w-8 border-b-2 border-indigo-600" />
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
  const liveScore = run.score ?? 0

  return (
    <div className="min-h-screen bg-gray-50">
      <header className="bg-white border-b">
        <div className="max-w-2xl mx-auto px-6 py-4 flex items-center gap-3">
          <Shield className="w-6 h-6 text-indigo-600" />
          <div>
            <h1 className="text-lg font-semibold text-gray-900">NIS2-Self-Assessment</h1>
            <p className="text-xs text-gray-500">30 Fragen · ~10 Minuten · keine Anmeldung nötig</p>
          </div>
        </div>
      </header>

      <main className="max-w-2xl mx-auto px-6 py-8">
        {/* Progress */}
        <div className="mb-6">
          <div className="flex justify-between text-xs text-gray-500 mb-1">
            <span>Frage {answered + (run.answers[q.id] ? 0 : 1)} von {total}</span>
            <span>{progressPct}% beantwortet · Live-Score: {liveScore}</span>
          </div>
          <div className="w-full bg-gray-200 rounded-full h-1.5">
            <div className="bg-indigo-600 h-1.5 rounded-full transition-all" style={{ width: `${progressPct}%` }} />
          </div>
        </div>

        {/* Question */}
        <div className="bg-white border rounded-xl p-6 space-y-4">
          <div className="text-xs uppercase text-indigo-600 font-medium">{q.area} · {q.nis2_ref}</div>
          <h2 className="text-xl font-semibold text-gray-900">{q.title}</h2>
          {q.help && <p className="text-sm text-gray-600">{q.help}</p>}

          <div className="space-y-2">
            {VALUE_LABELS.map((label, value) => (
              <button
                key={value}
                onClick={() => void submitAnswer(value)}
                className="w-full text-left px-4 py-3 rounded-lg border border-gray-200 hover:border-indigo-300 hover:bg-indigo-50 transition-colors flex items-center justify-between"
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

        <div className="flex justify-between mt-6">
          <button
            onClick={() => setStepIdx((i) => Math.max(i - 1, 0))}
            disabled={stepIdx === 0}
            className="text-sm text-gray-600 disabled:text-gray-300 inline-flex items-center gap-1"
          >
            <ChevronLeft className="w-4 h-4" /> Zurück
          </button>
          <span className="text-xs text-gray-400">Antworten werden anonym gespeichert · 7 Tage Lebensdauer</span>
        </div>
      </main>
    </div>
  )
}

function ResultView({ result }: { result: ResultResponse }) {
  const score = result.score ?? 0
  const scoreColor = score >= 70 ? 'text-green-600' : score >= 40 ? 'text-amber-600' : 'text-red-600'
  return (
    <div className="min-h-screen bg-gray-50">
      <header className="bg-white border-b">
        <div className="max-w-2xl mx-auto px-6 py-4 flex items-center gap-3">
          <Shield className="w-6 h-6 text-indigo-600" />
          <h1 className="text-lg font-semibold text-gray-900">Ihr NIS2-Ergebnis</h1>
        </div>
      </header>
      <main className="max-w-2xl mx-auto px-6 py-8 space-y-6">
        <div className="bg-white border rounded-xl p-8 text-center">
          <div className={`text-6xl font-bold ${scoreColor}`}>{score}</div>
          <div className="text-sm text-gray-500 mt-2">NIS2-Reifegrad (0–100)</div>
          <p className="text-sm text-gray-700 mt-4">
            {score >= 70 ? (
              <><CheckCircle2 className="w-4 h-4 inline mr-1 text-green-500" />Solide Basis. Optimierung in den hervorgehobenen Bereichen empfohlen.</>
            ) : score >= 40 ? (
              <><AlertTriangle className="w-4 h-4 inline mr-1 text-amber-500" />Wichtige Lücken — Roadmap für NIS2-Compliance erforderlich.</>
            ) : (
              <><AlertTriangle className="w-4 h-4 inline mr-1 text-red-500" />Akuter Handlungsbedarf vor NIS2-Stichtag.</>
            )}
          </p>
        </div>

        <div className="bg-white border rounded-xl p-6">
          <h2 className="text-base font-semibold text-gray-900 mb-3">Top-3 Lücken</h2>
          <ul className="space-y-2">
            {result.top_gaps.map((gap) => (
              <li key={gap.area} className="flex items-center justify-between text-sm">
                <span className="text-gray-700">{gap.area_title}</span>
                <span className={gap.score < 40 ? 'text-red-600 font-semibold' : 'text-amber-600 font-semibold'}>{gap.score}%</span>
              </li>
            ))}
          </ul>
        </div>

        <div className="bg-indigo-50 border border-indigo-200 rounded-xl p-6 space-y-3">
          <h3 className="text-base font-semibold text-indigo-900">Nächster Schritt</h3>
          <p className="text-sm text-indigo-800">
            Speichern Sie Ihr Ergebnis und schließen Sie die Lücken systematisch — mit Vakt-Compliance-Plattform.
          </p>
          <a
            href={`/auth/signup?nis2_token=${result.token}`}
            className="inline-block bg-indigo-600 text-white text-sm font-medium px-5 py-2 rounded-lg hover:bg-indigo-700"
          >
            Account erstellen + Ergebnis übernehmen →
          </a>
          <p className="text-xs text-indigo-700">
            Bei Account-Erstellung werden Ihre Antworten in Vakt-Comply als initialer Compliance-Status übernommen.
            Spart ~30 Minuten manueller Einrichtung.
          </p>
        </div>
      </main>
    </div>
  )
}
