import { useState } from 'react'
import { Link } from 'react-router-dom'
import { HardDrive } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import type { BackupStatus } from '../../hooks/useDashboard'

interface BackupBannerProps {
  status: BackupStatus | undefined
}

/**
 * Backup-Hinweis über allen Seiten.
 *
 * UX-10: Auf einer frischen Instanz (noch nie ein Backup, kein Ziel eingerichtet,
 * jünger als 7 Tage — `setup_pending` vom Backend) stand ab der ersten Minute
 * „Kein Backup in den letzten 7 Tagen". Das ist eine Warnung, die man nicht
 * vermeiden konnte. Dort steht jetzt ein freundlicher Hinweis „Backup einrichten".
 * Die Warnung bleibt, sobald wirklich ein Backup fällig ist (`stale`).
 */
export function BackupBanner({ status }: BackupBannerProps) {
  const { t } = useTranslation()
  const [dismissed, setDismissed] = useState(false)

  if (!status || dismissed) return null

  if (status.setup_pending) {
    return (
      <div
        data-testid="backup-setup-hint"
        className="bg-brand/5 border-b border-brand/20 px-4 py-2 flex items-center justify-between gap-3 text-sm shrink-0"
      >
        <span className="text-primary flex items-center gap-2 min-w-0">
          <HardDrive className="w-4 h-4 shrink-0 text-brand" aria-hidden="true" />
          <span>
            {t('backup.setupHint')}{' '}
            <Link to="/settings#system" className="underline font-medium text-brand hover:text-brand/80">
              {t('backup.setupAction')}
            </Link>
          </span>
        </span>
        <button
          onClick={() => { setDismissed(true) }}
          aria-label={t('common.close')}
          className="text-secondary hover:text-primary shrink-0"
        >
          ✕
        </button>
      </div>
    )
  }

  if (!status.stale) return null

  return (
    <div
      data-testid="backup-stale-warning"
      className="bg-amber-50 dark:bg-amber-950/30 border-b border-amber-200 dark:border-amber-800 px-4 py-2 flex items-center justify-between gap-3 text-sm shrink-0"
    >
      <span className="text-amber-800 dark:text-amber-300">
        ⚠ {t('backup.staleWarning')} —{' '}
        <Link to="/settings#system" className="underline font-medium hover:text-amber-900 dark:hover:text-amber-200">
          {t('backup.staleAction')}
        </Link>
      </span>
      <button
        onClick={() => { setDismissed(true) }}
        aria-label={t('common.close')}
        className="text-amber-600 dark:text-amber-400 hover:text-amber-800 dark:hover:text-amber-200 shrink-0"
      >
        ✕
      </button>
    </div>
  )
}
