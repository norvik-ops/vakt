import { describe, it, expect, vi } from 'vitest'
import { render, screen, fireEvent, waitFor } from '@testing-library/react'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { MemoryRouter } from 'react-router-dom'
import { ReportabilityWizard } from './ReportabilityWizard'

vi.mock('../../../shared/stores/auth', () => ({
  useAuthStore: (selector: (s: { token: string | null }) => unknown) =>
    selector({ token: 'test-token' }),
}))

const mockMutate = vi.fn()

vi.mock('../hooks/useIncidents', () => ({
  useAssessReportability: () => ({ mutate: mockMutate, isPending: false }),
}))

function wrapper({ children }: { children: React.ReactNode }) {
  const qc = new QueryClient({ defaultOptions: { queries: { retry: false } } })
  return (
    <MemoryRouter>
      <QueryClientProvider client={qc}>{children}</QueryClientProvider>
    </MemoryRouter>
  )
}

describe('ReportabilityWizard', () => {
  it('renders first question when open', () => {
    render(
      <ReportabilityWizard incidentId="inc-1" open={true} onClose={vi.fn()} />,
      { wrapper },
    )
    expect(screen.getByTestId('reportability-question')).toBeTruthy()
    expect(screen.getByText(/Frage 1 von 3/)).toBeTruthy()
  })

  it('advances to next question on Yes click', async () => {
    render(
      <ReportabilityWizard incidentId="inc-1" open={true} onClose={vi.fn()} />,
      { wrapper },
    )
    fireEvent.click(screen.getByTestId('reportability-yes-btn'))
    await waitFor(() => {
      expect(screen.getByText(/Frage 2 von 3/)).toBeTruthy()
    })
  })

  it('calls mutate after answering all questions', async () => {
    render(
      <ReportabilityWizard incidentId="inc-1" open={true} onClose={vi.fn()} />,
      { wrapper },
    )
    fireEvent.click(screen.getByTestId('reportability-no-btn'))
    await waitFor(() => { expect(screen.getByText(/Frage 2 von 3/)).toBeTruthy(); })
    fireEvent.click(screen.getByTestId('reportability-no-btn'))
    await waitFor(() => { expect(screen.getByText(/Frage 3 von 3/)).toBeTruthy(); })
    fireEvent.click(screen.getByTestId('reportability-no-btn'))
    await waitFor(() => {
      expect(mockMutate).toHaveBeenCalledWith(
        { affects_external_data: false, affects_essential_service: false, personal_data_compromised: false },
        expect.any(Object),
      )
    })
  })

  it('does not render when closed', () => {
    render(
      <ReportabilityWizard incidentId="inc-1" open={false} onClose={vi.fn()} />,
      { wrapper },
    )
    expect(screen.queryByTestId('reportability-question')).toBeNull()
  })
})
