import { describe, it, expect, vi } from 'vitest'
import { render, screen, fireEvent, waitFor } from '@testing-library/react'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { AIClassificationWizard } from './AIClassificationWizard'

vi.mock('../../../shared/stores/auth', () => ({
  useAuthStore: (selector: (s: { token: string | null }) => unknown) =>
    selector({ token: 'test-token' }),
}))

const mockMutate = vi.fn()

vi.mock('../hooks/useAISystems', () => ({
  useAISystems: () => ({ data: [], isLoading: false, isError: false }),
  useCreateAISystem: () => ({ mutate: vi.fn(), isPending: false }),
  useUpdateAISystem: () => ({ mutate: vi.fn(), isPending: false }),
  useDeleteAISystem: () => ({ mutate: vi.fn(), isPending: false }),
  useAIClassifications: () => ({ data: [], isLoading: false }),
  useClassifyAISystem: () => ({ mutate: mockMutate, isPending: false }),
}))

function wrapper({ children }: { children: React.ReactNode }) {
  const qc = new QueryClient({ defaultOptions: { queries: { retry: false } } })
  return <QueryClientProvider client={qc}>{children}</QueryClientProvider>
}

describe('AIClassificationWizard', () => {
  it('renders first wizard question', () => {
    render(
      <AIClassificationWizard
        systemId="sys-1"
        systemName="Test KI"
        open={true}
        onClose={vi.fn()}
      />,
      { wrapper },
    )
    const matches = screen.getAllByText(/Art. 5 EU AI Act/)
    expect(matches.length).toBeGreaterThan(0)
    expect(screen.getByTestId('wizard-yes-btn')).toBeTruthy()
    expect(screen.getByTestId('wizard-no-btn')).toBeTruthy()
  })

  it('answering "Yes" to prohibited leads to unacceptable result', async () => {
    render(
      <AIClassificationWizard
        systemId="sys-1"
        systemName="Test KI"
        open={true}
        onClose={vi.fn()}
      />,
      { wrapper },
    )
    fireEvent.click(screen.getByTestId('wizard-yes-btn'))
    await waitFor(() => {
      expect(screen.getByTestId('wizard-result')).toBeTruthy()
    })
    expect(screen.getByText(/Inakzeptables Risiko/)).toBeTruthy()
  })

  it('answering "No" three times leads to minimal risk result', async () => {
    render(
      <AIClassificationWizard
        systemId="sys-1"
        systemName="Test KI"
        open={true}
        onClose={vi.fn()}
      />,
      { wrapper },
    )
    // Step 1: step_prohibited → "No" → step_high_risk
    fireEvent.click(screen.getByTestId('wizard-no-btn'))
    // Step 2: step_high_risk → "No" → step_transparency
    await waitFor(() => { expect(screen.getByTestId('wizard-no-btn')).toBeTruthy(); })
    fireEvent.click(screen.getByTestId('wizard-no-btn'))
    // Step 3: step_transparency → "No" → minimal result
    await waitFor(() => { expect(screen.getByTestId('wizard-no-btn')).toBeTruthy(); })
    fireEvent.click(screen.getByTestId('wizard-no-btn'))
    await waitFor(() => {
      expect(screen.getByTestId('wizard-result')).toBeTruthy()
    })
    expect(screen.getByText(/Minimales Risiko/)).toBeTruthy()
  })

  it('save button calls classify mutate', async () => {
    render(
      <AIClassificationWizard
        systemId="sys-1"
        systemName="Test KI"
        open={true}
        onClose={vi.fn()}
      />,
      { wrapper },
    )
    fireEvent.click(screen.getByTestId('wizard-yes-btn'))
    await waitFor(() => {
      expect(screen.getByTestId('wizard-save-btn')).toBeTruthy()
    })
    fireEvent.click(screen.getByTestId('wizard-save-btn'))
    expect(mockMutate).toHaveBeenCalledWith(
      expect.objectContaining({ risk_class: 'unacceptable' }),
      expect.any(Object),
    )
  })

  // UX-36: Der zweizeilige Titel lief über den Fortschrittsbalken (leading-none +
  // negativer Abstand), und „Ja" war bei der Verbotsfrage der blaue Hauptknopf.
  it('UX-36: Titel darf umbrechen ohne Überlappung, Ja und Nein gleich gewichtet', () => {
    render(
      <AIClassificationWizard
        systemId="sys-1"
        systemName="Bewerber-Vorauswahl mit einem sehr langen Namen KW10"
        open={true}
        onClose={vi.fn()}
      />,
      { wrapper },
    )
    const title = screen.getByTestId('wizard-title')
    expect(title.className).not.toMatch(/leading-none/)
    expect(title.className).toMatch(/leading-snug/)
    expect(screen.getByTestId('wizard-progress').className).not.toMatch(/-mt-/)
    const yes = screen.getByTestId('wizard-yes-btn')
    const no = screen.getByTestId('wizard-no-btn')
    expect(yes.className).toBe(no.className)
  })
})
