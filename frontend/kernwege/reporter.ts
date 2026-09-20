import fs from 'node:fs'
import path from 'node:path'
import type { FullResult, Reporter, TestCase, TestResult } from '@playwright/test/reporter'

/**
 * Zählt die Kernwege so, wie P7c es verlangt: grün, erwartet-rot und übersprungen
 * GETRENNT. Playwrights eigene Zusammenfassung zählt ein `test.fail()`, das wie
 * erwartet scheitert, als „passed" — für eine Ziellinie wäre das eine Lüge.
 *
 *   grün          — erwartet bestanden, bestanden
 *   erwartet-rot  — test.fail() mit Befund-ID, scheitert wie erwartet (Befund besteht)
 *   übersprungen  — test.skip()/test.fixme() mit Grund; NICHT grün
 *   ROT           — alles andere. Dazu gehört ein test.fail(), das plötzlich besteht:
 *                   dann ist der Befund behoben und das fail() muss raus (Sperre gegen
 *                   Verschlechterung wirkt in beide Richtungen).
 */
type Kind = 'gruen' | 'erwartet-rot' | 'uebersprungen' | 'ROT'

interface Row {
  kind: Kind
  title: string
  file: string
  note: string
  ms: number
}

const LABEL: Record<Kind, string> = {
  gruen: 'grün',
  'erwartet-rot': 'erwartet-rot',
  uebersprungen: 'übersprungen',
  ROT: 'ROT',
}

function classify(test: TestCase, result: TestResult): { kind: Kind; note: string } {
  const ann = test.annotations.map((a) => `${a.type}${a.description ? `: ${a.description}` : ''}`).join('; ')
  if (test.expectedStatus === 'skipped' || result.status === 'skipped') {
    return { kind: 'uebersprungen', note: ann }
  }
  if (test.expectedStatus === 'failed') {
    if (result.status === 'failed' || result.status === 'timedOut') {
      return { kind: 'erwartet-rot', note: ann }
    }
    return {
      kind: 'ROT',
      note: 'test.fail() besteht jetzt — Befund behoben? Dann test.fail() entfernen und Befund schließen.',
    }
  }
  if (result.status === 'passed') return { kind: 'gruen', note: '' }
  const msg = (result.error?.message ?? result.status).split('\n')[0] ?? ''
  return { kind: 'ROT', note: msg.replace(/\[[0-9;]*m/g, '').slice(0, 200) }
}

export default class KernwegeReporter implements Reporter {
  private rows: Row[] = []
  private t0 = Date.now()

  onTestEnd(test: TestCase, result: TestResult): void {
    const { kind, note } = classify(test, result)
    const title = test.titlePath().slice(3).join(' › ')
    const row: Row = { kind, title, file: path.basename(test.location.file), note, ms: result.duration }
    this.rows.push(row)
    const mark = { gruen: '✓', 'erwartet-rot': '◐', uebersprungen: '–', ROT: '✗' }[kind]
    console.log(`  ${mark} ${LABEL[kind].padEnd(12)} ${title}${note ? `  [${note}]` : ''}`)
  }

  onEnd(result: FullResult): void {
    const count = (k: Kind) => this.rows.filter((r) => r.kind === k).length
    const secs = Math.round((Date.now() - this.t0) / 1000)
    const line =
      `Kernwege: ${count('gruen')} grün · ${count('erwartet-rot')} erwartet-rot (test.fail) · ` +
      `${count('uebersprungen')} übersprungen · ${count('ROT')} ROT — ${secs} s`
    console.log(`\n${line}`)
    const rot = this.rows.filter((r) => r.kind === 'ROT')
    if (rot.length > 0) {
      console.log('\nROT:')
      for (const r of rot) console.log(`  ✗ ${r.title} (${r.file})\n      ${r.note}`)
    }
    if (result.status !== 'passed' && rot.length === 0) {
      console.log(`\nLauf-Status: ${result.status} (Abbruch/Timeout außerhalb eines Tests?)`)
    }

    const outDir = path.join(process.cwd(), 'kernwege-results')
    fs.mkdirSync(outDir, { recursive: true })
    fs.writeFileSync(
      path.join(outDir, 'summary.json'),
      JSON.stringify({ status: result.status, seconds: secs, rows: this.rows }, null, 2),
    )
    const md = [
      `### ${line}`,
      '',
      '| Ergebnis | Test | Hinweis |',
      '|---|---|---|',
      ...this.rows.map((r) => `| ${LABEL[r.kind]} | ${r.title.replace(/\|/g, '\\|')} | ${r.note.replace(/\|/g, '\\|')} |`),
      '',
    ].join('\n')
    fs.writeFileSync(path.join(outDir, 'summary.md'), md)
    const stepSummary = process.env.GITHUB_STEP_SUMMARY
    if (stepSummary) fs.appendFileSync(stepSummary, md)
  }

  printsToStdio(): boolean {
    return true
  }
}
