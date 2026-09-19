import { useTranslation } from 'react-i18next'

// ModuleBetaBadge marks a single module as beta in the sidebar. The product as a
// whole is no longer labelled beta (2026-09-18); beta modules sit outside the launch gate
// (docs/launch-gate.md): they work, but their defects do not block the launch.
export function ModuleBetaBadge() {
  const { t } = useTranslation()
  return (
    <span
      title={t('beta.moduleTooltip')}
      data-testid="module-beta-badge"
      className="ml-auto rounded px-1 py-px text-[9px] font-semibold uppercase tracking-wide text-amber-600 bg-amber-50 border border-amber-200 dark:bg-amber-950/40 dark:border-amber-700 dark:text-amber-400"
    >
      {t('beta.module')}
    </span>
  )
}

// ModuleBetaNotice is the in-page counterpart: one line above the module's pages,
// so the beta status is visible where the work happens, not only in the nav.
export function ModuleBetaNotice({ messageKey }: { messageKey: string }) {
  const { t } = useTranslation()
  return (
    <div
      role="note"
      data-testid="module-beta-notice"
      className="mb-4 flex items-center gap-2 rounded-md border border-amber-200 bg-amber-50 px-3 py-2 text-xs text-amber-800 dark:border-amber-700 dark:bg-amber-950/40 dark:text-amber-300"
    >
      <ModuleBetaBadge />
      <span>{t(messageKey)}</span>
    </div>
  )
}
