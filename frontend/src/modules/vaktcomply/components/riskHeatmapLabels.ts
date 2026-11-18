/** The 1–5 scale of likelihood and impact in the risk heatmap. */
const SCALE = [1, 2, 3, 4, 5] as const

/**
 * Axis labels for the 1–5 scale from an i18n array (`returnObjects: true`).
 *
 * UX-25: while the keys were missing, i18next returned the key name as a
 * string and the axes indexed its single letters ("v a k t c"). Anything that
 * is not an array of five non-empty strings falls back to the plain scale
 * numbers "1" … "5".
 */
export function scaleLabels(value: unknown): string[] {
  if (
    Array.isArray(value) &&
    value.length === SCALE.length &&
    value.every((v) => typeof v === 'string' && v !== '')
  ) {
    return value as string[]
  }
  return SCALE.map(String)
}

/** Which value places a risk in the matrix: gross risk or residual risk (UX-27). */
export type HeatmapMode = 'inherent' | 'residual'

interface CellSource {
  likelihood: number
  impact: number
  residual_likelihood?: number | null
  residual_impact?: number | null
}

/**
 * Matrix cell of a risk in the chosen mode: likelihood/impact for the gross risk,
 * residual likelihood/impact for the residual risk. Null while the residual risk
 * is not assessed — such a risk has no place in the residual matrix.
 */
export function heatmapCell(risk: CellSource, mode: HeatmapMode): { likelihood: number; impact: number } | null {
  if (mode === 'inherent') return { likelihood: risk.likelihood, impact: risk.impact }
  if (risk.residual_likelihood == null || risk.residual_impact == null) return null
  return { likelihood: risk.residual_likelihood, impact: risk.residual_impact }
}
