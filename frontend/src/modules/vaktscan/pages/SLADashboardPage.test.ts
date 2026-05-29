import { describe, it, expect } from 'vitest'

// Pure helper extracted for testability — mirrors the inline slaPercent logic.
function slaPercent(daysOpen: number, slaDays: number): number {
  if (slaDays === 0) return 999
  return Math.round((daysOpen / slaDays) * 100)
}

function isOverdue(daysOpen: number, slaDays: number): boolean {
  return daysOpen > slaDays
}

function barColor(pct: number, overdue: boolean): string {
  if (overdue || pct > 90) return 'bg-red-500'
  if (pct > 50) return 'bg-amber-400'
  return 'bg-green-500'
}

describe('SLA dashboard helpers', () => {
  describe('slaPercent', () => {
    it('returns 0 when finding has just been opened', () => {
      expect(slaPercent(0, 7)).toBe(0)
    })

    it('returns 100 when finding is exactly at the SLA boundary', () => {
      expect(slaPercent(7, 7)).toBe(100)
    })

    it('returns > 100 when overdue', () => {
      expect(slaPercent(10, 7)).toBeGreaterThan(100)
    })

    it('handles zero slaDays without dividing by zero', () => {
      expect(slaPercent(5, 0)).toBe(999)
    })

    it('rounds to integer', () => {
      const result = slaPercent(1, 3)
      expect(Number.isInteger(result)).toBe(true)
    })
  })

  describe('isOverdue', () => {
    it('returns false when within SLA', () => {
      expect(isOverdue(6, 7)).toBe(false)
    })

    it('returns false when exactly at boundary', () => {
      expect(isOverdue(7, 7)).toBe(false)
    })

    it('returns true when one day over', () => {
      expect(isOverdue(8, 7)).toBe(true)
    })
  })

  describe('barColor', () => {
    it('is green below 50%', () => {
      expect(barColor(49, false)).toBe('bg-green-500')
    })

    it('is amber between 50% and 90%', () => {
      expect(barColor(75, false)).toBe('bg-amber-400')
    })

    it('is red above 90%', () => {
      expect(barColor(91, false)).toBe('bg-red-500')
    })

    it('is red when overdue regardless of percentage', () => {
      expect(barColor(30, true)).toBe('bg-red-500')
    })
  })
})
