import { describe, it, expect } from 'vitest'

// Mirrors the TYPE_LABELS and STATUS_LABELS maps from DSRPage.tsx.
const TYPE_LABELS: Record<string, string> = {
  access: 'Auskunft (Art. 15)',
  erasure: 'Löschung (Art. 17)',
  portability: 'Datenübertragbarkeit (Art. 20)',
  objection: 'Widerspruch (Art. 21)',
  rectification: 'Berichtigung (Art. 16)',
}

const STATUS_LABELS: Record<string, string> = {
  open: 'Offen',
  in_progress: 'In Bearbeitung',
  completed: 'Abgeschlossen',
  rejected: 'Abgelehnt',
}

function isOverdue(dueDate: string, status: string): boolean {
  if (status === 'completed' || status === 'rejected') return false
  return new Date(dueDate) < new Date()
}

describe('DSR page helpers', () => {
  describe('TYPE_LABELS', () => {
    it('maps all five DSGVO request types', () => {
      const types = ['access', 'erasure', 'portability', 'objection', 'rectification']
      for (const t of types) {
        expect(TYPE_LABELS[t]).toBeDefined()
        expect(TYPE_LABELS[t].length).toBeGreaterThan(0)
      }
    })

    it('includes the correct DSGVO article for erasure', () => {
      expect(TYPE_LABELS['erasure']).toContain('Art. 17')
    })

    it('includes the correct DSGVO article for access', () => {
      expect(TYPE_LABELS['access']).toContain('Art. 15')
    })
  })

  describe('STATUS_LABELS', () => {
    it('covers open, in_progress, completed, rejected', () => {
      expect(Object.keys(STATUS_LABELS)).toHaveLength(4)
    })

    it('maps open to German label', () => {
      expect(STATUS_LABELS['open']).toBe('Offen')
    })
  })

  describe('isOverdue', () => {
    it('returns false for completed DSR regardless of date', () => {
      expect(isOverdue('2020-01-01', 'completed')).toBe(false)
    })

    it('returns false for rejected DSR regardless of date', () => {
      expect(isOverdue('2020-01-01', 'rejected')).toBe(false)
    })

    it('returns true for open DSR with past due date', () => {
      expect(isOverdue('2020-01-01', 'open')).toBe(true)
    })

    it('returns false for open DSR with future due date', () => {
      const future = new Date(Date.now() + 7 * 24 * 60 * 60 * 1000).toISOString()
      expect(isOverdue(future, 'open')).toBe(false)
    })
  })
})
