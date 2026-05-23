import { ExternalLink, Sparkles } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { FeatureLockedError } from '../../api/client'
import { VAKT_PRO_CHECKOUT_URL } from '../../lib/constants'

interface ProGateProps {
  error: unknown
  children: React.ReactNode
}

/**
 * Wraps content that may fail with a 402 FeatureLockedError.
 * Shows a "coming soon" notice instead of a generic error when the feature requires Pro.
 */
export function ProGate({ error, children }: ProGateProps) {
  const { t } = useTranslation()

  if (error instanceof FeatureLockedError) {
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
          <a
            href={VAKT_PRO_CHECKOUT_URL}
            target="_blank"
            rel="noopener noreferrer"
            className="inline-flex items-center gap-1.5 text-xs font-semibold text-brand hover:underline"
          >
            <ExternalLink className="w-3.5 h-3.5" />
            {t('errors.pro.upgrade')}
          </a>
        </div>
      </div>
    )
  }

  return <>{children}</>
}
