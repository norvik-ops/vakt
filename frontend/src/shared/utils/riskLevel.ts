// Risk levels for the 5×5 risk score (likelihood × impact, 1–25).
//
// Single source for every view that names or colours a level: matrix legend and
// cell colour, list groups, score badges, residual risk, dashboard top 5,
// SecVitals and the methodology text (R1-KW-17 — before this, six views cut the
// scale at six different points). Gross and residual risk use the same cut.
// The backend mirrors this scheme in internal/shared/risklevel.

export type RiskLevel = 'critical' | 'high' | 'medium' | 'low'

/** Lower bounds of the levels (backend twin: risklevel.CriticalFrom/HighFrom/MediumFrom). */
export const CRITICAL_FROM = 15
export const HIGH_FROM = 10
export const MEDIUM_FROM = 5

/** Score range per level, highest level first. Ranges cover 1–25 without gaps. */
export const RISK_LEVELS: readonly { level: RiskLevel; from: number; to: number }[] = [
  { level: 'critical', from: CRITICAL_FROM, to: 25 },
  { level: 'high', from: HIGH_FROM, to: CRITICAL_FROM - 1 },
  { level: 'medium', from: MEDIUM_FROM, to: HIGH_FROM - 1 },
  { level: 'low', from: 1, to: MEDIUM_FROM - 1 },
]

export function riskLevel(score: number): RiskLevel {
  if (score >= CRITICAL_FROM) return 'critical'
  if (score >= HIGH_FROM) return 'high'
  if (score >= MEDIUM_FROM) return 'medium'
  return 'low'
}

/** Badge classes per level (list cards, risk detail, residual risk, create dialog). */
export const RISK_LEVEL_BADGE: Record<RiskLevel, string> = {
  critical: 'bg-red-500/20 text-red-400 border-red-500/30',
  high: 'bg-orange-500/20 text-orange-400 border-orange-500/30',
  medium: 'bg-yellow-500/20 text-yellow-400 border-yellow-500/30',
  low: 'bg-green-500/20 text-green-400 border-green-500/30',
}

/** Light fill colours of the risk matrix (cells, legend, methodology). */
export const RISK_LEVEL_FILL: Record<RiskLevel, string> = {
  critical: '#fecaca',
  high: '#fed7aa',
  medium: '#fef3c7',
  low: '#d1fae5',
}

/** i18n key of the level name („Kritisch", „Hoch", „Mittel", „Niedrig"). */
export const RISK_LEVEL_LABEL_KEY: Record<RiskLevel, string> = {
  critical: 'vaktcomply.risksPage.levelCritical',
  high: 'vaktcomply.risksPage.levelHigh',
  medium: 'vaktcomply.risksPage.levelMedium',
  low: 'vaktcomply.risksPage.levelLow',
}
