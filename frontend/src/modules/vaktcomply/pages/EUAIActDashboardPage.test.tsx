import { describe, it, expect, vi } from 'vitest'
import { render, screen, within } from '@testing-library/react'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { MemoryRouter } from 'react-router-dom'
import EUAIActDashboardPage from './EUAIActDashboardPage'

vi.mock('../../../shared/stores/auth', () => ({
  useAuthStore: (selector: (s: { token: string | null }) => unknown) =>
    selector({ token: 'test-token' }),
}))

vi.mock('@tanstack/react-query', async (importOriginal) => {
  const actual = await importOriginal<typeof import('@tanstack/react-query')>()
  return {
    ...actual,
    useQuery: vi.fn().mockReturnValue({
      data: {
        total_systems: 5,
        systems_by_risk_class: { high: 2, limited: 1, minimal: 2, unacceptable: 0 },
        systems_by_status: { under_review: 3, approved: 2 },
        systems_without_documentation: 1,
        high_risk_deadline: '2026-08-02T00:00:00Z',
        high_risk_deadline_days_left: 445,
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
    }),
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

  it('renders without documentation KPI card', () => {
    render(<EUAIActDashboardPage />, { wrapper })
    expect(screen.getByTestId('kpi-without-docs')).toBeTruthy()
  })

  it('renders high risk count KPI card', () => {
    render(<EUAIActDashboardPage />, { wrapper })
    const card = screen.getByTestId('kpi-high-risk-count')
    expect(card).toBeTruthy()
    expect(within(card).getByText('2')).toBeTruthy()
  })

  it('renders deadline KPI card', () => {
    render(<EUAIActDashboardPage />, { wrapper })
    expect(screen.getByTestId('kpi-deadline')).toBeTruthy()
    expect(screen.getByText('445 Tage')).toBeTruthy()
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
