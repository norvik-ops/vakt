/**
 * Dunkelmodus-Sweep: kein fest verdrahtetes `bg-white` ohne `dark:bg-*` im selben
 * Klassen-String — außer an begründeten Stellen (Liste unten).
 *
 * Muster aus der SoA-Seite (#210): Ein weißer Container erbt `text-primary`, im
 * Dunkelmodus fast weiß — Schrift weiß auf weiß. Danach fanden sich dieselbe Klasse
 * in NIS2-Neubewertung, Auditprogramm, interessierten Parteien und im Dialog der
 * Prüfungsansicht, dazu der Knopf „Seite neu laden“ der Fehlerseite.
 *
 * jsdom wendet kein CSS an, deshalb prüft der Test den Quelltext.
 * Was er NICHT prüft: `bg-gray-50`/`bg-gray-100` ohne dark-Variante, Inline-Styles,
 * zusammengesetzte Klassen aus Variablen, und den echten Kontrast im Browser.
 */
import { describe, it, expect } from 'vitest'
import { readFileSync, readdirSync, statSync } from 'node:fs'
import { join, relative } from 'node:path'

// vitest läuft mit dem frontend-Verzeichnis als cwd.
const SRC = join(process.cwd(), 'src')

/** Bewusst helle Stellen. Jede mit Grund — eine neue Ausnahme braucht einen. */
const ALLOWED: Record<string, string> = {
  // Schalter-Knöpfe: weißer Punkt auf farbiger Schiene, in beiden Modi gewollt.
  'components/ui/switch.tsx': 'Schalter-Knopf',
  'modules/vaktcomply/pages/TISAXMappingPage.tsx': 'Schalter-Knopf',
  'modules/settings/pages/AlertingSettingsPage.tsx': 'Schalter-Knopf',
  // Eigenständige Seiten ohne App-Rahmen mit durchgehend fester heller Palette
  // (text-gray-900 auf Weiß, keine Theme-Tokens) — hell, aber lesbar.
  'pages/MultiFrameworkWizardPage.tsx': 'feste helle Palette',
  'pages/NIS2WizardPage.tsx': 'feste helle Palette',
  'pages/TrustPage.tsx': 'öffentliche Seite, feste helle Palette',
  'pages/SupplierPortalPage.tsx': 'öffentliche Seite, feste helle Palette',
  // Weiße Markierungspunkte (akzeptierte Risiken) auf farbigen Heatmap-Zellen.
  'modules/vaktcomply/components/RiskHeatmap.tsx': 'Markierungspunkt',
}

function files(dir: string): string[] {
  return readdirSync(dir).flatMap((name) => {
    const p = join(dir, name)
    if (statSync(p).isDirectory()) return files(p)
    return /\.tsx?$/.test(name) && !/\.test\./.test(name) ? [p] : []
  })
}

function offenders(): string[] {
  const out: string[] = []
  for (const f of files(SRC)) {
    const rel = relative(SRC, f)
    if (rel in ALLOWED) continue
    readFileSync(f, 'utf-8').split('\n').forEach((line, i) => {
      // `bg-white/10` (Transparenz auf dunklem Grund) ist kein heller Hintergrund.
      if (/(^|[\s'"`])bg-white(?![\w/-])/.test(line) && !/dark:bg-/.test(line)) out.push(`${rel}:${String(i + 1)}`)
    })
  }
  return out
}

describe('Dunkelmodus: bg-white nur mit dark:-Gegenstück', () => {
  it('findet keine ungeschützte bg-white-Stelle außerhalb der Ausnahmen', () => {
    expect(files(SRC).length).toBeGreaterThan(100)
    expect(offenders()).toEqual([])
  })

  it('Fehlerseite: „Seite neu laden“ nutzt kein Text-Token als Hintergrund', () => {
    const src = readFileSync(join(SRC, 'shared/components/ErrorBoundary.tsx'), 'utf-8')
    expect(src).not.toMatch(/bg-primary\b/)
    expect(src).not.toMatch(/text-primary-foreground/)
  })
})
