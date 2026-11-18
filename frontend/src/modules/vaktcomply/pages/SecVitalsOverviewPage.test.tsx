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

// UX-38: Risiken, Richtlinien und Umsetzungsgrad kommen aus denselben Quellen
// wie das Dashboard (GET /dashboard/aggregate und /dashboard/score).
let mockAggregate: Record<string, unknown> | undefined
let mockScore: Record<string, unknown> | undefined

vi.mock('../../../hooks/useDashboard', () => ({
  useDashboardAggregate: () => ({ data: mockAggregate }),
  useDashboardScore: () => ({ data: mockScore }),
}))

vi.mock('../hooks/useIncidents', () => ({
  useIncidents: () => ({ data: [] }),
}))

let mockPolicies: Array<Record<string, unknown>> = []

vi.mock('../hooks/usePolicies', () => ({
  usePolicies: () => ({ data: mockPolicies }),
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

// UX-38: Dashboard „Kritische Risiken 1" neben Übersicht „2 Offene Risiken ·
// 2 kritisch/hoch"; Übersicht „2 Richtlinien · alle aktuell", Dashboard
// „1 / 2 Richtlinien aktiv". Dieselbe Kennzahl, derselbe Name, derselbe Wert.
describe('SecVitalsOverviewPage — dieselben Zahlen wie das Dashboard (UX-38)', () => {
  it('Risiken, Richtlinien und Umsetzungsgrad aus Dashboard-Aggregat und -Score', () => {
    mockFrameworks = [{ ...TISAX_FRAMEWORK, id: 'a', name: 'ISO27001' }, { ...TISAX_FRAMEWORK, id: 'b', name: 'NIS2' }]
    mockAggregate = {
      open_risks: 2, critical_risks: 1, policies_approved: 1, policies_total: 2,
      framework_scores: [], overall_readiness: { covered: 1, partial: 10, applicable: 214, score_pct: (6 / 214) * 100 },
    }
    mockScore = { score: 3, has_data: true, readiness: { covered: 1, partial: 10, applicable: 214, score_pct: (6 / 214) * 100 } }
    mockPolicies = [{ id: 'p1', status: 'active' }, { id: 'p2', status: 'draft' }]
    renderPage()

    // Risiken: „Offene Risiken 2 · 1 kritisch" — kritisch wie die Dashboard-Kachel „Kritische Risiken".
    const risks = screen.getByRole('button', { name: /Offene Risiken/ })
    expect(risks).toHaveTextContent('2')
    expect(risks).toHaveTextContent('1 kritisch')
    expect(risks).not.toHaveTextContent('kritisch/hoch')

    // Richtlinien: aktiv / gesamt wie im Dashboard, kein „alle aktuell" bei einem Entwurf.
    const policies = screen.getByRole('button', { name: /Richtlinien aktiv/ })
    expect(policies).toHaveTextContent('1 / 2')
    expect(policies).not.toHaveTextContent('alle aktuell')

    // Frameworks: Umsetzungsgrad in derselben Zeile wie Dashboard und Framework-Seiten.
    const fws = screen.getByRole('button', { name: /Frameworks aktiv/ })
    expect(fws).toHaveTextContent('1 / 214 umgesetzt · 10 in Bearbeitung · 3%')
  })
})
