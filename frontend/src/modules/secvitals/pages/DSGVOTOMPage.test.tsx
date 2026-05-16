import { describe, it, expect, vi, beforeEach } from 'vitest'
import { render, screen, waitFor } from '@testing-library/react'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { MemoryRouter } from 'react-router-dom'
import DSGVOTOMPage from './DSGVOTOMPage'

vi.mock('../hooks/useDSGVOMapping', () => ({
  useDSGVOTOMCoverage: vi.fn(() => ({
    data: [
      {
        tisax_control_id: 'TOM-1',
        tisax_control_title: 'Zutrittskontrolle',
        iso_control_id: 'A.9.1.2',
        iso_control_title: 'Netzwerkzugänge',
        covered: true,
      },
      {
        tisax_control_id: 'TOM-2',
        tisax_control_title: 'Zugangskontrolle',
        iso_control_id: 'A.9.4.2',
        iso_control_title: 'Sichere Anmeldeverfahren',
        covered: false,
      },
    ],
    isLoading: false,
  })),
}))

vi.mock('../hooks/useFrameworks', () => ({
  useFrameworks: vi.fn(() => ({
    data: [{ id: 'fw-dsgvo-1', name: 'DSGVO-TOM', version: '1.0' }],
  })),
}))

import { useDSGVOTOMCoverage } from '../hooks/useDSGVOMapping'
import { useFrameworks } from '../hooks/useFrameworks'

function renderPage() {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  })
  return render(
    <QueryClientProvider client={queryClient}>
      <MemoryRouter>
        <DSGVOTOMPage />
      </MemoryRouter>
    </QueryClientProvider>,
  )
}

describe('DSGVOTOMPage', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    vi.mocked(useFrameworks).mockReturnValue({
      data: [{ id: 'fw-dsgvo-1', name: 'DSGVO-TOM', version: '1.0', created_at: '' }],
    } as ReturnType<typeof useFrameworks>)
    vi.mocked(useDSGVOTOMCoverage).mockReturnValue({
      data: [
        {
          tisax_control_id: 'TOM-1',
          tisax_control_title: 'Zutrittskontrolle',
          iso_control_id: 'A.9.1.2',
          iso_control_title: 'Netzwerkzugänge',
          covered: true,
        },
        {
          tisax_control_id: 'TOM-2',
          tisax_control_title: 'Zugangskontrolle',
          iso_control_id: 'A.9.4.2',
          iso_control_title: 'Sichere Anmeldeverfahren',
          covered: false,
        },
      ],
      isLoading: false,
    } as ReturnType<typeof useDSGVOTOMCoverage>)
  })

  it('renders KPI total = 2, covered = 1, open = 1', async () => {
    renderPage()

    await waitFor(() => {
      const totalCard = screen.getByTestId('kpi-total')
      const coveredCard = screen.getByTestId('kpi-covered')
      const openCard = screen.getByTestId('kpi-open')

      expect(totalCard.textContent).toContain('2')
      expect(coveredCard.textContent).toContain('1')
      expect(openCard.textContent).toContain('1')
    })
  })

  it('renders TOM rows with correct data-testid attributes', async () => {
    renderPage()

    await waitFor(() => {
      const list = screen.getByTestId('tom-list')
      expect(list).toBeInTheDocument()

      const row1 = screen.getByTestId('tom-row-TOM-1')
      const row2 = screen.getByTestId('tom-row-TOM-2')
      expect(row1).toBeInTheDocument()
      expect(row2).toBeInTheDocument()
    })
  })

  it('covered TOM shows green "Abgedeckt" badge', async () => {
    renderPage()

    await waitFor(() => {
      const row1 = screen.getByTestId('tom-row-TOM-1')
      expect(row1.textContent).toContain('Abgedeckt')
    })
  })

  it('shows "nicht aktiviert" message when no DSGVO-TOM framework found', async () => {
    vi.mocked(useFrameworks).mockReturnValue({
      data: [],
    } as unknown as ReturnType<typeof useFrameworks>)

    renderPage()

    await waitFor(() => {
      expect(screen.getByText(/nicht aktiviert/)).toBeInTheDocument()
    })
  })
})
