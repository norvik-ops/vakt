import { render, screen, fireEvent } from '@testing-library/react'
import { describe, it, expect, vi } from 'vitest'
import { MemoryRouter } from 'react-router-dom'
import RecoveryPlansPage from './RecoveryPlansPage'

vi.mock('react-i18next', () => ({
  useTranslation: () => ({ t: (k: string) => k }),
}))

const mockCreate = vi.fn()
const mockDelete = vi.fn()

vi.mock('../hooks/useRecoveryPlans', () => ({
  useRecoveryPlans: vi.fn(),
  useCreateRecoveryPlan: () => ({ mutate: mockCreate, isPending: false }),
  useUpdateRecoveryPlan: () => ({ mutate: vi.fn(), isPending: false }),
  useDeleteRecoveryPlan: () => ({ mutate: mockDelete }),
}))

vi.mock('../hooks/useBIA', () => ({
  useBIAProcesses: () => ({ data: [] }),
}))

const { useRecoveryPlans } = await import('../hooks/useRecoveryPlans')
const mockUseRecoveryPlans = vi.mocked(useRecoveryPlans)

function wrapper({ children }: { children: React.ReactNode }) {
  return <MemoryRouter>{children}</MemoryRouter>
}

describe('RecoveryPlansPage', () => {
  it('shows empty state when no plans', () => {
    mockUseRecoveryPlans.mockReturnValue({ data: [], isLoading: false, isError: false } as unknown as ReturnType<typeof useRecoveryPlans>)
    render(<RecoveryPlansPage />, { wrapper })
    expect(screen.getByText('bcm.recoveryPlans.emptyTitle')).toBeTruthy()
  })

  it('shows error state on failure', () => {
    mockUseRecoveryPlans.mockReturnValue({ data: undefined, isLoading: false, isError: true } as unknown as ReturnType<typeof useRecoveryPlans>)
    render(<RecoveryPlansPage />, { wrapper })
    expect(screen.getByText('bcm.recoveryPlans.loadError')).toBeTruthy()
  })

  it('renders plan card when data is available', () => {
    mockUseRecoveryPlans.mockReturnValue({
      data: [
        {
          id: '1',
          org_id: 'org1',
          bia_process_id: null,
          bia_process_name: '',
          title: 'WAP IT-Infrastruktur',
          activation_criteria: 'Totalausfall RZ',
          responsible: 'IT-Leiter',
          rto_hours: 4,
          status: 'tested' as const,
          steps: [
            { order: 1, action: 'Notfallteam alarmieren', responsible: 'ISB', duration_min: 15 },
          ],
          last_tested_at: '2026-03-01',
          created_at: '2026-01-01T00:00:00Z',
          updated_at: '2026-01-01T00:00:00Z',
        },
      ],
      isLoading: false,
      isError: false,
    } as unknown as ReturnType<typeof useRecoveryPlans>)
    render(<RecoveryPlansPage />, { wrapper })
    expect(screen.getByText('WAP IT-Infrastruktur')).toBeTruthy()
    expect(screen.getByText('RTO: 4h')).toBeTruthy()
    expect(screen.getByText('Totalausfall RZ')).toBeTruthy()
  })

  it('shows steps count when steps available', () => {
    mockUseRecoveryPlans.mockReturnValue({
      data: [
        {
          id: '1',
          org_id: 'org1',
          bia_process_id: null,
          bia_process_name: '',
          title: 'WAP Test',
          activation_criteria: '',
          responsible: '',
          rto_hours: 2,
          status: 'draft' as const,
          steps: [
            { order: 1, action: 'Schritt A', responsible: 'A', duration_min: 10 },
            { order: 2, action: 'Schritt B', responsible: 'B', duration_min: 20 },
          ],
          last_tested_at: null,
          created_at: '2026-01-01T00:00:00Z',
          updated_at: '2026-01-01T00:00:00Z',
        },
      ],
      isLoading: false,
      isError: false,
    } as unknown as ReturnType<typeof useRecoveryPlans>)
    render(<RecoveryPlansPage />, { wrapper })
    expect(screen.getByText('2 Schritte')).toBeTruthy()
  })

  it('opens create dialog on button click', () => {
    mockUseRecoveryPlans.mockReturnValue({ data: [], isLoading: false, isError: false } as unknown as ReturnType<typeof useRecoveryPlans>)
    render(<RecoveryPlansPage />, { wrapper })
    const addButton = screen.getAllByText('bcm.recoveryPlans.new')[0]
    fireEvent.click(addButton)
    expect(screen.getByRole('dialog')).toBeTruthy()
  })
})
