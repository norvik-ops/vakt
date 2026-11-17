/**
 * Sweep: jeder im Code fest geschriebene i18n-Schlüssel `t('a.b')` steht in de.json —
 * sonst zeigt die Oberfläche den rohen Schlüssel (UX-09: Knopf „settingsPage.save“
 * in der Karte „Backup-Ziel“).
 *
 * Ein Aufruf mit Text-Default (`t('a.b', 'Text')`) zählt nicht als Fehler: Er zeigt
 * den Default, nicht den Schlüssel.
 * Was der Test NICHT sieht: dynamische Schlüssel (`t(`x.${y}`)`, `t(variable)`),
 * Pluralformen mit anderem Suffix als _one/_other/_zero, und fehlende Übersetzungen
 * in en/fr/nl (das prüft i18n.lint.test.ts).
 */
/// <reference types="node" />
import { describe, it, expect } from 'vitest'
import { readFileSync, readdirSync, statSync } from 'node:fs'
import { join, relative } from 'node:path'

const SRC = join(process.cwd(), 'src')
const de = JSON.parse(readFileSync(join(SRC, 'i18n/locales/de.json'), 'utf-8')) as Record<string, unknown>

function has(key: string): boolean {
  let cur: unknown = de
  for (const part of key.split('.')) {
    if (typeof cur !== 'object' || cur === null || !(part in cur)) return false
    cur = (cur as Record<string, unknown>)[part]
  }
  return true
}

function files(dir: string): string[] {
  return readdirSync(dir).flatMap((name) => {
    const p = join(dir, name)
    if (statSync(p).isDirectory()) return files(p)
    return /\.tsx?$/.test(name) && !/\.test\./.test(name) && !name.startsWith('generated') ? [p] : []
  })
}

function missing(): { checked: number; out: string[] } {
  const re = /\bt\(\s*['"]([A-Za-z0-9_]+(?:\.[A-Za-z0-9_-]+)+)['"]\s*(,\s*['"`])?/g
  const out: string[] = []
  let checked = 0
  for (const f of files(SRC)) {
    const src = readFileSync(f, 'utf-8')
    for (const m of src.matchAll(re)) {
      checked++
      const key = m[1]
      if (m[2]) continue // Text-Default vorhanden
      if (has(key) || ['_one', '_other', '_zero'].some((s) => has(key + s))) continue
      out.push(`${key} (${relative(SRC, f)})`)
    }
  }
  return { checked, out }
}

describe('i18n: im Code benutzte Schlüssel', () => {
  it('stehen alle in de.json', () => {
    const { checked, out } = missing()
    expect(checked).toBeGreaterThan(1000)
    expect(out).toEqual([])
  })

  it('UX-09: Speichern-Knopf der Karte „Backup-Ziel“ ist in allen Sprachen übersetzt', () => {
    for (const lang of ['de', 'en', 'fr', 'nl']) {
      const cat = JSON.parse(readFileSync(join(SRC, `i18n/locales/${lang}.json`), 'utf-8')) as {
        settingsPage: Record<string, string>
      }
      for (const k of ['save', 'saving']) {
        expect(cat.settingsPage[k], `${lang}: settingsPage.${k}`).toBeTruthy()
        expect(cat.settingsPage[k]).not.toBe(`settingsPage.${k}`)
      }
    }
  })
})
