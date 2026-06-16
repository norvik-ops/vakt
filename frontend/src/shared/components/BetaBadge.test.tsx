import { render, screen } from '@testing-library/react'
import { describe, it, expect, vi } from 'vitest'
import { BetaBadge } from './BetaBadge'

vi.mock('react-i18next', () => ({
  useTranslation: () => ({ t: (k: string) => k }),
}))

describe('BetaBadge', () => {
  it('renders the Private Beta label and links to the disclaimer', () => {
    render(<BetaBadge />)
    const link = screen.getByTestId('beta-badge')
    expect(link.textContent).toContain('beta.badge')
    expect(link.getAttribute('href')).toContain('beta-disclaimer.md')
    expect(link.getAttribute('title')).toBe('beta.tooltip')
    expect(link.getAttribute('rel')).toContain('noopener')
  })

  it('renders a discreet dot variant when collapsed (still linked)', () => {
    render(<BetaBadge collapsed />)
    const link = screen.getByTestId('beta-badge')
    expect(link.getAttribute('href')).toContain('beta-disclaimer.md')
    expect(link.getAttribute('aria-label')).toBe('beta.badge')
  })
})
