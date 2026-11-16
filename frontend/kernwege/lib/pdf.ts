import zlib from 'node:zlib'

/**
 * Minimaler PDF-Textleser für die Prüfung von Exporten (Dossier, Berichte). Keine
 * Abhängigkeit, wie lib/zip.ts: der Prüfer installiert nichts, was das Produkt nicht
 * auch hat.
 *
 * Abgedeckt ist genau das, was die Generatoren des Backends erzeugen (go-pdf/fpdf mit
 * pdfutil: DejaVu Sans als UTF-8-TrueType, Identity-H, Inhaltsströme mit FlateDecode).
 * fpdf schreibt jeden Zelltext als EINEN Literal-String `(…) Tj`, kodiert als UTF-16BE
 * ohne BOM. Der Leser entpackt alle Ströme, sammelt die Strings vor `Tj`/`TJ` in
 * Reihenfolge und dekodiert sie.
 *
 * Was er NICHT kann: Layout (Spalten, Zeilenumbrüche, Lesereihenfolge über Seiten
 * jenseits der Stromreihenfolge), Fonts mit eigener Kodierung ohne Unicode-CIDs,
 * verschlüsselte PDFs, Objektströme. Liefert ein Generator so etwas, findet der Leser
 * keinen Text — die Prüfung wird dann rot, nicht still grün.
 */
export function pdfTexts(buf: Buffer): string[] {
  if (buf.subarray(0, 5).toString('latin1') !== '%PDF-') throw new Error('kein PDF (Kopf %PDF- fehlt)')
  const out: string[] = []
  const src = buf.toString('latin1')
  const streamRe = /(?<!end)stream\r?\n/g
  let m: RegExpExecArray | null
  while ((m = streamRe.exec(src)) !== null) {
    const start = m.index + m[0].length
    const end = src.indexOf('endstream', start)
    if (end < 0) break
    // Das Wörterbuch vor `stream` sagt, ob der Strom gepackt ist.
    const dictStart = src.lastIndexOf('obj', m.index)
    const dict = src.slice(dictStart, m.index)
    streamRe.lastIndex = end
    // Fonts und Bilder tragen keinen Text, und ihr Inhalt sähe für den Leser zufällig aus.
    if (/\/Length1|\/Subtype\s*\/Image|\/FontFile/.test(dict)) continue
    let data = buf.subarray(start, end)
    if (/\/FlateDecode/.test(dict)) {
      try {
        data = zlib.inflateSync(data, { finishFlush: zlib.constants.Z_SYNC_FLUSH })
      } catch {
        continue // Font-Daten o. Ä. — kein Inhaltsstrom
      }
    }
    out.push(...contentStrings(data))
  }
  return out
}

/** Alle Texte eines PDFs, eine Zeile je Textoperator. */
export function pdfText(buf: Buffer): string {
  return pdfTexts(buf).join('\n')
}

function contentStrings(data: Buffer): string[] {
  const s = data.toString('latin1')
  if (!/\bT[jJ]\b/.test(s)) return []
  const out: string[] = []
  let i = 0
  let pending: Buffer[] = []
  while (i < s.length) {
    const ch = s[i]
    if (ch === '(') {
      const [bytes, next] = readLiteral(data, i)
      pending.push(bytes)
      i = next
    } else if (ch === '<' && s[i + 1] !== '<') {
      const close = s.indexOf('>', i)
      const hex = s.slice(i + 1, close).replace(/\s+/g, '')
      pending.push(Buffer.from(hex.length % 2 ? hex + '0' : hex, 'hex'))
      i = close + 1
    } else if (ch === 'T' && (s[i + 1] === 'j' || s[i + 1] === 'J')) {
      if (pending.length) out.push(pending.map(decode).join(''))
      pending = []
      i += 2
    } else if (ch === "'" || ch === '"') {
      if (pending.length) out.push(pending.map(decode).join(''))
      pending = []
      i += 1
    } else {
      // Ein anderer Operator zwischen String und Tj (kommt bei fpdf nicht vor) verwirft nichts:
      // erst beim nächsten Tj/TJ wird ausgegeben.
      i += 1
    }
  }
  return out
}

/** Liest einen Literal-String ab Position der öffnenden Klammer (PDF 1.7, 7.3.4.2). */
function readLiteral(data: Buffer, at: number): [Buffer, number] {
  const bytes: number[] = []
  let depth = 0
  let i = at
  for (; i < data.length; i++) {
    const c = data[i]!
    if (c === 0x5c) { // Backslash
      const n = data[++i]!
      const map: Record<number, number> = { 0x6e: 0x0a, 0x72: 0x0d, 0x74: 0x09, 0x62: 0x08, 0x66: 0x0c }
      if (map[n] !== undefined) bytes.push(map[n]!)
      else if (n >= 0x30 && n <= 0x37) {
        let oct = String.fromCharCode(n)
        while (oct.length < 3 && data[i + 1]! >= 0x30 && data[i + 1]! <= 0x37) oct += String.fromCharCode(data[++i]!)
        bytes.push(parseInt(oct, 8) & 0xff)
      } else if (n === 0x0d || n === 0x0a) {
        if (n === 0x0d && data[i + 1] === 0x0a) i++
      } else bytes.push(n)
      continue
    }
    if (c === 0x28) { depth++; if (depth === 1) continue }
    if (c === 0x29) { depth--; if (depth === 0) { i++; break } }
    bytes.push(c)
  }
  return [Buffer.from(bytes), i]
}

/** fpdf-UTF-8-Fonts: UTF-16BE ohne BOM. Ungerade Länge → kein UTF-16, dann Latin-1. */
function decode(b: Buffer): string {
  if (b.length >= 2 && b[0] === 0xfe && b[1] === 0xff) b = b.subarray(2)
  if (b.length % 2 === 0 && b.length > 0) {
    const sw = Buffer.from(b)
    sw.swap16()
    return sw.toString('utf16le')
  }
  return b.toString('latin1')
}
