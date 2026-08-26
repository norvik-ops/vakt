import { describe, it, expect } from 'vitest'
import { readFileSync } from 'node:fs'
import { join } from 'node:path'

/**
 * R1-18-D9 — the background kept scrolling behind every modal.
 *
 * react-remove-scroll (the scroll lock inside every Radix overlay) works in two
 * halves: it sets `data-scroll-locked` on <body>, and it injects a <style>
 * element with the rules keyed on that attribute. Our CSP sets
 * `style-src-elem 'self'`, so the injected sheet is blocked on every page. The
 * attribute landed, nothing acted on it.
 *
 * WHAT THIS FILE PROVES, precisely: the replacement rules are in the bundled
 * stylesheet and are keyed on the attribute the library actually sets — read
 * out of the installed package rather than copied from its docs, so a version
 * that renames the attribute breaks this test instead of silently breaking the
 * scroll lock again.
 *
 * WHAT IT DOES NOT PROVE: that the background stops scrolling. jsdom applies no
 * stylesheets and no browser was reachable in this environment, so the visual
 * outcome is NOT MEASURED here. A CSS rule in the right file with the right
 * selector is necessary for the fix and is not sufficient evidence of it.
 */

const CSS = readFileSync(join(process.cwd(), 'src/index.css'), 'utf8')

describe('scroll lock CSS (R1-18-D9)', () => {
  it('carries the rules the blocked stylesheet would have carried', () => {
    expect(CSS).toMatch(/body\[data-scroll-locked]\s*\{[^}]*overflow:\s*hidden\s*!important/)
    expect(CSS).toMatch(/body\[data-scroll-locked]\s*\{[^}]*overscroll-behavior:\s*contain/)
  })

  it('reserves the scrollbar gutter so hiding the overflow does not shift the page', () => {
    // Without this, opening a modal removes the scrollbar and the page jumps by
    // its width — react-remove-scroll compensates with padding computed from a
    // runtime measurement that CSS cannot reproduce.
    expect(CSS).toMatch(/html\s*\{[^}]*scrollbar-gutter:\s*stable/)
  })

  it('uses the attribute name the installed library actually sets', () => {
    // The selector above is only right for as long as this holds. Reading it
    // from the package means an upgrade that renames it fails here rather than
    // re-breaking the scroll lock in silence.
    const component = readFileSync(
      require.resolve('react-remove-scroll-bar/dist/es2015/component.js'),
      'utf8',
    )
    const match = /lockAttribute = '([^']+)'/.exec(component)
    expect(match).not.toBeNull()
    expect(CSS).toContain(`body[${match![1]}]`)
  })
})
