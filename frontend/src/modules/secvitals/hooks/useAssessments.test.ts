import { describe, it, expect } from 'vitest'
import { statusToVariant } from './useAssessments'

describe('statusToVariant', () => {
  it('returns success for green', () => {
    expect(statusToVariant('green')).toBe('success')
  })

  it('returns warning for yellow', () => {
    expect(statusToVariant('yellow')).toBe('warning')
  })

  it('returns destructive for red', () => {
    expect(statusToVariant('red')).toBe('destructive')
  })
})
