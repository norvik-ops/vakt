import { Ban, Sparkles } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { Link } from 'react-router-dom'
import { FeatureLockedError } from '../../api/client'
import { ErrorState } from './ErrorState'
import { useFeature } from '../hooks/useFeature'
import { isUnsoldFeature } from '../featureTiers'
import { VAKT_PRO_CHECKOUT_URL } from '../../lib/constants'

interface ProGateProps {
  /** Query error — renders the lock if FeatureLockedError, ErrorState otherwise. */
  error: unknown
  children: React.ReactNode
  /** Optional feature name: pre-check license before the first API call to avoid flicker. */
  feature?: string
}

interface FeatureLockProps {
  /**
   * 'pro'    = eine Pro-Lizenz schaltet die Funktion frei → Weg zur Lizenz.
   * 'unsold' = das Merkmal liegt in keinem Tarif (features.UnsoldFeatures) → kein
   *            Kauf-Link, denn kein Kauf schaltet es frei.
   */
  kind: 'pro' | 'unsold'
  /** Zusätzlicher Ausweg neben den Lizenz-Links, z. B. eine kostenlose Alternative. */
  extraAction?: React.ReactNode
  className?: string
}

/**
 * Die eine Sperr-Darstellung für gesperrte Funktionen. Alle Stellen, die eine
 * Lizenz-Sperre zeigen, nutzen diese Komponente — vorher gab es vier Formen
 * derselben Sperre (Upgrade-Karte, Vollbild mit rohem Feature-Schlüssel, roter
 * Ladefehler, „Demnächst verfügbar“).
 */
export function FeatureLock({ kind, extraAction, className = 'mx-6 mt-4' }: FeatureLockProps) {
  const { t } = useTranslation()
  const unsold = kind === 'unsold'
  const Icon = unsold ? Ban : Sparkles
  return (
    <div
      data-testid={unsold ? 'feature-lock-unsold' : 'feature-lock-pro'}
      className={`${className} p-5 rounded-xl border border-brand/20 bg-brand/5 flex items-start gap-4`}
    >
      <div className="mt-0.5 p-2 rounded-lg bg-brand/10 shrink-0">
        <Icon className="w-4 h-4 text-brand" />
      </div>
      <div>
        <h2 className="font-semibold text-primary text-sm mb-1">
          {unsold ? t('vaktcomply.frameworksPage.frameworkStatusNotOffered') : t('errors.pro.title')}
        </h2>
        <p className="text-secondary text-sm leading-relaxed mb-2">
          {unsold ? t('errors.notOffered.description') : t('errors.pro.description')}
        </p>
        {!unsold && <p className="text-xs text-secondary/70 mb-3">{t('errors.pro.featureList')}</p>}
        {(!unsold || extraAction) && (
          <div className="flex flex-wrap items-center gap-x-4 gap-y-2">
            {!unsold && (
              <>
                <Link
                  to="/settings"
                  className="inline-flex items-center text-xs font-semibold text-brand hover:text-brand/80 transition-colors"
                >
                  {t('errors.pro.activateLicense')}
                </Link>
                <a
                  href={VAKT_PRO_CHECKOUT_URL}
                  target="_blank"
                  rel="noopener noreferrer"
                  className="inline-flex items-center gap-1.5 text-xs font-semibold text-brand hover:text-brand/80 transition-colors"
                >
                  <Sparkles className="w-3.5 h-3.5" />
                  {t('errors.pro.cta')}
                </a>
              </>
            )}
            {extraAction}
          </div>
        )}
      </div>
    </div>
  )
}

export function ProGate({ error, children, feature }: ProGateProps) {
  const { enabled, loading } = useFeature(feature ?? '')

  // Early gate: if we know from the license that this feature is locked,
  // show the lock immediately without waiting for an API call to fail.
  if (feature && !loading && !enabled) {
    return <FeatureLock kind={isUnsoldFeature(feature) ? 'unsold' : 'pro'} />
  }

  if (error instanceof FeatureLockedError) {
    return <FeatureLock kind={isUnsoldFeature(error.feature) ? 'unsold' : 'pro'} />
  }

  if (error != null) {
    const msg = error instanceof Error ? error.message : undefined
    return <ErrorState message={msg} />
  }

  return <>{children}</>
}
