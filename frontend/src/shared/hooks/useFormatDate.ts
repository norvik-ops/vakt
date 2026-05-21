import { useTranslation } from 'react-i18next'
import { useCallback, useMemo } from 'react'

/**
 * useFormatDate — gibt Formatter zurueck, die die aktuelle i18n-Locale beruecksichtigen.
 *
 * Hintergrund (S13-27): Vor Sprint 13 nutzten 60+ Stellen `toLocaleDateString('de-DE', …)`
 * und `toLocaleString('de-DE', …)` hardcoded — eine viersprachige App, die auf
 * Englisch, Französisch oder Niederländisch falsche Datumsformate zeigt.
 *
 * Nutzung:
 *   const { formatDate, formatDateTime, formatTime } = useFormatDate()
 *   <span>{formatDate(iso)}</span>
 *   <span>{formatDateTime(iso, { dateStyle: 'medium', timeStyle: 'short' })}</span>
 *
 * `value` darf String, number oder Date sein. Ungueltige Werte werfen kein Throw —
 * sie liefern den Original-String zurueck (defensiv, damit eine kaputte API-Antwort
 * nicht die ganze Seite crasht).
 */
type DateLike = string | number | Date | null | undefined

const localeMap: Record<string, string> = {
  de: 'de-DE',
  en: 'en-US',
  fr: 'fr-FR',
  nl: 'nl-NL',
}

function toBCP47(i18nLanguage: string): string {
  const base = (i18nLanguage || 'de').toLowerCase().split('-')[0]
  return localeMap[base] ?? 'de-DE'
}

function safeDate(value: DateLike): Date | null {
  if (value == null || value === '') return null
  const d = value instanceof Date ? value : new Date(value)
  return isNaN(d.getTime()) ? null : d
}

export function useFormatDate() {
  const { i18n } = useTranslation()
  const bcp47 = useMemo(() => toBCP47(i18n.language), [i18n.language])

  const formatDate = useCallback(
    (value: DateLike, options: Intl.DateTimeFormatOptions = { dateStyle: 'medium' }) => {
      const d = safeDate(value)
      if (!d) return typeof value === 'string' ? value : ''
      return d.toLocaleDateString(bcp47, options)
    },
    [bcp47],
  )

  const formatDateTime = useCallback(
    (value: DateLike, options: Intl.DateTimeFormatOptions = { dateStyle: 'medium', timeStyle: 'short' }) => {
      const d = safeDate(value)
      if (!d) return typeof value === 'string' ? value : ''
      return d.toLocaleString(bcp47, options)
    },
    [bcp47],
  )

  const formatTime = useCallback(
    (value: DateLike, options: Intl.DateTimeFormatOptions = { timeStyle: 'short' }) => {
      const d = safeDate(value)
      if (!d) return typeof value === 'string' ? value : ''
      return d.toLocaleTimeString(bcp47, options)
    },
    [bcp47],
  )

  /**
   * formatRelative gibt einen relativen String zurueck ("vor 3 Tagen", "in 2 Stunden").
   * Faellt auf das absolute Datum zurueck, wenn die Differenz > 30 Tage ist.
   */
  const formatRelative = useCallback(
    (value: DateLike) => {
      const d = safeDate(value)
      if (!d) return ''
      const diffSec = Math.round((d.getTime() - Date.now()) / 1000)
      const absSec = Math.abs(diffSec)
      const rtf = new Intl.RelativeTimeFormat(bcp47, { numeric: 'auto' })
      if (absSec < 60) return rtf.format(diffSec, 'second')
      if (absSec < 3600) return rtf.format(Math.round(diffSec / 60), 'minute')
      if (absSec < 86400) return rtf.format(Math.round(diffSec / 3600), 'hour')
      if (absSec < 30 * 86400) return rtf.format(Math.round(diffSec / 86400), 'day')
      return d.toLocaleDateString(bcp47, { dateStyle: 'medium' })
    },
    [bcp47],
  )

  return { formatDate, formatDateTime, formatTime, formatRelative, bcp47 }
}
