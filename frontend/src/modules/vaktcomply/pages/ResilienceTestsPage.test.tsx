import { describe, it, expect, vi } from 'vitest'
import { render, screen } from '@testing-library/react'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import ResilienceTestsPage from './ResilienceTestsPage'
import type { ResilienceTestsResponse } from '../types'

const mockResponseWithOverdue: ResilienceTestsResponse = {
  tlpt_overdue_warning: true,
  tests: [
    {
      id: 'rt-1',
      org_id: 'org-1',
      type: 'tlpt',
      scope: 'Core banking',
      provider: 'RedTeam GmbH',
      test_date: '2020-01-15T00:00:00Z',
      summary: 'TLPT exercise',
      remediation_status: 'completed',
      overdue_warning: true,
      created_at: '2020-01-15T00:00:00Z',
      updated_at: '2020-01-15T00:00:00Z',
    },
    {
      id: 'rt-2',
      org_id: 'org-1',
      type: 'pentest',
      provider: 'SecTech',
      test_date: '2024-06-01T00:00:00Z',
      remediation_status: 'in_progress',
      created_at: '2024-06-01T00:00:00Z',
      updated_at: '2024-06-01T00:00:00Z',
    },
  ],
}

const mockResponseNoOverdue: ResilienceTestsResponse = {
  tlpt_overdue_warning: false,
  tests: [
    {
      id: 'rt-3',
      org_id: 'org-1',
      type: 'tlpt',
      test_date: '2024-03-01T00:00:00Z',
      remediation_status: 'open',
      created_at: '2024-03-01T00:00:00Z',
      updated_at: '2024-03-01T00:00:00Z',
    },
  ],
}

vi.mock('../hooks/useResilienceTests', () => ({
  useResilienceTests: vi.fn(),
  useCreateResilienceTest: () => ({ mutate: vi.fn(), isPending: false }),
  useUpdateResilienceTest: () => ({ mutate: vi.fn(), isPending: false }),
  useDeleteResilienceTest: () => ({ mutate: vi.fn(), isPending: false }),
  useUploadResilienceTestAttachment: () => ({ mutate: vi.fn(), isPending: false }),
  useLinkResilienceTestAsEvidence: () => ({ mutate: vi.fn(), isPending: false }),
}))

import { useResilienceTests } from '../hooks/useResilienceTests'

function renderPage() {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } })
  return render(
    <QueryClientProvider client={client}>
      <ResilienceTestsPage />
    </QueryClientProvider>,
  )
}

describe('ResilienceTestsPage', () => {
  it('renders tlpt-overdue-warning banner when tlpt_overdue_warning is true', () => {
    vi.mocked(useResilienceTests).mockReturnValue({
      data: mockResponseWithOverdue,
      isLoading: false,
      isError: false,
    } as ReturnType<typeof useResilienceTests>)

    renderPage()
    const banner = screen.getByTestId('tlpt-overdue-warning')
    expect(banner).toBeTruthy()
    expect(banner.textContent).toContain('DORA Art. 26')
  })

  it('does NOT render tlpt-overdue-warning banner when tlpt_overdue_warning is false', () => {
    vi.mocked(useResilienceTests).mockReturnValue({
      data: mockResponseNoOverdue,
      isLoading: false,
      isError: false,
    } as ReturnType<typeof useResilienceTests>)

    renderPage()
    expect(screen.queryByTestId('tlpt-overdue-warning')).toBeNull()
  })

  it('renders type badges for each test', () => {
    vi.mocked(useResilienceTests).mockReturnValue({
      data: mockResponseWithOverdue,
      isLoading: false,
      isError: false,
    } as ReturnType<typeof useResilienceTests>)

    renderPage()
    expect(screen.getByText('TLPT')).toBeTruthy()
    expect(screen.getByText('Pentest')).toBeTruthy()
  })

  it('renders Neuer Test button', () => {
    vi.mocked(useResilienceTests).mockReturnValue({
      data: mockResponseNoOverdue,
      isLoading: false,
      isError: false,
    } as ReturnType<typeof useResilienceTests>)

    renderPage()
    expect(screen.getByText('Neuer Test')).toBeTruthy()
  })

  it('shows loading spinner when isLoading is true', () => {
    vi.mocked(useResilienceTests).mockReturnValue({
      data: undefined,
      isLoading: true,
      isError: false,
    } as ReturnType<typeof useResilienceTests>)

    const { container } = renderPage()
    expect(container.querySelector('.animate-spin')).toBeTruthy()
  })
})
