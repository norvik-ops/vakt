import { describe, it, expect } from 'vitest'
import { maturityLabel, maturityColor } from './tisax'

describe('maturityLabel', () => {
  it('returns "Nicht erfüllt" for score 0', () => {
    expect(maturityLabel(0)).toBe('Nicht erfüllt')
  })

  it('returns "Angestoßen" for score 1', () => {
    expect(maturityLabel(1)).toBe('Angestoßen')
  })

  it('returns "Teilweise" for score 2', () => {
    expect(maturityLabel(2)).toBe('Teilweise')
  })

  it('returns "Vollständig" for score 3', () => {
    expect(maturityLabel(3)).toBe('Vollständig')
  })

  it('returns "Unbekannt" for score 4', () => {
    expect(maturityLabel(4)).toBe('Unbekannt')
  })

  it('returns "Unbekannt" for score -1', () => {
    expect(maturityLabel(-1)).toBe('Unbekannt')
  })
})

describe('maturityColor', () => {
  it('returns red for score 0', () => {
    expect(maturityColor(0)).toBe('text-red-500')
  })

  it('returns orange for score 1', () => {
    expect(maturityColor(1)).toBe('text-orange-500')
  })

  it('returns yellow for score 2', () => {
    expect(maturityColor(2)).toBe('text-yellow-500')
  })

  it('returns green for score 3', () => {
    expect(maturityColor(3)).toBe('text-green-500')
  })

  it('returns secondary for unknown scores', () => {
    expect(maturityColor(99)).toBe('text-secondary')
  })
})
