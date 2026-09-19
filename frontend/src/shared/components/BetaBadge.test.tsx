import { render, screen } from '@testing-library/react'
import { describe, it, expect, vi } from 'vitest'
import { ModuleBetaBadge, ModuleBetaNotice } from './BetaBadge'

vi.mock('react-i18next', () => ({
  useTranslation: () => ({ t: (k: string) => k }),
}))

describe('ModuleBetaBadge / ModuleBetaNotice', () => {
  it('labels a single module as beta with an explaining tooltip', () => {
    render(<ModuleBetaBadge />)
    const badge = screen.getByTestId('module-beta-badge')
    expect(badge.textContent).toBe('beta.module')
    expect(badge.getAttribute('title')).toBe('beta.moduleTooltip')
  })

  it('shows the module-specific notice next to the badge', () => {
    render(<ModuleBetaNotice messageKey="beta.hrNotice" />)
    const notice = screen.getByTestId('module-beta-notice')
    expect(notice.getAttribute('role')).toBe('note')
    expect(notice.textContent).toContain('beta.hrNotice')
    expect(notice.textContent).toContain('beta.module')
  })
})
