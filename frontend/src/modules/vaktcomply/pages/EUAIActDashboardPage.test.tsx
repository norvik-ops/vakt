import { describe, it, expect, vi } from 'vitest'
import { render, screen, within } from '@testing-library/react'
import { QueryClient, QueryClientProvider, useQuery } from '@tanstack/react-query'
import { MemoryRouter } from 'react-router-dom'
import EUAIActDashboardPage from './EUAIActDashboardPage'

vi.mock('../../../shared/stores/auth', () => ({
  useAuthStore: (selector: (s: { token: string | null }) => unknown) =>
    selector({ token: 'test-token' }),
}))

const { QUERY_RESULT } = vi.hoisted(() => ({
  QUERY_RESULT: {
      data: {
        total_systems: 5,
        systems_by_risk_class: { high: 2, limited: 1, minimal: 2, unacceptable: 0 },
        systems_by_status: { under_review: 3, approved: 2 },
        systems_without_documentation: 1,
        systems_incomplete_documentation: 3,
        high_risk_deadline: '2027-12-02T00:00:00Z',
        high_risk_deadline_days_left: 381,
        iso27001_mappings: [
          {
            eu_ai_act_article: 'Art. 9',
            eu_ai_act_topic: 'Risikomanagement',
            iso27001_control: '6.1.2',
            iso27001_title: 'Informationssicherheits-Risikobeurteilung',
          },
        ],
      },
      isLoading: false,
      isError: false,
  },
}))

vi.mock('@tanstack/react-query', async (importOriginal) => {
  const actual = await importOriginal<typeof import('@tanstack/react-query')>()
  return {
    ...actual,
    useQuery: vi.fn().mockReturnValue(QUERY_RESULT),
  }
})

function wrapper({ children }: { children: React.ReactNode }) {
  const qc = new QueryClient({ defaultOptions: { queries: { retry: false } } })
  return (
    <MemoryRouter>
      <QueryClientProvider client={qc}>{children}</QueryClientProvider>
    </MemoryRouter>
  )
}

describe('EUAIActDashboardPage', () => {
  it('renders total systems KPI card', () => {
    render(<EUAIActDashboardPage />, { wrapper })
    const card = screen.getByTestId('kpi-total-systems')
    expect(card).toBeTruthy()
    expect(within(card).getByText('5')).toBeTruthy()
  })

  // UX-35: „Ohne Dossier: 0", obwohl das Dossier ein Entwurf mit sieben leeren
  // Feldern war. Die Kachel zählt unvollständige Dossiers, „ohne" steht darunter.
  it('zählt unvollständige Dossiers und nennt die ohne Dossier darunter', () => {
    render(<EUAIActDashboardPage />, { wrapper })
    const card = screen.getByTestId('kpi-without-docs')
    expect(within(card).getByText('Dossier unvollständig')).toBeTruthy()
    expect(within(card).getByText('3')).toBeTruthy()
    expect(within(card).getByText('davon ohne Dossier: 1')).toBeTruthy()
    expect(within(card).queryByText('Ohne Dossier')).toBeNull()
    expect(card.getAttribute('title')).toMatch(/Entwurf/)
  })

  it('renders high risk count KPI card', () => {
    render(<EUAIActDashboardPage />, { wrapper })
    const card = screen.getByTestId('kpi-high-risk-count')
    expect(card).toBeTruthy()
    expect(within(card).getByText('2')).toBeTruthy()
  })

  it('renders deadline KPI card', () => {
    vi.useFakeTimers({ toFake: ['Date'] })
    vi.setSystemTime(new Date('2026-11-16T12:00:00Z'))
    try {
      render(<EUAIActDashboardPage />, { wrapper })
      expect(screen.getByTestId('kpi-deadline')).toBeTruthy()
      expect(screen.getByText('381 Tage')).toBeTruthy()
    } finally {
      vi.useRealTimers()
    }
  })

  it('shows "gilt seit" instead of a 0-day countdown once the deadline has passed', () => {
    const mocked = vi.mocked(useQuery)
    mocked.mockReturnValue({
      ...QUERY_RESULT,
      data: { ...QUERY_RESULT.data, high_risk_deadline_days_left: 0 },
    } as unknown as ReturnType<typeof useQuery>)
    vi.useFakeTimers({ toFake: ['Date'] })
    vi.setSystemTime(new Date('2028-01-15T12:00:00Z'))
    try {
      render(<EUAIActDashboardPage />, { wrapper })
      const card = screen.getByTestId('kpi-deadline')
      expect(within(card).getByText(/gilt seit/)).toBeTruthy()
      expect(within(card).queryByText(/Tage/)).toBeNull()
    } finally {
      vi.useRealTimers()
      mocked.mockReturnValue(QUERY_RESULT as unknown as ReturnType<typeof useQuery>)
    }
  })

  it('treats the deadline day as a local date: "gilt seit" from local midnight on', () => {
    vi.useFakeTimers({ toFake: ['Date'] })
    // 00:30 Ortszeit am Fristtag — in DE noch vor UTC-Mitternacht.
    vi.setSystemTime(new Date(2027, 11, 2, 0, 30))
    try {
      render(<EUAIActDashboardPage />, { wrapper })
      expect(within(screen.getByTestId('kpi-deadline')).getByText(/gilt seit/)).toBeTruthy()
    } finally {
      vi.useRealTimers()
    }
  })

  it('renders risk class breakdown', () => {
    render(<EUAIActDashboardPage />, { wrapper })
    expect(screen.getByTestId('risk-class-breakdown')).toBeTruthy()
  })

  it('renders status breakdown', () => {
    render(<EUAIActDashboardPage />, { wrapper })
    expect(screen.getByTestId('status-breakdown')).toBeTruthy()
  })

  it('renders ISO 27001 mapping table', () => {
    render(<EUAIActDashboardPage />, { wrapper })
    expect(screen.getByTestId('iso-mapping-table')).toBeTruthy()
    expect(screen.getByText('Art. 9')).toBeTruthy()
    expect(screen.getByText('6.1.2')).toBeTruthy()
  })

  it('renders export PDF button', () => {
    render(<EUAIActDashboardPage />, { wrapper })
    expect(screen.getByTestId('export-report-pdf-btn')).toBeTruthy()
  })
})
