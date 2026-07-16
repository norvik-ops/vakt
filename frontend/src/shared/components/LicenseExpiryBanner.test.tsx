import { describe, it, expect, vi, beforeEach } from 'vitest'
import { screen, fireEvent } from '@testing-library/react'
import { renderWithProviders } from '../../test-utils'
import { LicenseExpiryBanner } from './LicenseExpiryBanner'
import { apiFetch } from '../../api/client'

vi.mock('../../api/client', async (importOriginal) => {
  const actual = await importOriginal<typeof import('../../api/client')>()
  return { ...actual, apiFetch: vi.fn() }
})

// Role names are case-sensitive and capitalised across the platform
// ('Admin', 'SecurityAnalyst', 'Viewer', 'AuditorReadOnly' — see auth.RequireRole).
// The mock previously used lowercase 'admin', so LicenseExpiryBanner's
// `roles.includes('Admin')` guard was always false and the banner never
// rendered: the two "renders nothing" cases passed vacuously and the two
// banner cases failed.
vi.mock('../stores/auth', () => ({
  useAuthStore: vi.fn(() => ({
    user: { id: 'u1', email: 'admin@test.com', roles: ['Admin'] },
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
      auto_renewal_enabled: false, renewal_failing: false,
    })

    const { container } = renderWithProviders(<LicenseExpiryBanner />)
    // Wait for query to resolve
    await new Promise((r) => { setTimeout(r, 50); })
    expect(container).toBeEmptyDOMElement()
  })

  it('renders nothing while auto-renewal is armed AND working', async () => {
    vi.mocked(apiFetch).mockResolvedValue({
      tier: 'pro', is_pro: true, features: ['audit_pdf'],
      org_name: 'Test', expires_at: makeExpiry(5), demo: false,
      auto_renewal_enabled: true, renewal_failing: false,
    })

    const { container } = renderWithProviders(<LicenseExpiryBanner />)
    await new Promise((r) => { setTimeout(r, 50); })
    expect(container).toBeEmptyDOMElement()
  })

  // The bug this guards: the banner used to suppress on auto_renewal_enabled alone.
  // Renewal can be refused (open invoice, cancelled seat) and then, per
  // autorefresh.go, "nothing happens: the current key simply runs out" — so the one
  // case that truly needed a warning was the one that stayed silent.
  it('DOES warn when auto-renewal is armed but failing', async () => {
    vi.mocked(apiFetch).mockResolvedValue({
      tier: 'pro', is_pro: true, features: ['audit_pdf'],
      org_name: 'Test', expires_at: makeExpiry(5), demo: false,
      auto_renewal_enabled: true, renewal_failing: true,
    })

    renderWithProviders(<LicenseExpiryBanner />)
    await new Promise((r) => { setTimeout(r, 50); })

    expect(screen.getByText(/automatische Verlängerung/i)).toBeInTheDocument()
    expect(screen.getByRole('link', { name: /verlängern/i })).toBeInTheDocument()
  })

  // Failing renewal must warn across the whole remaining window, not only in the
  // last 30 days — the admin needs the time to settle an invoice.
  it('warns on failing renewal even when expiry is far away', async () => {
    vi.mocked(apiFetch).mockResolvedValue({
      tier: 'pro', is_pro: true, features: ['audit_pdf'],
      org_name: 'Test', expires_at: makeExpiry(60), demo: false,
      auto_renewal_enabled: true, renewal_failing: true,
    })

    renderWithProviders(<LicenseExpiryBanner />)
    await new Promise((r) => { setTimeout(r, 50); })
    expect(screen.getByText(/automatische Verlängerung/i)).toBeInTheDocument()
  })

  it('shows amber warning banner when expiry is 8–30 days away', async () => {
    vi.mocked(apiFetch).mockResolvedValue({
      tier: 'pro', is_pro: true, features: ['audit_pdf'],
      org_name: 'Test', expires_at: makeExpiry(15), demo: false,
      auto_renewal_enabled: false, renewal_failing: false,
    })

    renderWithProviders(<LicenseExpiryBanner />)
    await new Promise((r) => { setTimeout(r, 50); })

    expect(screen.getByRole('link', { name: /verlängern/i })).toBeInTheDocument()
  })

  it('shows red urgent banner when expiry is ≤ 7 days away and dismisses on click', async () => {
    vi.mocked(apiFetch).mockResolvedValue({
      tier: 'pro', is_pro: true, features: ['audit_pdf'],
      org_name: 'Test', expires_at: makeExpiry(3), demo: false,
      auto_renewal_enabled: false, renewal_failing: false,
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
