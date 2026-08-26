import { Sparkles } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { useDemoMode } from '../hooks/useDemoMode'

/**
 * Shows a soft "Pro feature" badge in demo mode so visitors understand
 * which tier a module belongs to. Renders nothing outside demo mode.
 *
 * Takes no tier argument: there is exactly one paid tier. The second variant
 * this component used to render was removed on 2026-08-08 — no signing path
 * ever issued that tier, so the badge advertised something nobody could buy
 * (features.UnsoldFeatures). Whether the badge shows at all is still a
 * decision of the calling page (PageHeader's `tier` prop).
 */
export function DemoTierHint() {
  const demoMode = useDemoMode()
  const { t } = useTranslation()

  if (!demoMode) return null

  const label = t('errors.pro.demoHint')

  return (
    <span className="inline-flex items-center gap-1 text-xs font-semibold text-brand/80 bg-brand/8 border border-brand/20 rounded-full px-2.5 py-0.5">
      <Sparkles className="w-3 h-3" />
      {label}
    </span>
  )
}
