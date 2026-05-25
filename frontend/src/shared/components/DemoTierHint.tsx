import { Sparkles } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { useDemoMode } from '../hooks/useDemoMode'

interface DemoTierHintProps {
  tier?: 'pro' | 'enterprise'
}

/**
 * Shows a soft "Pro feature" badge in demo mode so visitors understand
 * which tier a module belongs to. Renders nothing outside demo mode.
 */
export function DemoTierHint({ tier = 'pro' }: DemoTierHintProps) {
  const demoMode = useDemoMode()
  const { t } = useTranslation()

  if (!demoMode) return null

  const label = tier === 'enterprise' ? 'Enterprise' : t('errors.pro.demoHint')

  return (
    <span className="inline-flex items-center gap-1 text-xs font-semibold text-brand/80 bg-brand/8 border border-brand/20 rounded-full px-2.5 py-0.5">
      <Sparkles className="w-3 h-3" />
      {label}
    </span>
  )
}
