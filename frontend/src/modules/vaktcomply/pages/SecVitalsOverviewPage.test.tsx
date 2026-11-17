import { describe, it, expect, vi } from 'vitest'
import { render, screen } from '@testing-library/react'
import { MemoryRouter } from 'react-router-dom'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import SecVitalsOverviewPage from './SecVitalsOverviewPage'
import type { Framework } from '../types'

// ── mocks ─────────────────────────────────────────────────────────────────────

const TISAX_FRAMEWORK: Framework = {
  id: 'fw-tisax-1',
  org_id: 'org-1',
  name: 'TISAX',
  is_builtin: true,
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
      <MemoryRouter initialEntries={['/vaktcomply']}>
        <SecVitalsOverviewPage />
      </MemoryRouter>
    </QueryClientProvider>,
  )
}

// ── tests ──────────────────────────────────────────────────────────────────────

describe('SecVitalsOverviewPage', () => {
  // UX-01: DORA ist nicht käuflich (features.UnsoldFeatures). Die Kachel unter
  // „Bereiche“ führte auch Pro-Kunden auf eine Sperrseite mit Kaufangebot.
  it('bietet keine DORA-Kachel an', () => {
    mockFrameworks = []
    renderPage()
    expect(screen.queryByRole('link', { name: /DORA Dashboard/i })).toBeNull()
    expect(document.querySelector('a[href="/vaktcomply/dora/dashboard"]')).toBeNull()
  })

  it('shows TISAX tile when TISAX framework is in the list', () => {
    mockFrameworks = [TISAX_FRAMEWORK]
    renderPage()
    const tile = screen.getByTestId('tisax-tile')
    expect(tile).toBeTruthy()
    expect(tile.getAttribute('href')).toBe(`/vaktcomply/frameworks/${TISAX_FRAMEWORK.id}/tisax`)
  })

  it('does not show TISAX tile when no TISAX framework is present', () => {
    mockFrameworks = []
    renderPage()
    const tile = screen.queryByTestId('tisax-tile')
    expect(tile).toBeNull()
  })
})
