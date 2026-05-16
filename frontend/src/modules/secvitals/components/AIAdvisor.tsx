import { useState } from 'react'
import { Sparkles, Loader2, AlertTriangle } from 'lucide-react'
import { apiFetch, FeatureLockedError } from '../../../api/client'
import { ProGate } from '../../../shared/components/ProGate'

interface AdviceResponse {
  advice: string
}

interface Props {
  /** When false the component renders a "not configured" notice instead of the action button. */
  aiAvailable: boolean
}

export function AIAdvisor({ aiAvailable }: Props) {
  const [loading, setLoading] = useState(false)
  const [advice, setAdvice] = useState<string | null>(null)
  const [error, setError] = useState<Error | null>(null)

  const loadAdvice = async () => {
    setLoading(true)
    setError(null)
    setAdvice(null)
    try {
      const data = await apiFetch<AdviceResponse>('/secvitals/ai/advice', { method: 'POST' })
      setAdvice(data.advice)
    } catch (e: unknown) {
      setError(e instanceof Error ? e : new Error('Unbekannter Fehler'))
    } finally {
      setLoading(false)
    }
  }

  return (
    <div className="rounded-xl border border-border bg-surface p-5 space-y-4">
      {/* Header */}
      <div className="flex items-center gap-2">
        <Sparkles className="w-4 h-4 text-brand shrink-0" />
        <h2 className="text-sm font-semibold text-primary">KI-Compliance-Berater</h2>
      </div>

      {/* Not configured */}
      {!aiAvailable && (
        <p className="text-xs text-secondary italic">
          KI nicht konfiguriert — <code className="text-primary">VAKT_AI_PROVIDER</code> setzen
        </p>
      )}

      {/* Available: action button */}
      {aiAvailable && !advice && !loading && !error && (
        <button
          onClick={() => void loadAdvice()}
          className="w-full text-sm font-medium text-brand border border-brand/40 rounded-lg py-2 px-4 hover:bg-brand/10 transition-colors"
        >
          Empfehlungen laden
        </button>
      )}

      {/* Loading */}
      {loading && (
        <div className="flex items-center gap-2 text-sm text-secondary">
          <Loader2 className="w-4 h-4 animate-spin shrink-0" />
          <span>Analysiere Compliance-Daten...</span>
        </div>
      )}

      {/* Error */}
      {error && !loading && (
        error instanceof FeatureLockedError
          ? <ProGate error={error}>{null}</ProGate>
          : <div className="flex items-start gap-2 text-xs text-red-400 bg-red-500/10 border border-red-500/20 rounded-lg px-3 py-2">
              <AlertTriangle className="w-3.5 h-3.5 mt-0.5 shrink-0" />
              <span>KI temporär nicht verfügbar</span>
            </div>
      )}

      {/* Result */}
      {advice && !loading && (
        <div className="space-y-3">
          <ol className="space-y-2">
            {advice
              .split('\n')
              .map((line) => line.trim())
              .filter(Boolean)
              .map((line, i) => {
                // Strip leading "1. ", "2. " etc. if present, then display as numbered list
                const stripped = line.replace(/^\d+\.\s*/, '')
                return (
                  <li key={i} className="flex items-start gap-2 text-xs text-primary leading-relaxed">
                    <span className="font-bold text-brand shrink-0 w-4 text-right">{i + 1}.</span>
                    <span>{stripped}</span>
                  </li>
                )
              })}
          </ol>
          <button
            onClick={() => void loadAdvice()}
            className="text-xs text-secondary hover:text-brand transition-colors"
          >
            Neu laden
          </button>
        </div>
      )}
    </div>
  )
}
