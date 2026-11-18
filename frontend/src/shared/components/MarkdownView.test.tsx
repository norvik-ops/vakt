import { describe, it, expect } from 'vitest'
import { render, screen } from '@testing-library/react'
import { MarkdownView } from './MarkdownView'
import { markdownToPlainText, parseMarkdownBlocks, safeHref, scanInline } from '../utils/markdown'

// UX-23: Richtlinientexte erscheinen gerendert statt als roher Markdown.

const POLICY = `# Passwortrichtlinie

## 1. Geltungsbereich

Diese Richtlinie gilt für **alle** Konten und *jeden* Zugang.

- Mindestlänge: 12 Zeichen
- Kein \`admin\` als Name

1. Erster Schritt
2. Zweiter Schritt

| Rolle | Pflicht |
|---|---|
| ISB | Freigabe |

> Hinweis: gilt ab sofort.

---

Mehr unter [Handbuch](https://example.org/handbuch).`

describe('MarkdownView', () => {
  it('rendert Überschriften, Listen, Absätze, Tabellen, Zitate und Hervorhebungen', () => {
    const { container } = render(<MarkdownView source={POLICY} />)
    // # wird h2 (die Seite trägt bereits ein h1), ## wird h3.
    expect(screen.getByRole('heading', { level: 2, name: 'Passwortrichtlinie' })).toBeInTheDocument()
    expect(screen.getByRole('heading', { level: 3, name: '1. Geltungsbereich' })).toBeInTheDocument()
    expect(container.querySelector('strong')?.textContent).toBe('alle')
    expect(container.querySelector('em')?.textContent).toBe('jeden')
    expect(container.querySelector('code')?.textContent).toBe('admin')
    expect(container.querySelectorAll('ul > li')).toHaveLength(2)
    expect(container.querySelectorAll('ol > li')).toHaveLength(2)
    expect(screen.getByRole('table')).toBeInTheDocument()
    expect(screen.getByRole('cell', { name: 'Freigabe' })).toBeInTheDocument()
    expect(container.querySelector('blockquote')?.textContent).toBe('Hinweis: gilt ab sofort.')
    expect(container.querySelector('hr')).not.toBeNull()
    const link = screen.getByRole('link', { name: 'Handbuch' })
    expect(link).toHaveAttribute('href', 'https://example.org/handbuch')
    expect(link).toHaveAttribute('rel', 'noopener noreferrer')
    // Keine Markdown-Zeichen mehr im sichtbaren Text.
    expect(container.textContent).not.toContain('##')
    expect(container.textContent).not.toContain('**')
  })

  it('führt <script> nicht aus, sondern zeigt es als Text', () => {
    const { container } = render(<MarkdownView source={'Vorher <script>window.__pwned = 1</script> nachher'} />)
    expect(container.querySelector('script')).toBeNull()
    expect(container.textContent).toContain('<script>window.__pwned = 1</script>')
    expect((window as unknown as { __pwned?: number }).__pwned).toBeUndefined()
  })

  it('erzeugt aus <img onerror> kein Element mit Event-Handler', () => {
    const { container } = render(<MarkdownView source={'<img src=x onerror="alert(1)">\n\n**fett** <b onclick="x()">b</b>'} />)
    expect(container.querySelector('img')).toBeNull()
    expect(container.querySelector('b')).toBeNull()
    expect(container.querySelector('[onerror]')).toBeNull()
    expect(container.querySelector('[onclick]')).toBeNull()
    expect(container.textContent).toContain('onerror="alert(1)"')
  })

  it('gibt javascript:- und data:-Links kein href', () => {
    const { container } = render(
      <MarkdownView source={'[klick](javascript:alert(1)) [daten](data:text/html,x) [fremd](//evil.example) [intern](/vaktcomply/policies)'} />,
    )
    const anchors = Array.from(container.querySelectorAll('a'))
    expect(anchors.map((a) => a.getAttribute('href'))).toEqual(['/vaktcomply/policies'])
    expect(container.innerHTML).not.toContain('javascript:')
    expect(container.textContent).toContain('klick')
    expect(container.textContent).toContain('daten')
  })

  it('safeHref lässt nur http(s), mailto und interne Pfade zu', () => {
    expect(safeHref('https://a.example')).toBe('https://a.example')
    expect(safeHref('mailto:isb@a.example')).toBe('mailto:isb@a.example')
    expect(safeHref('/settings')).toBe('/settings')
    expect(safeHref('#abschnitt')).toBe('#abschnitt')
    expect(safeHref('JavaScript:alert(1)')).toBeNull()
    expect(safeHref(' javascript:alert(1)')).toBeNull()
    expect(safeHref('vbscript:x')).toBeNull()
    expect(safeHref('data:text/html,x')).toBeNull()
    expect(safeHref('//evil.example')).toBeNull()
  })

  it('kursiviert keine Unterstriche mitten in Wörtern', () => {
    const { container } = render(<MarkdownView source={'Feld policy_owner_id bleibt'} />)
    expect(container.querySelector('em')).toBeNull()
    expect(container.textContent).toBe('Feld policy_owner_id bleibt')
  })
})

describe('markdownToPlainText', () => {
  it('entfernt Auszeichnung für die Kartenvorschau', () => {
    const plain = markdownToPlainText(POLICY)
    expect(plain.startsWith('Passwortrichtlinie 1. Geltungsbereich Diese Richtlinie gilt für alle Konten')).toBe(true)
    expect(plain).not.toMatch(/[#*`|>]/)
    expect(plain).toContain('Handbuch')
    expect(plain).not.toContain('https://')
  })

  it('lässt Text ohne Markdown unverändert', () => {
    expect(markdownToPlainText('Kurzbeschreibung der Richtlinie.')).toBe('Kurzbeschreibung der Richtlinie.')
  })
})

describe('parseMarkdownBlocks', () => {
  it('erkennt eine Tabelle nur mit Trennzeile', () => {
    expect(parseMarkdownBlocks('a | b\nc | d').map((b) => b.kind)).toEqual(['paragraph'])
    expect(parseMarkdownBlocks('a | b\n--|--\nc | d').map((b) => b.kind)).toEqual(['table'])
  })
})

// ReDoS-Schutz: Vorher lief das Inline-Parsing über Regexe mit überlappenden
// Quantoren (`\*([^*\s](?:[^*]*[^*\s])?)\*`, `\*\*(.+?)\*\*`), je Treffer neu
// angesetzt. Pathologische Eingaben müssen jetzt in linearer Zeit durchlaufen.
describe('Markdown — pathologische Eingaben (ReDoS)', () => {
  const CASES: Array<[string, string]> = [
    ['Sterne', '*'.repeat(50000) + 'x'],
    ['offene Klammern', '['.repeat(50000)],
    ['offene Fettungen', '**a'.repeat(20000)],
    ['Unterstriche', '_'.repeat(50000) + 'x'],
    ['Backticks', '`'.repeat(50000)],
    ['Link ohne Ende', '[a]('.repeat(20000)],
    ['Klammer-Wechsel', '[]('.repeat(20000) + ']'],
    ['Überschrift mit Leerraum', '#' + ' '.repeat(50000) + 'x'],
    ['Liste mit Leerraum', '-' + ' '.repeat(50000) + '\u2028x'],
    ['Tabelle', 'a|b\n|' + ' '.repeat(50000) + '-x'],
    ['Trennlinie', '- '.repeat(25000) + 'x'],
  ]
  for (const [name, input] of CASES) {
    it(`${name}: Parsen und Klartext < 200 ms`, () => {
      const t0 = performance.now()
      parseMarkdownBlocks(input)
      markdownToPlainText(input)
      scanInline(input)
      expect(performance.now() - t0).toBeLessThan(200)
    })
  }

  it('rendert eine pathologische Eingabe ohne Hänger', () => {
    const t0 = performance.now()
    render(<MarkdownView source={'**a'.repeat(20000)} />)
    expect(performance.now() - t0).toBeLessThan(2000)
  })

  it('erkennt „C#" nicht als schließende Raute', () => {
    expect(parseMarkdownBlocks('## Sprache C#')).toEqual([{ kind: 'heading', level: 2, text: 'Sprache C#' }])
    expect(parseMarkdownBlocks('## Titel ##')).toEqual([{ kind: 'heading', level: 2, text: 'Titel' }])
  })
})
