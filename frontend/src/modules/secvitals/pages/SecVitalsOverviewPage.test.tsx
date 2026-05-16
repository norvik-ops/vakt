import { describe, it, expect, vi } from 'vitest'
import { render, screen } from '@testing-library/react'
import { MemoryRouter } from 'react-router-dom'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import SecVitalsOverviewPage from './SecVitalsOverviewPage'
import type { Framework } from '../types'

// ── mocks ─────────────────────────────────────────────────────────────────────

const TISAX_FRAMEWORK: Framework = {
  id: 'fw-tisax-1',
  name: 'TISAX',
  version: '5.1',
  created_at: '2026-01-01T00:00:00Z',
}

// Default mock: no frameworks
let mockFrameworks: Framework[] = []

vi.mock('../hooks/useFrameworks', () => ({
  useFrameworks: () => ({ data: mockFrameworks }),
  useTISAXReport: () => ({ data: { tisax_maturity: { readiness_percent: 65.0 } } }),
}))

vi.mock('../hooks/useRisks', () => ({
  useRisks: () => ({ data: [] }),
}))

vi.mock('../hooks/useIncidents', () => ({
  useIncidents: () => ({ data: [] }),
}))

vi.mock('../hooks/usePolicies', () => ({
  usePolicies: () => ({ data: [] }),
}))

vi.mock('../hooks/useAudits', () => ({
  useAuditRecords: () => ({ data: [] }),
}))

vi.mock('../hooks/useDORADashboard', () => ({
  useDORADashboard: () => ({
    data: { data: { readiness_pct: 72, open_critical_controls: 1, expired_suppliers: 0, tlpt_overdue_warning: false }, notEnabled: false },
    isLoading: false,
  }),
}))

vi.mock('../components/ExpiringEvidenceWidget', () => ({
  ExpiringEvidenceWidget: () => null,
}))

function renderPage() {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } })
  return render(
    <QueryClientProvider client={client}>
      <MemoryRouter initialEntries={['/secvitals']}>
        <SecVitalsOverviewPage />
      </MemoryRouter>
    </QueryClientProvider>,
  )
}

// ── tests ──────────────────────────────────────────────────────────────────────

describe('SecVitalsOverviewPage', () => {
  it('renders DORA Dashboard link with correct href and text', () => {
    mockFrameworks = []
    renderPage()
    const link = screen.getByRole('link', { name: /DORA Dashboard/i })
    expect(link).toBeTruthy()
    expect(link.getAttribute('href')).toBe('/secvitals/dora/dashboard')
  })

  it('shows TISAX tile when TISAX framework is in the list', () => {
    mockFrameworks = [TISAX_FRAMEWORK]
    renderPage()
    const tile = screen.getByTestId('tisax-tile')
    expect(tile).toBeTruthy()
    expect(tile.getAttribute('href')).toBe(`/secvitals/frameworks/${TISAX_FRAMEWORK.id}/tisax`)
  })

  it('does not show TISAX tile when no TISAX framework is present', () => {
    mockFrameworks = []
    renderPage()
    const tile = screen.queryByTestId('tisax-tile')
    expect(tile).toBeNull()
  })
})
