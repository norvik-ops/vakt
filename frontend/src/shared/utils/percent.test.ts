import { describe, it, expect } from 'vitest'
import { roundPercent } from './percent'

describe('roundPercent — dieselbe Rundung wie backend/internal/shared/percent', () => {
  it('rundet halbe Werte auf', () => {
    expect(roundPercent(12.5)).toBe(13)
    expect(roundPercent(2.5)).toBe(3)
    expect(roundPercent(0.5)).toBe(1)
    expect(roundPercent(18.75)).toBe(19)
  })

  it('rundet darunter ab', () => {
    expect(roundPercent(0)).toBe(0)
    expect(roundPercent(0.49)).toBe(0)
    expect(roundPercent(5.113636)).toBe(5)
    expect(roundPercent(100)).toBe(100)
  })

  it('gleicht Gleitkomma-Rauschen bei genau x,5 aus (14 + 1 halb von 100)', () => {
    const pct = ((14 + 1 * 0.5) / 100) * 100
    expect(pct).toBeLessThan(14.5)
    expect(roundPercent(pct)).toBe(15)
  })
})
