import { describe, it, expect } from 'vitest'
import { readFileSync } from 'node:fs'
import { join } from 'node:path'
import { riskLevel, RISK_LEVELS } from './riskLevel'

// Shared with backend/internal/shared/risklevel/risklevel_test.go: a boundary
// that moves in Go or TypeScript only turns one of the two tests red.
const FIXTURE = JSON.parse(
  readFileSync(join(process.cwd(), '../backend/internal/shared/risklevel/testdata/risk-levels.json'), 'utf8'),
) as { cases: { score: number; level: string }[] }

// R1-KW-17: every view (matrix legend, list groups, badges, residual risk,
// dashboard, SecVitals, methodology text) reads its level from this function.
// The grid points are the boundaries of the agreed 15/10/5 scheme — each
// assertion fails if a boundary moves by one in either direction.
describe('riskLevel', () => {
  it.each([
    [1, 'low'],
    [4, 'low'],
    [5, 'medium'],
    [9, 'medium'],
    [10, 'high'],
    [14, 'high'],
    [15, 'critical'],
    [25, 'critical'],
  ] as const)('score %i → %s', (score, level) => {
    expect(riskLevel(score)).toBe(level)
  })

  it('matches the shared Go/TS fixture for every score 0–25', () => {
    expect(FIXTURE.cases).toHaveLength(26)
    for (const c of FIXTURE.cases) {
      expect(riskLevel(c.score), `score ${c.score}`).toBe(c.level)
    }
  })

  it('gross and residual risk use the same cut (residual 12 is high, 6 is medium)', () => {
    expect(riskLevel(12)).toBe('high')
    expect(riskLevel(6)).toBe('medium')
  })

  it('RISK_LEVELS covers 1–25 without gaps or overlap, highest level first', () => {
    expect(RISK_LEVELS.map((r) => r.level)).toEqual(['critical', 'high', 'medium', 'low'])
    for (let s = 1; s <= 25; s++) {
      const hits = RISK_LEVELS.filter((r) => r.from <= s && s <= r.to)
      expect(hits, `score ${s}`).toHaveLength(1)
      expect(hits[0]?.level).toBe(riskLevel(s))
    }
  })
})
