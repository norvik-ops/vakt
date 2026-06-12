import { Sparkles } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { FeatureLockedError } from '../../api/client'
import { ErrorState } from './ErrorState'
import { useFeature } from '../hooks/useFeature'
import { VAKT_PRO_CHECKOUT_URL } from '../../lib/constants'

interface ProGateProps {
  /** Query error — renders Pro upgrade UI if FeatureLockedError, ErrorState otherwise. */
  error: unknown
  children: React.ReactNode
  /** Optional feature name: pre-check license before the first API call to avoid flicker. */
  feature?: string
}

function ProUpgradeUI() {
  const { t } = useTranslation()
  return (
    <div className="mx-6 mt-4 p-5 rounded-xl border border-brand/20 bg-brand/5 flex items-start gap-4">
      <div className="mt-0.5 p-2 rounded-lg bg-brand/10">
        <Sparkles className="w-4 h-4 text-brand" />
      </div>
      <div>
        <p className="font-semibold text-primary text-sm mb-1">{t('errors.pro.title')}</p>
        <p className="text-secondary text-sm leading-relaxed mb-2">
          {t('errors.pro.description')}
        </p>
        <p className="text-xs text-secondary/70 mb-3">{t('errors.pro.featureList')}</p>
        <a
          href={VAKT_PRO_CHECKOUT_URL}
          target="_blank"
          rel="noopener noreferrer"
          className="inline-flex items-center gap-1.5 text-xs font-semibold text-brand hover:text-brand/80 transition-colors"
        >
          <Sparkles className="w-3.5 h-3.5" />
          {t('errors.pro.cta')}
        </a>
      </div>
    </div>
  )
}

export function ProGate({ error, children, feature }: ProGateProps) {
  const { enabled, loading } = useFeature(feature ?? '')

  // Early gate: if we know from the license that this feature is locked,
  // show upgrade UI immediately without waiting for an API call to fail.
  if (feature && !loading && !enabled) {
    return <ProUpgradeUI />
  }

  if (error instanceof FeatureLockedError) {
    return <ProUpgradeUI />
  }

  if (error != null) {
    const msg = error instanceof Error ? error.message : undefined
    return <ErrorState message={msg} />
  }

  return <>{children}</>
}
