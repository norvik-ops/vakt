/**
 * Gruppierung der Kontrollen im Framework-Detail (UX-19).
 *
 * ISO 27001 wurde nach den eigenen Themen der Seed-Daten gruppiert („Richtlinien",
 * „Asset Management", „Zugangskontrolle" mit A.5- und A.8-Kontrollen gemischt),
 * die Anwendbarkeitserklärung (SoA) dagegen nach den Kapiteln der Norm. Wer beide
 * Seiten nebeneinander hatte, fand dieselbe Kontrolle an zwei verschiedenen Stellen.
 * Für ISO 27001 gruppieren beide Seiten jetzt nach Anhang-A-Kapiteln A.5–A.8, in
 * der Reihenfolge der Norm (A.5.2 vor A.5.10).
 *
 * Alle anderen Frameworks behalten die Gruppierung nach `domain` — ihre Themen
 * folgen dort der Gliederung des jeweiligen Regelwerks.
 */

interface GroupableControl {
  control_id: string
  domain: string
}

const ANNEX_A = /^A\.([5-8])\.(\d+)$/

/** ISO 27001 (nicht 27017/27018): „ISO27001", „ISO 27001", „iso-27001". */
export function isISO27001(frameworkName: string | undefined): boolean {
  return (frameworkName ?? '').toLowerCase().replace(/[^a-z0-9]/g, '') === 'iso27001'
}

/** Vergleicht Kontroll-IDs nach ihren Zahlen: A.5.2 < A.5.10 < A.6.1. */
export function compareControlIds(a: string, b: string): number {
  const na = a.match(/\d+/g)?.map(Number) ?? []
  const nb = b.match(/\d+/g)?.map(Number) ?? []
  for (let i = 0; i < Math.max(na.length, nb.length); i++) {
    const d = (na[i] ?? -1) - (nb[i] ?? -1)
    if (d !== 0) return d
  }
  return a.localeCompare(b)
}

/**
 * Gruppen in Anzeigereihenfolge. `chapterLabels` bildet „5"…„8" auf die
 * Überschrift ab (dieselben Texte wie die SoA: vaktcomply.soaPage.groupA5…A8).
 * Kontrollen eines ISO-Frameworks ohne Anhang-A-ID (eigene Ergänzungen) stehen
 * danach unter ihrem Thema.
 */
export function groupControls<T extends GroupableControl>(
  controls: T[],
  frameworkName: string | undefined,
  chapterLabels: Record<string, string>,
): Array<[string, T[]]> {
  if (!isISO27001(frameworkName)) {
    const byDomain = new Map<string, T[]>()
    for (const c of controls) {
      const list = byDomain.get(c.domain) ?? []
      list.push(c)
      byDomain.set(c.domain, list)
    }
    return Array.from(byDomain.entries())
  }

  const chapters = new Map<string, T[]>()
  const rest = new Map<string, T[]>()
  for (const c of controls) {
    const m = ANNEX_A.exec(c.control_id)
    const target = m ? chapters : rest
    const key = m ? m[1] : c.domain
    const list = target.get(key) ?? []
    list.push(c)
    target.set(key, list)
  }
  const sorted = (list: T[]) => [...list].sort((a, b) => compareControlIds(a.control_id, b.control_id))
  const out: Array<[string, T[]]> = []
  for (const ch of ['5', '6', '7', '8']) {
    const list = chapters.get(ch)
    if (list) out.push([chapterLabels[ch] ?? `A.${ch}`, sorted(list)])
  }
  for (const [domain, list] of rest) out.push([domain, sorted(list)])
  return out
}
