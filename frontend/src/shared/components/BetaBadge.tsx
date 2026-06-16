import { useTranslation } from 'react-i18next'

// Public disclaimer (synced to the public mirror via docs/wiki/**).
const DISCLAIMER_URL = 'https://github.com/norvik-ops/vatk/blob/main/docs/wiki/beta-disclaimer.md'

// BetaBadge is a discreet "Private Beta" pill shown in the sidebar header. It
// links to the beta disclaimer (status, best-effort support, backup
// responsibility) so the beta status and support expectations are always one
// click away (S89-3).
export function BetaBadge({ collapsed = false }: { collapsed?: boolean }) {
  const { t } = useTranslation()
  if (collapsed) {
    return (
      <a
        href={DISCLAIMER_URL}
        target="_blank"
        rel="noopener noreferrer"
        title={t('beta.tooltip')}
        aria-label={t('beta.badge')}
        className="mx-auto block w-2 h-2 rounded-full bg-amber-500"
        data-testid="beta-badge"
      />
    )
  }
  return (
    <a
      href={DISCLAIMER_URL}
      target="_blank"
      rel="noopener noreferrer"
      title={t('beta.tooltip')}
      data-testid="beta-badge"
      className="inline-flex items-center gap-1 rounded px-1.5 py-0.5 text-[10px] font-semibold uppercase tracking-wide text-amber-600 bg-amber-50 border border-amber-200 hover:bg-amber-100 dark:bg-amber-950/40 dark:border-amber-700 dark:text-amber-400"
    >
      {t('beta.badge')}
    </a>
  )
}
