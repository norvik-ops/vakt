import { useTranslation } from 'react-i18next'
import { roundPercent } from '../../../shared/utils/percent'
import { cn } from '../../../lib/utils'

export interface ReadinessCounts {
  /** Effectively implemented/covered controls. */
  covered: number
  /** Effectively in progress/partial — weighs half in `scorePct`. */
  partial: number
  /** All controls except not applicable — the denominator. */
  applicable: number
  /** Canonical readiness 0–100 (ADR-0086). */
  scorePct: number
}

/**
 * UX-15: "0 von 88 Controls umgesetzt" stood next to "5 %" — controls in
 * progress count half in the percentage but were invisible in the count. One
 * line, the same on dashboard, framework card, framework detail and Comply
 * overview: "0 / 88 umgesetzt · 9 in Bearbeitung · 5%". The tooltip states the
 * counting rule.
 */
export function ReadinessBreakdown({ counts, className }: { counts: ReadinessCounts; className?: string }) {
  const { t } = useTranslation()
  return (
    <span className={cn('tabular-nums', className)} title={t('vaktcomply.readiness.hint')} data-testid="readiness-breakdown">
      {t('vaktcomply.readiness.breakdown', {
        covered: counts.covered,
        partial: counts.partial,
        applicable: counts.applicable,
        percent: roundPercent(counts.scorePct),
      })}
    </span>
  )
}
