// Centralised status/severity → visual mapping.
// Keeping this here avoids duplicating these records across 10+ pages.

// ── Finding severity ──────────────────────────────────────────────────────────

// `unknown` heißt „nicht bewertet", nicht „harmlos". Trivy und Nuclei liefern den
// Grad regulär (bei Trivy steht er in der Vorgabemenge von `--severity`), und seit
// Migration 265 speichert das Backend ihn als eigenen Zustand, statt ihn auf `info`
// abzubilden — `info` wäre die Behauptung „bewertet, und zwar als unkritisch".
export type FindingSeverity = 'info' | 'low' | 'medium' | 'high' | 'critical' | 'unknown'

/** CSS classes for severity badge-style chips (border + bg + text). */
export const findingSeverityClass: Record<FindingSeverity, string> = {
  info:     'bg-surface2 text-muted border-transparent',
  low:      'bg-severity-info-bg text-severity-info border-transparent',
  medium:   'bg-severity-medium-bg text-severity-medium border-transparent',
  high:     'bg-severity-high-bg text-severity-high border-transparent',
  critical: 'bg-severity-critical-bg text-severity-critical border-transparent',
  // Bewusst KEINE Schweregrad-Farbe und bewusst nicht die von `info`: Eine
  // gestrichelte, sichtbare Umrandung liest sich als „vorläufig, muss noch
  // eingestuft werden" und ist von jedem gefüllten Chip unterscheidbar. Gäbe man
  // ihm das Aussehen von `info`, wäre die Verwechslung, die das Backend gerade
  // verhindert, in der Oberfläche wieder da.
  unknown:  'bg-surface2 text-secondary border border-dashed border-border',
}

/** shadcn Badge `variant` for severity. */
export const findingSeverityVariant: Record<FindingSeverity, 'secondary' | 'outline' | 'warning' | 'destructive' | 'default'> = {
  info:     'secondary',
  low:      'outline',
  medium:   'warning',
  high:     'outline',
  critical: 'destructive',
  // `default` ist die Markenfarbe — als einzige weder von `info` (secondary) noch
  // von `low`/`high` (outline) belegt, also tatsächlich eigenständig.
  unknown:  'default',
}

/**
 * Numeric sort weight — higher is more severe.
 *
 * Wo steht „unbewertet" in einer nach Schwere sortierten Liste? Direkt UNTER
 * `critical` und ÜBER `high` — das ist eine bewusste Entscheidung, keine
 * Verlegenheitslösung:
 *
 *  - Ganz unten (die bequeme Antwort, und bis eben das faktische Verhalten über
 *    den `|| 0`-Fallback in FindingsPage) ist falsch. Ein unbewerteter Fund KANN
 *    kritisch sein — Trivy meldet `UNKNOWN` gerade für Schwachstellen, die noch
 *    keine Bewertung haben, und die werden später regelmäßig HIGH oder CRITICAL.
 *    Sortiert man sie unter `info`, sieht sie in einer langen Liste niemand, und
 *    die Sicherheitsfolge, die das Backend gerade beseitigt hat, wäre über die
 *    Sortierung wieder da.
 *  - Ganz oben ist ebenfalls falsch: Das behauptete, ein unbewerteter Fund sei
 *    schlimmer als ein bestätigt kritischer, und schöbe echte Kritische nach
 *    unten. Bei vielen Unbewerteten (Trivy liefert `UNKNOWN` für OS-Pakete recht
 *    häufig) macht das die Sortierung unbrauchbar.
 *  - Dazwischen ist die ehrliche Aussage: Wir können nicht ausschließen, dass es
 *    kritisch ist, also gehört es vor alles, von dem wir WISSEN, dass es nur hoch
 *    ist. Ein bestätigt kritischer Fund bleibt trotzdem vorn, weil Gewissheit bei
 *    knapper Zeit den Ausschlag gibt.
 *
 * Die eigentliche Handlung bei `unknown` ist Triage, nicht Behebung — die
 * Sortierung stellt sicher, dass sie überhaupt stattfindet.
 */
export const findingSeverityOrder: Record<FindingSeverity, number> = {
  critical: 6, unknown: 5, high: 4, medium: 3, low: 2, info: 1,
}

/**
 * Sortiergewicht für einen Schweregrad, der als beliebiger String aus der API
 * kommt.
 *
 * Ein Grad, den die Tabelle nicht kennt, bekommt das Gewicht von `unknown` —
 * denn genau das ist er: ein Fund, den wir nicht einstufen konnten. Ihn
 * stattdessen auf 0 zu setzen, schöbe ihn unter `info` ans Listenende, wo ihn
 * niemand sieht.
 *
 * `hasOwnProperty` statt `in` oder `?? `: `in` trifft auch Prototyp-Schlüssel
 * (`'toString' in obj` ist wahr) und lieferte dann eine Funktion statt einer
 * Zahl. Und ein `??` hinter einem `as`-Cast prüft nichts — der Cast behauptet
 * dem Typsystem gegenüber, der Wert könne nicht fehlen, und der Fallback wäre
 * toter Code (genau das hat der Linter zu Recht angemerkt).
 */
export function findingSeverityWeight(severity: string): number {
  return Object.prototype.hasOwnProperty.call(findingSeverityOrder, severity)
    ? findingSeverityOrder[severity as FindingSeverity]
    : findingSeverityOrder.unknown
}

// ── Campaign status ───────────────────────────────────────────────────────────

export type CampaignStatus = 'draft' | 'scheduled' | 'running' | 'completed' | 'aborted'

export const campaignStatusVariant: Record<CampaignStatus, 'secondary' | 'default' | 'success' | 'destructive'> = {
  draft:     'secondary',
  scheduled: 'default',
  running:   'default',
  completed: 'success',
  aborted:   'destructive',
}

// ── Generic job / scan status ─────────────────────────────────────────────────
// Covers Report.status (pending/processing/completed/failed)
// and GitScan.status (pending/running/completed/failed).

export type JobStatus = 'pending' | 'processing' | 'running' | 'completed' | 'failed'

export const jobStatusVariant: Record<JobStatus, 'secondary' | 'default' | 'success' | 'destructive'> = {
  pending:    'secondary',
  processing: 'default',
  running:    'default',
  completed:  'success',
  failed:     'destructive',
}

// ── Assignment status (Vakt Aware) ────────────────────────────────────────────

export type AssignmentStatus = 'assigned' | 'completed' | 'failed'

export const assignmentStatusVariant: Record<AssignmentStatus, 'secondary' | 'success' | 'destructive'> = {
  assigned:  'secondary',
  completed: 'success',
  failed:    'destructive',
}
