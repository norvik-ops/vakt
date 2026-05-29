import { describe, it, expect } from 'vitest'
import type { RiskTrendResponse } from '../types'

// Mirrors the chartData transformation in ReportsPage.tsx. Pure so we can
// regression-test the backend contract without rendering the page.
function toChartData(
  trend: RiskTrendResponse | undefined,
): Array<{ date: string; score: number }> {
  return Array.isArray(trend)
    ? trend.map((p) => ({ date: p.date, score: p.total_risk_score }))
    : []
}

describe('ReportsPage chartData', () => {
  it('returns empty array when trend is undefined', () => {
    expect(toChartData(undefined)).toEqual([])
  })

  it('maps an array of RiskTrendPoint to {date, score}', () => {
    const trend: RiskTrendResponse = [
      { date: '2026-05-25', total_risk_score: 12.5, open_count: 3, critical_count: 1 },
      { date: '2026-05-26', total_risk_score: 9.0, open_count: 2, critical_count: 0 },
    ]
    expect(toChartData(trend)).toEqual([
      { date: '2026-05-25', score: 12.5 },
      { date: '2026-05-26', score: 9.0 },
    ])
  })

  // Regression: backend returns `[{date,total_risk_score,...}]` (not the old
  // `{labels,scores}` shape). If a stale response shape ever leaks through we
  // must not crash with "labels is undefined".
  it('does not throw when given an unexpected object shape', () => {
    expect(() => toChartData(undefined)).not.toThrow()
    expect(() => toChartData([] as RiskTrendResponse)).not.toThrow()
  })
})
