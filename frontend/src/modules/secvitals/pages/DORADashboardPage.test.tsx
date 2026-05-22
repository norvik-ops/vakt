import { describe, it, expect, vi } from 'vitest'
import { render, screen } from '@testing-library/react'
import { MemoryRouter } from 'react-router-dom'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import DORADashboardPage from './DORADashboardPage'
import type { DORADashboard } from '../types'

// ── mocks ─────────────────────────────────────────────────────────────────────

vi.mock('../hooks/useDORADashboard', () => ({
  useDORADashboard: vi.fn(),
}))

vi.mock('../../../shared/stores/auth', () => ({
  useAuthStore: (selector: (s: { token: string | null }) => unknown) =>
    selector({ token: 'test-token' }),
}))

import { useDORADashboard } from '../hooks/useDORADashboard'

function makeDashboard(overrides: Partial<DORADashboard> = {}): DORADashboard {
  return {
    readiness_pct: 75,
    open_critical_controls: 2,
    next_deadline: undefined,
    expired_suppliers: 0,
    tlpt_overdue_warning: false,
    third_party_count: 0,
    critical_third_parties: 0,
    missing_exit_strategies: 0,
    ...overrides,
  }
}

function renderPage() {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } })
  return render(
    <QueryClientProvider client={client}>
      <MemoryRouter>
        <DORADashboardPage />
      </MemoryRouter>
    </QueryClientProvider>,
  )
}

// ── tests ──────────────────────────────────────────────────────────────────────

describe('DORADashboardPage', () => {
  it('Case a: readiness_pct 45 → readiness tile has red color class', () => {
    vi.mocked(useDORADashboard).mockReturnValue({
      data: { data: makeDashboard({ readiness_pct: 45 }), notEnabled: false },
      isLoading: false,
      isError: false,
    } as ReturnType<typeof useDORADashboard>)

    renderPage()
    const readinessValue = screen.getByTestId('readiness-value')
    expect(readinessValue).toBeTruthy()
    expect(readinessValue.className).toContain('text-red-500')
    expect(readinessValue.textContent).toBe('45%')
  })

  it('Case b: next_deadline with deadline_type "4h" → "4h" visible in DOM', () => {
    vi.mocked(useDORADashboard).mockReturnValue({
      data: {
        data: makeDashboard({
          next_deadline: {
            incident_id: 'inc-1',
            title: 'Test DORA Incident',
            deadline_type: '4h',
            deadline_at: new Date(Date.now() + 2 * 60 * 60 * 1000).toISOString(),
          },
        }),
        notEnabled: false,
      },
      isLoading: false,
      isError: false,
    } as ReturnType<typeof useDORADashboard>)

    renderPage()
    const badge = screen.getByTestId('deadline-type-badge')
    expect(badge).toBeTruthy()
    expect(badge.textContent).toBe('4h')
  })

  it('Case c: tlpt_overdue_warning true → warning badge visible', () => {
    vi.mocked(useDORADashboard).mockReturnValue({
      data: { data: makeDashboard({ tlpt_overdue_warning: true }), notEnabled: false },
      isLoading: false,
      isError: false,
    } as ReturnType<typeof useDORADashboard>)

    renderPage()
    const badge = screen.getByTestId('tlpt-warning-badge')
    expect(badge).toBeTruthy()
  })

  it('notEnabled: shows "DORA ist noch nicht aktiviert" banner', () => {
    vi.mocked(useDORADashboard).mockReturnValue({
      data: { data: null, notEnabled: true },
      isLoading: false,
      isError: false,
    } as ReturnType<typeof useDORADashboard>)

    renderPage()
    expect(screen.getByTestId('dora-not-enabled-banner')).toBeTruthy()
    expect(screen.getByText('DORA ist noch nicht aktiviert')).toBeTruthy()
  })

  it('readiness 80+ → green color class', () => {
    vi.mocked(useDORADashboard).mockReturnValue({
      data: { data: makeDashboard({ readiness_pct: 85 }), notEnabled: false },
      isLoading: false,
      isError: false,
    } as ReturnType<typeof useDORADashboard>)

    renderPage()
    const readinessValue = screen.getByTestId('readiness-value')
    expect(readinessValue.className).toContain('text-green-500')
  })

  it('readiness 50–79 → yellow color class', () => {
    vi.mocked(useDORADashboard).mockReturnValue({
      data: { data: makeDashboard({ readiness_pct: 65 }), notEnabled: false },
      isLoading: false,
      isError: false,
    } as ReturnType<typeof useDORADashboard>)

    renderPage()
    const readinessValue = screen.getByTestId('readiness-value')
    expect(readinessValue.className).toContain('text-yellow-500')
  })

  it('shows loading spinner when isLoading is true', () => {
    vi.mocked(useDORADashboard).mockReturnValue({
      data: undefined,
      isLoading: true,
      isError: false,
    } as ReturnType<typeof useDORADashboard>)

    const { container } = renderPage()
    expect(container.querySelector('.animate-spin')).toBeTruthy()
  })
})
