import zlib from 'node:zlib'

/**
 * Minimaler ZIP-Leser für die Prüfung von Exporten (Audit-Paket, XLSX, DOCX).
 * Liest das zentrale Verzeichnis und entpackt „stored" (0) und „deflate" (8) —
 * mehr erzeugt Gos archive/zip nicht. Keine Abhängigkeit, damit der Prüfer nichts
 * installiert, was das Produkt nicht auch hat.
 */
export function unzip(buf: Buffer): Map<string, Buffer> {
  const out = new Map<string, Buffer>()
  // End of Central Directory: Signatur 0x06054b50, von hinten suchen (Kommentar ≤ 64 KiB).
  let eocd = -1
  for (let i = buf.length - 22; i >= Math.max(0, buf.length - 22 - 0xffff); i--) {
    if (buf.readUInt32LE(i) === 0x06054b50) { eocd = i; break }
  }
  if (eocd < 0) throw new Error('kein ZIP (End-of-Central-Directory fehlt)')
  const entries = buf.readUInt16LE(eocd + 10)
  let p = buf.readUInt32LE(eocd + 16)
  for (let n = 0; n < entries; n++) {
    if (buf.readUInt32LE(p) !== 0x02014b50) throw new Error('ZIP: zentrales Verzeichnis beschädigt')
    const method = buf.readUInt16LE(p + 10)
    const csize = buf.readUInt32LE(p + 20)
    const nameLen = buf.readUInt16LE(p + 28)
    const extraLen = buf.readUInt16LE(p + 30)
    const commentLen = buf.readUInt16LE(p + 32)
    const localOff = buf.readUInt32LE(p + 42)
    const name = buf.subarray(p + 46, p + 46 + nameLen).toString('utf8')
    const lNameLen = buf.readUInt16LE(localOff + 26)
    const lExtraLen = buf.readUInt16LE(localOff + 28)
    const dataStart = localOff + 30 + lNameLen + lExtraLen
    const data = buf.subarray(dataStart, dataStart + csize)
    if (method === 0) out.set(name, Buffer.from(data))
    else if (method === 8) out.set(name, zlib.inflateRawSync(data))
    else throw new Error(`ZIP: Kompressionsmethode ${method} nicht unterstützt (${name})`)
    p += 46 + nameLen + extraLen + commentLen
  }
  return out
}

/** Alle Zelltexte eines XLSX-Arbeitsblatts (shared strings + inline), zeilenweise. */
export function xlsxRows(buf: Buffer, sheet = 'xl/worksheets/sheet1.xml'): string[][] {
  const files = unzip(buf)
  const sst = files.get('xl/sharedStrings.xml')?.toString('utf8') ?? ''
  const shared = [...sst.matchAll(/<si>([\s\S]*?)<\/si>/g)].map((m) =>
    [...m[1].matchAll(/<t[^>]*>([\s\S]*?)<\/t>/g)].map((t) => decodeXml(t[1])).join(''),
  )
  const xml = files.get(sheet)?.toString('utf8')
  if (!xml) throw new Error(`XLSX: ${sheet} fehlt`)
  return [...xml.matchAll(/<row[^>]*>([\s\S]*?)<\/row>/g)].map((row) =>
    [...row[1].matchAll(/<c([^>]*?)(?:\/>|>([\s\S]*?)<\/c>)/g)].map((c) => {
      const attrs = c[1]
      const inner = c[2] ?? ''
      const v = /<v>([\s\S]*?)<\/v>/.exec(inner)?.[1]
      if (/t="s"/.test(attrs) && v !== undefined) return shared[Number(v)] ?? ''
      const is = /<t[^>]*>([\s\S]*?)<\/t>/.exec(inner)?.[1]
      return decodeXml(is ?? v ?? '')
    }),
  )
}

function decodeXml(s: string): string {
  return s.replace(/&lt;/g, '<').replace(/&gt;/g, '>').replace(/&quot;/g, '"').replace(/&apos;/g, "'").replace(/&amp;/g, '&')
}
