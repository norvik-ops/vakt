import { describe, it, expect } from 'vitest'
import { readFileSync, readdirSync, statSync } from 'node:fs'
import { join } from 'node:path'

/**
 * R23-02 — a boolean UI flag that nothing ever sets to true.
 *
 * AssetsPage held `const [importOpen, setImportOpen] = useState(false)` and a
 * whole import dialog behind it. `setImportOpen` appeared twice, both times
 * with `false`. No call site anywhere set it to true, so the dialog could not
 * be opened, the endpoint behind it had no reachable caller, and the hook that
 * posted to it had exactly one dead caller.
 *
 * Nothing catches this shape today. TypeScript is happy — the state is read,
 * the setter is called, every branch type-checks. Lint sees no unused symbol.
 * A rendering test sees nothing either: an unopenable dialog renders exactly
 * like a closed one, so a behavioural assertion about it passes identically
 * before and after the dead code is removed. (Checked, not assumed: the first
 * version of this file asserted "no file input is rendered", and that
 * assertion was green against the BROKEN page too. It was replaced with this
 * sweep rather than kept as decoration.)
 *
 * So the check has to be on the source. It is a sweep, not a spot check: the
 * point of the finding was a class, and this file is the only place that
 * states it.
 *
 * Detector reach, deliberately narrow to stay free of false positives:
 * a `useState(false)` whose setter is never called with anything other than
 * `false` AND is never passed by reference (e.g. `onOpenChange={setOpen}`,
 * which hands the true-setting to a child). Verified non-vacuous against the
 * pre-fix AssetsPage: it reported exactly `importOpen`, and nothing else in
 * the tree.
 *
 * What it does NOT see: a flag opened through a ref, a reducer, a store, or a
 * setter reached across module boundaries; and a dialog that IS openable but
 * whose trigger is unreachable for some other reason.
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

interface DeadFlag { file: string; state: string; calls: string[] }

function findDeadFlags(): DeadFlag[] {
  const dead: DeadFlag[] = []
  for (const file of tsxFiles(SRC)) {
    const text = readFileSync(file, 'utf8')
    const decl = /const \[(\w+), (set\w+)] = useState(?:<[^>]*>)?\(\s*false\s*\)/g
    let m: RegExpExecArray | null
    while ((m = decl.exec(text)) !== null) {
      const [, state, setter] = m
      const withoutDecl = text.replace(`const [${state}, ${setter}]`, '')
      const calls = [...withoutDecl.matchAll(new RegExp(`${setter}\\(\\s*([^)]*?)\\s*\\)`, 'g'))]
        .map((c) => c[1])
      // Passed by reference somewhere — a child component may set it to true.
      const byReference = new RegExp(`${setter}(?!\\s*\\()`).test(withoutDecl)
      const setsTruthy = calls.some((arg) => arg.trim() !== 'false' && arg.trim() !== '')
      if (!setsTruthy && !byReference) {
        dead.push({ file: file.slice(SRC.length), state, calls })
      }
    }
  }
  return dead
}

describe('dead UI state sweep (R23-02)', () => {
  it('has no boolean flag that nothing can ever set to true', () => {
    const dead = findDeadFlags()
    expect(
      dead.map((d) => `${d.file}: ${d.state} — only ever set to ${d.calls.join(', ') || 'nothing'}`),
    ).toEqual([])
  })

  it('the detector finds the shape it is looking for', () => {
    // Guards against the sweep above passing because the regex stopped
    // matching rather than because the tree is clean — the failure mode that
    // makes a green gate worthless.
    const sample = `
      const [importOpen, setImportOpen] = useState(false)
      function close() { setImportOpen(false) }
      <Dialog open={importOpen} onOpenChange={(v) => { if (!v) setImportOpen(false) }} />
    `
    const decl = /const \[(\w+), (set\w+)] = useState(?:<[^>]*>)?\(\s*false\s*\)/.exec(sample)
    expect(decl).not.toBeNull()
    const setter = decl![2]
    const rest = sample.replace(`const [${decl![1]}, ${setter}]`, '')
    const calls = [...rest.matchAll(new RegExp(`${setter}\\(\\s*([^)]*?)\\s*\\)`, 'g'))].map((c) => c[1])
    expect(calls.length).toBeGreaterThan(0)
    expect(calls.every((a) => a.trim() === 'false')).toBe(true)
    expect(new RegExp(`${setter}(?!\\s*\\()`).test(rest)).toBe(false)
  })
})
