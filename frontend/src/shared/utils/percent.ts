/**
 * Rounds a 0–100 share to the whole percent every view shows: half rounds up
 * (12.5 → 13). Same rule and the same epsilon as the backend
 * (backend/internal/shared/percent), so dashboard, framework pages and PDFs
 * print one number for one readiness value.
 *
 * The epsilon absorbs float noise: 14 implemented + 1 in progress of 100 is
 * exactly 14.5 %, but the computed value is 14.499999999999998.
 */
export function roundPercent(pct: number): number {
  return Math.floor(pct + 0.5 + 1e-9)
}
