import { describe, it, expect } from 'vitest'
import { render, screen } from '@testing-library/react'
import { AISystemStatusBadge } from './AISystemStatusBadge'

describe('AISystemStatusBadge', () => {
  it('under_review renders with gray styling', () => {
    render(<AISystemStatusBadge status="under_review" />)
    const badge = screen.getByTestId('ai-status-badge')
    expect(badge.className).toContain('bg-gray-100')
    expect(badge.textContent).toBe('In Prüfung')
  })

  it('classified renders with blue styling', () => {
    render(<AISystemStatusBadge status="classified" />)
    const badge = screen.getByTestId('ai-status-badge')
    expect(badge.className).toContain('bg-blue-100')
  })

  it('compliant renders with green styling', () => {
    render(<AISystemStatusBadge status="compliant" />)
    const badge = screen.getByTestId('ai-status-badge')
    expect(badge.className).toContain('bg-green-100')
  })

  it('decommissioned renders with red styling', () => {
    render(<AISystemStatusBadge status="decommissioned" />)
    const badge = screen.getByTestId('ai-status-badge')
    expect(badge.className).toContain('bg-red-100')
  })
})
