/**
 * Parser für den kleinen Markdown-Ausschnitt der Richtlinientexte (UX-23).
 * Gerendert wird in shared/components/MarkdownView.tsx — Begründung und
 * Sicherheitsmodell stehen dort.
 */

export type MarkdownBlock =
  | { kind: 'heading'; level: number; text: string }
  | { kind: 'paragraph'; text: string }
  | { kind: 'list'; ordered: boolean; items: string[] }
  | { kind: 'quote'; text: string }
  | { kind: 'code'; text: string }
  | { kind: 'table'; header: string[]; rows: string[][] }
  | { kind: 'hr' }

// Keine Regex mit verschachtelten oder überlappenden Quantoren auf Nutzertext:
// Zeilenformen werden von Hand erkannt, Inline-Auszeichnung von einem linearen
// Scanner (scanInline). Vorher lief das Inline-Parsing über eine Regex wie
// `\*([^*\s](?:[^*]*[^*\s])?)\*` und `\*\*(.+?)\*\*`, je Treffer neu ab dem Rest
// angesetzt — bei `'*'.repeat(50000)` polynomiell bis kubisch (ReDoS).

function isWs(c: string | undefined): boolean {
  return c === ' ' || c === '\t' || c === '\f' || c === '\v' || c === '\u00a0'
}

function isWordChar(c: string | undefined): boolean {
  return c !== undefined && /[A-Za-z0-9_]/.test(c)
}

/** `# Titel ##` → { level, text }; null, wenn keine Überschrift. */
function parseHeading(line: string): { level: number; text: string } | null {
  let n = 0
  while (n < line.length && line[n] === '#') n++
  if (n < 1 || n > 6 || !isWs(line[n])) return null
  let text = line.slice(n).trim()
  let e = text.length
  while (e > 0 && text[e - 1] === '#') e--
  // Schließende #-Folge nur, wenn sie allein steht oder ein Leerzeichen davor hat („C#" bleibt).
  if (e === 0) text = ''
  else if (e < text.length && isWs(text[e - 1])) text = text.slice(0, e).trimEnd()
  return { level: n, text }
}

/** `---`, `* * *`, `___` (mind. drei gleiche Zeichen, Leerraum egal). */
function isHr(line: string): boolean {
  let ch = ''
  let count = 0
  for (const c of line) {
    if (isWs(c)) continue
    if (c !== '-' && c !== '*' && c !== '_') return false
    if (ch === '') ch = c
    else if (c !== ch) return false
    count++
  }
  return count >= 3
}

/** Trennzeile einer Tabelle: `|---|:--:|`. */
function isTableSep(line: string): boolean {
  let s = line.trim()
  if (s.startsWith('|')) s = s.slice(1)
  if (s.endsWith('|')) s = s.slice(0, -1)
  if (s === '') return false
  return s.split('|').every((cell) => /^:?-{2,}:?$/.test(cell.trim()))
}

/** Inhalt eines Listenpunkts oder null. ordered: `1.`/`1)`, sonst `-`/`*`/`+`. */
function listItem(line: string, ordered: boolean): string | null {
  let j = 0
  while (j < line.length && isWs(line[j])) j++
  if (ordered) {
    const d = j
    while (j < line.length && line[j] >= '0' && line[j] <= '9') j++
    if (j === d || (line[j] !== '.' && line[j] !== ')')) return null
  } else if (line[j] !== '-' && line[j] !== '*' && line[j] !== '+') {
    return null
  }
  if (!isWs(line[j + 1])) return null
  return line.slice(j + 1).trim()
}

const isBullet = (l: string) => listItem(l, false) !== null
const isOrdered = (l: string) => listItem(l, true) !== null

function splitRow(line: string): string[] {
  let s = line.trim()
  if (s.startsWith('|')) s = s.slice(1)
  if (s.endsWith('|')) s = s.slice(0, -1)
  return s.split('|').map((c) => c.trim())
}

/** Zerlegt Markdown in Blöcke. Exportiert für Tests. */
export function parseMarkdownBlocks(src: string): MarkdownBlock[] {
  const lines = src.replace(/\r\n?/g, '\n').split('\n')
  const blocks: MarkdownBlock[] = []
  let i = 0
  while (i < lines.length) {
    const line = lines[i]
    if (line.trim() === '') { i++; continue }

    if (line.trim().startsWith('```')) {
      const body: string[] = []
      i++
      while (i < lines.length && !lines[i].trim().startsWith('```')) { body.push(lines[i]); i++ }
      i++ // schließender Zaun (falls vorhanden)
      blocks.push({ kind: 'code', text: body.join('\n') })
      continue
    }

    const h = parseHeading(line)
    if (h) { blocks.push({ kind: 'heading', level: h.level, text: h.text }); i++; continue }

    if (isHr(line)) { blocks.push({ kind: 'hr' }); i++; continue }

    if (line.includes('|') && i + 1 < lines.length && isTableSep(lines[i + 1])) {
      const header = splitRow(line)
      i += 2
      const rows: string[][] = []
      while (i < lines.length && lines[i].includes('|') && lines[i].trim() !== '') {
        rows.push(splitRow(lines[i]))
        i++
      }
      blocks.push({ kind: 'table', header, rows })
      continue
    }

    if (line.trimStart().startsWith('>')) {
      const body: string[] = []
      while (i < lines.length && lines[i].trimStart().startsWith('>')) {
        body.push(lines[i].trimStart().replace(/^>\s?/, ''))
        i++
      }
      blocks.push({ kind: 'quote', text: body.join(' ') })
      continue
    }

    const ordered = isOrdered(line)
    if (ordered || isBullet(line)) {
      const items: string[] = []
      while (i < lines.length) {
        const item = listItem(lines[i], ordered)
        if (item !== null) { items.push(item); i++; continue }
        // Fortsetzungszeile eines Punkts (eingerückt, nicht leer)
        if (lines[i].trim() !== '' && isWs(lines[i][0]) && items.length > 0 &&
            !isBullet(lines[i]) && !isOrdered(lines[i])) {
          items[items.length - 1] += ' ' + lines[i].trim()
          i++
          continue
        }
        break
      }
      blocks.push({ kind: 'list', ordered, items })
      continue
    }

    const para: string[] = []
    while (
      i < lines.length && lines[i].trim() !== '' && !parseHeading(lines[i]) &&
      !isBullet(lines[i]) && !isOrdered(lines[i]) && !isHr(lines[i]) &&
      !lines[i].trimStart().startsWith('>') && !lines[i].trim().startsWith('```')
    ) {
      para.push(lines[i].trim())
      i++
    }
    blocks.push({ kind: 'paragraph', text: para.join(' ') })
  }
  return blocks
}

/**
 * Erlaubte Linkziele: http(s), mailto und Pfade innerhalb von Vakt („/…", „#…").
 * Alles andere (javascript:, data:, vbscript:, „//fremd.example") bekommt kein href.
 */
export function safeHref(url: string): string | null {
  const u = url.trim()
  if (/^(https?:|mailto:)/i.test(u)) return u
  if (u.startsWith('#')) return u
  if (u.startsWith('/') && !u.startsWith('//')) return u
  return null
}

export type InlineToken =
  | { kind: 'text'; text: string }
  | { kind: 'code'; text: string }
  | { kind: 'link'; text: string; url: string }
  | { kind: 'strong'; text: string }
  | { kind: 'em'; text: string }

/**
 * indexOf mit Gedächtnis: Der Scanner fragt mit steigender Startposition nach
 * demselben Zeichen. Ohne Gedächtnis kostete `'['.repeat(n)` n-mal einen
 * Suchlauf bis ans Ende (quadratisch); so wird jede Fundstelle einmal gesucht.
 */
function makeFinder(text: string) {
  const cache = new Map<string, { from: number; pos: number }>()
  return (needle: string, from: number): number => {
    const c = cache.get(needle)
    if (c && from >= c.from && (c.pos === -1 || c.pos >= from)) return c.pos
    const pos = text.indexOf(needle, from)
    cache.set(needle, { from, pos })
    return pos
  }
}

/**
 * Zerlegt eine Zeile in Text, `code`, [Link](url), **fett** oder __fett__ und
 * *kursiv* oder _kursiv_. Linear: jede Position wird einmal betrachtet, je
 * Auszeichnung gibt es genau einen Kandidaten für das Ende (das nächste
 * Schlusszeichen) — passt der nicht, ist das Zeichen gewöhnlicher Text.
 * Inhalt von Link/fett/kursiv liefert der Aufrufer erneut an scanInline.
 */
export function scanInline(text: string): InlineToken[] {
  const out: InlineToken[] = []
  const find = makeFinder(text)
  let seg = 0
  let i = 0
  const push = (tok: InlineToken, start: number, end: number) => {
    if (start > seg) out.push({ kind: 'text', text: text.slice(seg, start) })
    out.push(tok)
    seg = end
    i = end
  }
  while (i < text.length) {
    const c = text[i]
    if (c === '`') {
      const k = find('`', i + 1)
      if (k > i + 1) { push({ kind: 'code', text: text.slice(i + 1, k) }, i, k + 1); continue }
    } else if (c === '[') {
      const j = find(']', i + 1)
      if (j > i + 1 && text[j + 1] === '(') {
        const k = find(')', j + 2)
        if (k > j + 2) {
          const url = text.slice(j + 2, k)
          if (!/\s/.test(url)) { push({ kind: 'link', text: text.slice(i + 1, j), url }, i, k + 1); continue }
        }
      }
    } else if (c === '*' || c === '_') {
      const under = c === '_'
      // Unterstriche nur an Wortgrenzen: policy_owner_id bleibt, wie es ist.
      if (!under || !isWordChar(text[i - 1])) {
        if (text[i + 1] === c) {
          const k = find(c + c, i + 2)
          if (k > i + 2 && (!under || !isWordChar(text[k + 2]))) {
            push({ kind: 'strong', text: text.slice(i + 2, k) }, i, k + 2)
            continue
          }
        }
        const k = find(c, i + 1)
        if (k > i + 1 && !isWs(text[i + 1]) && !isWs(text[k - 1]) && (!under || !isWordChar(text[k + 1]))) {
          push({ kind: 'em', text: text.slice(i + 1, k) }, i, k + 1)
          continue
        }
      }
    }
    i++
  }
  if (seg < text.length) out.push({ kind: 'text', text: text.slice(seg) })
  return out
}

function inlineToPlain(text: string): string {
  return scanInline(text)
    .map((t) => (t.kind === 'text' || t.kind === 'code' ? t.text : inlineToPlain(t.text)))
    .join('')
}

/**
 * Markdown als Fließtext ohne Auszeichnung — für einzeilige Vorschauen wie die
 * Richtlinienkarte, wo „# Titel ## 1. Geltungsbereich" sonst roh dasteht.
 */
export function markdownToPlainText(src: string): string {
  return parseMarkdownBlocks(src)
    .map((b) => {
      switch (b.kind) {
        case 'heading':
        case 'paragraph':
        case 'quote':
          return inlineToPlain(b.text)
        case 'code':
          return b.text
        case 'list':
          return b.items.map(inlineToPlain).join(' · ')
        case 'table':
          return [b.header, ...b.rows].map((r) => r.map(inlineToPlain).join(' ')).join(' ')
        case 'hr':
          return ''
      }
    })
    .filter((s) => s !== '')
    .join(' ')
}

