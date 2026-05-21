import { useTranslation } from 'react-i18next'
import { useCallback } from 'react'
import { humanizeError } from '../utils/errorMessages'

/**
 * useErrorMessage — i18n-bewusster Wrapper um {@link humanizeError}.
 *
 * Backend-Errors haben heute zwei Formen:
 *
 *  1. Strukturiertes `{ error, code, details? }` (siehe CLAUDE.md API-Conventions).
 *  2. Klartext-Strings, die direkt in `err.message` landen.
 *
 * Dieser Hook bevorzugt — wenn ein `code` gegeben ist — den i18n-Schluessel
 * `errors.<CODE>` aus den Locale-Files. Faellt der Lookup fehl (kein
 * Schluessel registriert) wird auf `humanizeError(error)` zurueckgegriffen,
 * das die existierende deutsche Substring-Map nutzt.
 *
 * Damit verschwinden mit der Zeit hardcoded deutsche Phrasen aus
 * `errorMessages.ts`, ohne dass Aufrufer-Seiten gleichzeitig migriert werden
 * muessen. Phase 1 (Sprint 13): Hook existiert, neue Seiten nutzen ihn.
 * Phase 2 (Sprint 16): bulk-Migration der bestehenden Aufrufe.
 */
type WithCode = { code?: string; error?: string; message?: string }

function extractCode(err: unknown): string | undefined {
  if (err && typeof err === 'object') {
    const obj = err as WithCode
    if (obj.code && typeof obj.code === 'string' && /^[A-Z][A-Z0-9_]+$/.test(obj.code)) {
      return obj.code
    }
  }
  return undefined
}

export function useErrorMessage() {
  const { t, i18n } = useTranslation()

  return useCallback(
    (err: unknown, fallback?: string): string => {
      const code = extractCode(err)
      if (code) {
        const key = `errors.${code}`
        // i18next gibt den Key zurueck wenn kein Match — wir pruefen darauf.
        const translated = t(key)
        if (translated !== key) return translated
      }
      const hum = humanizeError(err)
      if (hum) return hum
      return fallback ?? t('errors.GENERIC', { defaultValue: 'Ein Fehler ist aufgetreten.' })
    },
    [t, i18n.language],
  )
}
