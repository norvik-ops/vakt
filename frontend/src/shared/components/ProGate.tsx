import { Clock, Sparkles } from 'lucide-react'
import { FeatureLockedError } from '../../api/client'

interface ProGateProps {
  error: unknown
  children: React.ReactNode
}

/**
 * Wraps content that may fail with a 402 FeatureLockedError.
 * Shows a "coming soon" notice instead of a generic error when the feature requires Pro.
 */
export function ProGate({ error, children }: ProGateProps) {
  if (error instanceof FeatureLockedError) {
    return (
      <div className="mx-6 mt-4 p-5 rounded-xl border border-brand/20 bg-brand/5 flex items-start gap-4">
        <div className="mt-0.5 p-2 rounded-lg bg-brand/10">
          <Sparkles className="w-4 h-4 text-brand" />
        </div>
        <div>
          <p className="font-semibold text-primary text-sm mb-1">Pro-Feature</p>
          <p className="text-secondary text-sm leading-relaxed mb-2">
            Dieses Feature ist in der Community Edition nicht verfügbar.
            Vakt Pro mit TISAX, DORA, EU AI Act, NIS2-Meldungsassistent,
            Audit-PDF-Export, SSO, API-Access, granularen Modul-Berechtigungen
            und mehr ist in Planung.
          </p>
          <span className="inline-flex items-center gap-1.5 text-xs font-semibold text-brand">
            <Clock className="w-3.5 h-3.5" />
            Demnächst verfügbar
          </span>
        </div>
      </div>
    )
  }

  return <>{children}</>
}
