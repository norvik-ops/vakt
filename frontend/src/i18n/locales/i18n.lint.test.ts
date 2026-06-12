/**
 * i18n-Lint: validates that all locale files are valid JSON and share
 * the same flat key set as the reference locale (de.json).
 *
 * Red when: invalid JSON, missing key in any locale, extra key not in de.
 * Added in S81-4 — catches the nl/fr key drift that appeared in S77-3.
 */
import { describe, it, expect } from 'vitest'
import { readFileSync, readdirSync } from 'fs'
import { join } from 'path'

const LOCALES_DIR = join(import.meta.dirname ?? __dirname)
const REFERENCE = 'de'

function loadJSON(locale: string): Record<string, unknown> {
  const path = join(LOCALES_DIR, `${locale}.json`)
  const raw = readFileSync(path, 'utf-8')
  return JSON.parse(raw) as Record<string, unknown>
}

function flatKeys(obj: unknown, prefix = ''): string[] {
  if (typeof obj !== 'object' || obj === null || Array.isArray(obj)) return [prefix]
  const keys: string[] = []
  for (const [k, v] of Object.entries(obj as Record<string, unknown>)) {
    const full = prefix ? `${prefix}.${k}` : k
    if (typeof v === 'object' && v !== null && !Array.isArray(v)) {
      keys.push(...flatKeys(v, full))
    } else {
      keys.push(full)
    }
  }
  return keys
}

function localeFiles(): string[] {
  return readdirSync(LOCALES_DIR)
    .filter(f => f.endsWith('.json'))
    .map(f => f.replace('.json', ''))
}

describe('i18n locale files', () => {
  const locales = localeFiles()

  it('all locale files parse as valid JSON', () => {
    for (const locale of locales) {
      expect(() => loadJSON(locale), `${locale}.json should be valid JSON`).not.toThrow()
    }
  })

  it('all locales have the same keys as the reference locale (de)', () => {
    const reference = loadJSON(REFERENCE)
    const refKeys = new Set(flatKeys(reference))

    for (const locale of locales) {
      if (locale === REFERENCE) continue
      const data = loadJSON(locale)
      const localeKeys = new Set(flatKeys(data))

      const missing = [...refKeys].filter(k => !localeKeys.has(k))
      const extra = [...localeKeys].filter(k => !refKeys.has(k))

      expect(missing, `${locale}.json is missing keys from de.json`).toHaveLength(0)
      expect(extra, `${locale}.json has keys not in de.json (stale)`).toHaveLength(0)
    }
  })
})
