import { AlertTriangle } from 'lucide-react'
import { useTranslation } from 'react-i18next'

/**
 * Prominent AI disclaimer banner — used wherever AI-generated content is displayed.
 * Matches the policy-draft disclaimer style (bg-amber-500/10, border, AlertTriangle icon).
 *
 * Use `variant="draft"` for policy drafts (stronger wording), `variant="content"` for
 * generated reports/insights/narratives.
 */
export function AIDisclaimer({ variant = 'content' }: { variant?: 'content' | 'draft' }) {
  const { t } = useTranslation()
  const key = variant === 'draft' ? 'ai.draftDisclaimer' : 'ai.disclaimer'

  return (
    <div className="flex items-center gap-2 px-3 py-2 rounded-lg bg-amber-500/10 border border-amber-500/20 text-amber-700 dark:text-amber-400 text-xs font-medium">
      <AlertTriangle className="w-3.5 h-3.5 shrink-0" />
      {t(key)}
    </div>
  )
}
