/**
 * TISAX / VDA ISA maturity level utilities.
 * Maturity levels follow the VDA ISA scale:
 *   0 = Nicht erfüllt
 *   1 = Angestoßen
 *   2 = Teilweise
 *   3 = Vollständig
 */

const MATURITY_LABELS: Record<number, string> = {
  0: 'Nicht erfüllt',
  1: 'Angestoßen',
  2: 'Teilweise',
  3: 'Vollständig',
}

/**
 * Returns the German label for a TISAX maturity score (0–3).
 * Returns 'Unbekannt' for values outside the valid range.
 */
export function maturityLabel(score: number): string {
  return MATURITY_LABELS[score] ?? 'Unbekannt'
}

/**
 * Returns a Tailwind CSS text-color class for a TISAX maturity score (0–3).
 */
export function maturityColor(score: number): string {
  switch (score) {
    case 0: return 'text-red-500'
    case 1: return 'text-orange-500'
    case 2: return 'text-yellow-500'
    case 3: return 'text-green-500'
    default: return 'text-secondary'
  }
}
