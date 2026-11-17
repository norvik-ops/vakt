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
