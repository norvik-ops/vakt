import { Eye } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { useCanWrite } from '../hooks/useCanWrite'

/**
 * One line at the top of a page for users without write rights (Viewer,
 * AuditorReadOnly): the write buttons are hidden, and this says why — instead of
 * the refusal arriving only after a click.
 */
export function ReadOnlyHint({ className = '' }: { className?: string }) {
  const { t } = useTranslation()
  const canWrite = useCanWrite()
  if (canWrite) return null
  return (
    <p
      role="note"
      className={`flex items-center gap-2 rounded-md border border-border bg-surface2 px-3 py-2 text-xs text-secondary ${className}`}
    >
      <Eye className="w-3.5 h-3.5 shrink-0" aria-hidden="true" />
      {t('common.readOnlyHint')}
    </p>
  )
}
