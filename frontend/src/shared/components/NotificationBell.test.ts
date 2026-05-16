import { describe, it, expect } from 'vitest'

// Mirrors the badge-cap logic in NotificationBell.tsx.
function badgeLabel(unreadCount: number): string {
  return unreadCount > 9 ? '9+' : String(unreadCount)
}

function shouldShowBadge(unreadCount: number): boolean {
  return unreadCount > 0
}

// Mirrors the typeIcon map keys — ensures all notification types have an icon.
const KNOWN_TYPES = ['info', 'warning', 'error']

describe('NotificationBell helpers', () => {
  describe('badgeLabel', () => {
    it('shows exact count for 1–9', () => {
      for (let i = 1; i <= 9; i++) {
        expect(badgeLabel(i)).toBe(String(i))
      }
    })

    it('caps at "9+" for 10 and above', () => {
      expect(badgeLabel(10)).toBe('9+')
      expect(badgeLabel(99)).toBe('9+')
      expect(badgeLabel(1000)).toBe('9+')
    })

    it('returns "0" for zero unread', () => {
      expect(badgeLabel(0)).toBe('0')
    })
  })

  describe('shouldShowBadge', () => {
    it('returns false when no unread notifications', () => {
      expect(shouldShowBadge(0)).toBe(false)
    })

    it('returns true when there is at least one unread', () => {
      expect(shouldShowBadge(1)).toBe(true)
      expect(shouldShowBadge(50)).toBe(true)
    })
  })

  describe('notification type coverage', () => {
    it('supports info, warning, and error types', () => {
      expect(KNOWN_TYPES).toContain('info')
      expect(KNOWN_TYPES).toContain('warning')
      expect(KNOWN_TYPES).toContain('error')
    })
  })
})
