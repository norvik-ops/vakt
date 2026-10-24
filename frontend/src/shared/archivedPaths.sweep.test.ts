import { describe, it, expect } from 'vitest'
import { readFileSync, readdirSync, statSync } from 'node:fs'
import { join, relative } from 'node:path'

/**
 * Scan, Vault, Aware, Privacy und HR sind archiviert (docs/stories/module-archivieren.md).
 * Ihre Seiten und API-Pfade gibt es nicht mehr — ein Link oder Aufruf dorthin endet auf
 * "Seite nicht gefunden" bzw. 404.
 *
 * Erlaubt bleibt nur der Weiterleiter alter Asset-Lesezeichen (/vaktscan/assets → Comply).
 * Verified non-vacuous: against the pre-fix tree it reported the ISO-27001 and NIS2
 * checklist sites.
 *
 * What it does NOT see: paths built at runtime (`/${module}` — so the BSI checklist's 26
 * dead links slipped past it; ISO27001ChecklistPage.modules.test.tsx renders that page),
 * and generated.ts (the OpenAPI types still list the /vaktscan/assets alias, which the
 * backend serves).
 */

const SRC = join(process.cwd(), 'src')
const ARCHIVED = /['"`]\/(api\/v1\/)?(vaktscan|vaktvault|vaktprivacy|vaktaware|vakthr)(\/|['"`])/
const ALLOWED = [/path: '\/vaktscan\/assets(\/:id)?', element: <LegacyAssetRedirect \/>/]

function sourceFiles(dir: string, out: string[] = []): string[] {
  for (const name of readdirSync(dir)) {
    const full = join(dir, name)
    if (statSync(full).isDirectory()) {
      if (name !== 'node_modules') sourceFiles(full, out)
    } else if (/\.tsx?$/.test(name) && !name.includes('.test.') && name !== 'generated.ts') {
      out.push(full)
    }
  }
  return out
}

describe('keine Pfade auf archivierte Module', () => {
  it('kein Link oder API-Aufruf nach /vaktscan, /vaktvault, /vaktprivacy, /vaktaware, /vakthr', () => {
    const hits: string[] = []
    const files = sourceFiles(SRC)
    for (const file of files) {
      readFileSync(file, 'utf8').split('\n').forEach((line, i) => {
        if (ARCHIVED.test(line) && !ALLOWED.some((a) => a.test(line))) {
          hits.push(`${relative(SRC, file)}:${i + 1}`)
        }
      })
    }
    expect(files.length).toBeGreaterThan(100)
    expect(hits).toEqual([])
  })
})
