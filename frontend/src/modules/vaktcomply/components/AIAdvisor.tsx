import { useState, useMemo } from 'react'
import { Sparkles, Loader2, AlertTriangle, Square, ExternalLink } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { useNavigate } from 'react-router-dom'
import { FeatureLockedError } from '../../../api/client'
import { ProGate } from '../../../shared/components/ProGate'
import { useAIStream } from '../../../shared/hooks/useAIStream'
import { LocalLLMBadge } from '../../../shared/components/LocalLLMBadge'
import { TokenCostIndicator } from '../../../shared/components/TokenCostIndicator'
import { CEAICounter } from '../../../shared/components/CEAICounter'

interface SourceRef {
  label: string
  href: string
}

function extractSources(text: string): SourceRef[] {
  const found = new Map<string, string>()

  // NIS2 Art. N — link to vaktcomply compliance overview
  for (const m of text.matchAll(/NIS2\s+Art\.\s*(\d+[a-z]?)/gi)) {
    const label = `NIS2 Art. ${m[1]}`
    if (!found.has(label)) found.set(label, '/vaktcomply?framework=nis2')
  }

  // ISO 27001 A.N.N / ISO27001 A.N
  for (const m of text.matchAll(/ISO\s*27001\s+([A-Z]\.\d+(?:\.\d+)?)/gi)) {
    const label = `ISO 27001 ${m[1]}`
    if (!found.has(label)) found.set(label, '/vaktcomply?framework=iso27001')
  }

  // BSI IT-Grundschutz
  for (const m of text.matchAll(/BSI(?:\s+IT-Grundschutz)?\s+(ORP|APP|SYS|INF|NET|OPS|DER|CON|IND|ISMS)\.\d+/gi)) {
    const label = `BSI ${m[1]}`
    if (!found.has(label)) found.set(label, '/vaktcomply?framework=bsi')
  }

  // DSGVO / GDPR Art. N
  for (const m of text.matchAll(/(?:DSGVO|GDPR)\s+Art\.\s*(\d+)/gi)) {
    const label = `DSGVO Art. ${m[1]}`
    if (!found.has(label)) found.set(label, '/vaktcomply?framework=dsgvo')
  }

  // DORA Art. N
  for (const m of text.matchAll(/DORA\s+Art\.\s*(\d+)/gi)) {
    const label = `DORA Art. ${m[1]}`
    if (!found.has(label)) found.set(label, '/vaktcomply?framework=dora')
  }

  return Array.from(found.entries()).slice(0, 8).map(([label, href]) => ({ label, href }))
}

interface Props {
  /** When false the component renders a "not configured" notice instead of the action button. */
  aiAvailable: boolean
  /** Optional provider hostname für das Local-LLM-Badge (S15-8). */
  providerHost?: string
  /** Optional Modellname für Anzeige im Badge. */
  model?: string
}

// Sprint 15 S15-6/7/8/9: AIAdvisor nutzt jetzt den Streaming-Endpoint,
// zeigt Token/Time-Indikator und Local-LLM-Badge an, und hat einen Stop-Button.
// S58-6: Source-Attribution — extrahiert Compliance-Referenzen als klickbare Chip-Links.
export function AIAdvisor({ aiAvailable, providerHost, model }: Props) {
  const { t } = useTranslation()
  const navigate = useNavigate()
  const { text, isStreaming, error, durationMs, start, stop } = useAIStream()
  const [featureLockedError, setFeatureLockedError] = useState<FeatureLockedError | null>(null)

  const sources = useMemo(() => (text && !isStreaming) ? extractSources(text) : [], [text, isStreaming])

  const startAdvice = async () => {
    setFeatureLockedError(null)
    try {
      await start({
        endpoint: '/vaktcomply/ai/chat/stream',
        system:
          'Du bist ein ISO-27001/NIS2-Compliance-Berater. Antworte auf Deutsch, präzise und handlungsorientiert. Liefere eine nummerierte Liste mit 3–5 priorisierten Maßnahmen für diese Woche.',
        prompt: 'Erstelle einen Wochen-Aktionsplan basierend auf dem aktuellen Compliance-Stand meiner Organisation.',
        maxTokens: 600,
      })
    } catch (e) {
      if (e instanceof FeatureLockedError) {
        setFeatureLockedError(e)
      }
    }
  }

  // Rate-Limit + Quota-Errors aus dem Backend bekommen einen spezifischen Hint.
  const errorMessage = (() => {
    if (!error) return null
    const code = (error as Error & { code?: string }).code
    if (code === 'AI_RATE_LIMITED') return t('ai.stream.rateLimited')
    if (code === 'AI_QUOTA_EXCEEDED') return t('ai.stream.quotaExceeded')
    if (code === 'AI_CE_MONTHLY_LIMIT') return 'KI-Monatslimit (25 Anfragen) erreicht. Upgrade auf Pro für unbegrenzte Anfragen.'
    return null
  })()

  return (
    <div className="rounded-xl border border-border bg-surface p-5 space-y-4">
      {/* Header mit Badge */}
      <div className="flex items-center justify-between gap-2 flex-wrap">
        <div className="flex items-center gap-2">
          <Sparkles className="w-4 h-4 text-brand shrink-0" />
          <h2 className="text-sm font-semibold text-primary">KI-Compliance-Berater</h2>
        </div>
        <div className="flex items-center gap-2">
          <CEAICounter />
          {aiAvailable && <LocalLLMBadge providerHost={providerHost} model={model} />}
        </div>
      </div>

      {/* Not configured */}
      {!aiAvailable && (
        <p className="text-xs text-secondary italic">
          KI nicht konfiguriert — <code className="text-primary">VAKT_AI_PROVIDER</code> setzen
        </p>
      )}

      {/* Idle: action button */}
      {aiAvailable && !text && !isStreaming && !error && (
        <button
          onClick={() => void startAdvice()}
          className="w-full text-sm font-medium text-brand border border-brand/40 rounded-lg py-2 px-4 hover:bg-brand/10 transition-colors"
        >
          Empfehlungen laden
        </button>
      )}

      {/* Streaming: live text + stop button */}
      {(isStreaming || text) && !error && (
        <div className="space-y-3">
          <div className="whitespace-pre-wrap text-xs text-primary leading-relaxed min-h-[2rem]">
            {text}
            {isStreaming && <span className="inline-block w-1.5 h-3 ml-0.5 bg-brand/70 animate-pulse align-middle" aria-hidden="true" />}
          </div>
          <div className="flex items-center justify-between gap-2">
            {isStreaming ? (
              <button
                type="button"
                onClick={stop}
                className="inline-flex items-center gap-1.5 text-xs text-secondary border border-border rounded-md px-2 py-1 hover:bg-surface/80"
                aria-label={t('ai.stream.stop')}
              >
                <Square className="w-3 h-3" />
                {t('ai.stream.stop')}
              </button>
            ) : (
              <button
                onClick={() => void startAdvice()}
                className="text-xs text-secondary hover:text-brand transition-colors"
              >
                Neu laden
              </button>
            )}
            {!isStreaming && <TokenCostIndicator durationMs={durationMs} />}
          </div>
          {sources.length > 0 && (
            <div className="flex flex-wrap gap-1.5 pt-1 border-t border-border/50">
              <span className="text-xs text-secondary self-center">Quellen:</span>
              {sources.map((src) => (
                <button
                  key={src.label}
                  onClick={() => { navigate(src.href) }}
                  className="inline-flex items-center gap-1 text-xs text-brand border border-brand/30 rounded-full px-2 py-0.5 hover:bg-brand/10 transition-colors"
                >
                  {src.label}
                  <ExternalLink className="w-2.5 h-2.5" />
                </button>
              ))}
            </div>
          )}
        </div>
      )}

      {/* Pro-Gate (FeatureLocked) */}
      {featureLockedError && <ProGate error={featureLockedError}>{null}</ProGate>}

      {/* Regular error */}
      {error && !featureLockedError && (
        <div className="flex items-start gap-2 text-xs text-red-400 bg-red-500/10 border border-red-500/20 rounded-lg px-3 py-2">
          <AlertTriangle className="w-3.5 h-3.5 mt-0.5 shrink-0" />
          <span>{errorMessage ?? error.message}</span>
        </div>
      )}

      {/* Streaming-Status nur als Tooltip-Helper */}
      {isStreaming && !text && (
        <div className="flex items-center gap-2 text-xs text-secondary">
          <Loader2 className="w-3 h-3 animate-spin shrink-0" />
          <span>{t('ai.stream.thinking')}</span>
        </div>
      )}
    </div>
  )
}
