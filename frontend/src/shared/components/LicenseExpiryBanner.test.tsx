import { describe, it, expect, vi, beforeEach } from 'vitest'
import { screen, fireEvent } from '@testing-library/react'
import { renderWithProviders } from '../../test-utils'
import { LicenseExpiryBanner } from './LicenseExpiryBanner'
import { apiFetch } from '../../api/client'

vi.mock('../../api/client', async (importOriginal) => {
  const actual = await importOriginal<typeof import('../../api/client')>()
  return { ...actual, apiFetch: vi.fn() }
})

vi.mock('../stores/auth', () => ({
  useAuthStore: vi.fn(() => ({
    user: { id: 'u1', email: 'admin@test.com', roles: ['admin'] },
  })),
}))

beforeEach(() => {
  localStorage.clear()
  vi.clearAllMocks()
})

function makeExpiry(daysFromNow: number): string {
  const d = new Date()
  d.setDate(d.getDate() + daysFromNow)
  return d.toISOString()
}

describe('LicenseExpiryBanner', () => {
  it('renders nothing when license is Community (not pro)', async () => {
    vi.mocked(apiFetch).mockResolvedValue({
      tier: 'community', is_pro: false, features: [],
      org_name: 'Test', expires_at: null, demo: false,
      revoked: false, auto_renewal_enabled: false,
    })

    const { container } = renderWithProviders(<LicenseExpiryBanner />)
    // Wait for query to resolve
    await new Promise((r) => { setTimeout(r, 50); })
    expect(container).toBeEmptyDOMElement()
  })

  it('renders nothing when auto-renewal is enabled', async () => {
    vi.mocked(apiFetch).mockResolvedValue({
      tier: 'pro', is_pro: true, features: ['audit_pdf'],
      org_name: 'Test', expires_at: makeExpiry(5), demo: false,
      revoked: false, auto_renewal_enabled: true,
    })

    const { container } = renderWithProviders(<LicenseExpiryBanner />)
    await new Promise((r) => { setTimeout(r, 50); })
    expect(container).toBeEmptyDOMElement()
  })

  it('shows amber warning banner when expiry is 8–30 days away', async () => {
    vi.mocked(apiFetch).mockResolvedValue({
      tier: 'pro', is_pro: true, features: ['audit_pdf'],
      org_name: 'Test', expires_at: makeExpiry(15), demo: false,
      revoked: false, auto_renewal_enabled: false,
    })

    renderWithProviders(<LicenseExpiryBanner />)
    await new Promise((r) => { setTimeout(r, 50); })

    expect(screen.getByRole('link', { name: /verlängern/i })).toBeInTheDocument()
  })

  it('shows red urgent banner when expiry is ≤ 7 days away and dismisses on click', async () => {
    vi.mocked(apiFetch).mockResolvedValue({
      tier: 'pro', is_pro: true, features: ['audit_pdf'],
      org_name: 'Test', expires_at: makeExpiry(3), demo: false,
      revoked: false, auto_renewal_enabled: false,
    })

    renderWithProviders(<LicenseExpiryBanner />)
    await new Promise((r) => { setTimeout(r, 50); })

    const dismissBtn = screen.getByRole('button', { name: /schließen/i })
    expect(dismissBtn).toBeInTheDocument()

    fireEvent.click(dismissBtn)
    // After dismiss, the button disappears
    expect(screen.queryByRole('button', { name: /schließen/i })).not.toBeInTheDocument()
  })
})
