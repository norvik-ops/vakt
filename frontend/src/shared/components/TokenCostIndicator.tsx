import { Gauge } from 'lucide-react'
import { useTranslation } from 'react-i18next'

/**
 * TokenCostIndicator — Sprint 15 S15-9.
 *
 * Mini-Indikator am Ende einer AI-Generation: "~1.2k Tokens · 0.02 € · 4.3 s".
 * Felder die nicht bekannt sind werden weggelassen statt mit Pseudo-Werten
 * gefüllt — bei lokalem LLM ist `costEur` immer 0 (zeigt "lokal").
 *
 * Token-Counts kommen erst nach Stream-Ende vom Backend (POST /ai/usage-
 * Endpoint folgt in einer späteren Welle — aktuell nutzt das Frontend nur
 * die durationMs aus dem useAIStream-Hook).
 */
interface Props {
  durationMs?: number
  tokensIn?: number
  tokensOut?: number
  costMicroEur?: number
}

function fmtTokens(n: number | undefined): string | null {
  if (n == null) return null
  if (n < 1000) return `${n} Tk`
  return `${(n / 1000).toFixed(1)}k Tk`
}

function fmtCost(microEur: number | undefined): string | null {
  if (microEur == null) return null
  if (microEur === 0) return null // 0 = lokales LLM, kein €-Wert anzeigen
  const eur = microEur / 1_000_000
  if (eur < 0.01) return '<0.01 €'
  return eur.toFixed(2) + ' €'
}

function fmtDur(ms: number | undefined): string | null {
  if (ms == null || ms === 0) return null
  if (ms < 1000) return `${ms} ms`
  return `${(ms / 1000).toFixed(1)} s`
}

export function TokenCostIndicator({ durationMs, tokensIn, tokensOut, costMicroEur }: Props) {
  const { t } = useTranslation()
  const parts: string[] = []
  const tokens = (tokensIn ?? 0) + (tokensOut ?? 0)
  if (tokens > 0) {
    const s = fmtTokens(tokens)
    if (s) parts.push(s)
  }
  const cost = fmtCost(costMicroEur)
  if (cost) parts.push(cost)
  else if (costMicroEur === 0) parts.push(t('ai.cost.local'))
  const dur = fmtDur(durationMs)
  if (dur) parts.push(dur)

  if (parts.length === 0) return null

  return (
    <span
      className="inline-flex items-center gap-1.5 text-[11px] text-secondary"
      title={t('ai.cost.tooltip')}
    >
      <Gauge className="w-3 h-3" aria-hidden="true" />
      {parts.join(' · ')}
    </span>
  )
}
