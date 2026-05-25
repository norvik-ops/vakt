import { Sparkles } from 'lucide-react'
import { useAIUsage } from '../hooks/useAIUsage'
import { VAKT_PRO_CHECKOUT_URL } from '../../lib/constants'

/**
 * Shows CE monthly AI request usage (e.g. "18 / 25 KI-Anfragen diesen Monat").
 * Hidden for Pro/Enterprise orgs and when usage data is unavailable.
 */
export function CEAICounter() {
  const { data } = useAIUsage()

  if (!data || data.is_pro) return null

  const { used, limit } = data
  const remaining = Math.max(0, limit - used)
  const pct = Math.min(100, (used / limit) * 100)
  const isExhausted = used >= limit
  const isWarning = remaining <= 5 && !isExhausted

  return (
    <div className={`flex items-center gap-2 text-xs rounded-lg px-3 py-1.5 border ${
      isExhausted
        ? 'bg-red-50 dark:bg-red-950/30 border-red-200 dark:border-red-800 text-red-700 dark:text-red-400'
        : isWarning
        ? 'bg-amber-50 dark:bg-amber-950/30 border-amber-200 dark:border-amber-800 text-amber-700 dark:text-amber-400'
        : 'bg-muted/50 border-border text-muted-foreground'
    }`}>
      <Sparkles className="w-3.5 h-3.5 shrink-0" />
      <span>
        {isExhausted ? (
          <>
            KI-Limit erreicht.{' '}
            <a
              href={VAKT_PRO_CHECKOUT_URL}
              target="_blank"
              rel="noopener noreferrer"
              className="underline font-medium hover:opacity-80"
            >
              Upgrade auf Pro →
            </a>
          </>
        ) : (
          <>
            {used}&thinsp;/&thinsp;{limit} KI-Anfragen
            <span className="hidden sm:inline"> diesen Monat</span>
          </>
        )}
      </span>
      {!isExhausted && (
        <div className="w-16 h-1 rounded-full bg-muted overflow-hidden shrink-0">
          <div
            className={`h-full rounded-full transition-all ${
              isWarning ? 'bg-amber-500' : 'bg-brand'
            }`}
            style={{ width: `${pct}%` }}
          />
        </div>
      )}
    </div>
  )
}
