import { describe, it, expect } from 'vitest'
import { readFileSync, readdirSync, statSync } from 'node:fs'
import { join, relative } from 'node:path'

/**
 * R1-KW-7 / R1-KW-9 — `<SelectItem value="">` lets the page crash.
 *
 * Radix Select throws as soon as the list opens if an item carries an empty
 * value ("A <Select.Item /> must have a value prop that is not an empty
 * string"). "Richtlinie anlegen" and "Risiko erfassen" both died that way on
 * a Kernweg; four more Comply dialogs (BSI, CAPA) had the same literal. The
 * fix is the `SELECT_NONE` placeholder from components/ui/select.tsx.
 *
 * TypeScript and lint accept the literal, and a render test only sees it
 * once someone opens that exact dropdown — so the check is on the source.
 * Verified non-vacuous: against the pre-fix tree it reported all seven sites.
 *
 * What it does NOT see: a dynamic value that happens to be empty at runtime
 * (`value={member.name}` — the original R1-KW-9 shape; UserPicker.test.tsx
 * covers that one), and values built through a variable or helper.
 */

// vitest runs with the frontend package root as cwd.
const SRC = join(process.cwd(), 'src')

function tsxFiles(dir: string, out: string[] = []): string[] {
  for (const name of readdirSync(dir)) {
    const full = join(dir, name)
    if (statSync(full).isDirectory()) {
      if (name !== 'node_modules') tsxFiles(full, out)
    } else if (name.endsWith('.tsx') && !name.includes('.test.')) {
      out.push(full)
    }
  }
  return out
}

describe('SelectItem ohne leeren Wert', () => {
  it('kein <SelectItem value=""> im Quelltext', () => {
    const hits: string[] = []
    const files = tsxFiles(SRC)
    for (const file of files) {
      readFileSync(file, 'utf8').split('\n').forEach((line, i) => {
        if (line.trimStart().startsWith('//')) return
        if (/<SelectItem\b[^>]*\bvalue=(""|''|\{\s*(""|'')\s*\})/.test(line)) {
          hits.push(`${relative(SRC, file)}:${i + 1}`)
        }
      })
    }
    expect(files.length).toBeGreaterThan(100)
    expect(hits).toEqual([])
  })
})
