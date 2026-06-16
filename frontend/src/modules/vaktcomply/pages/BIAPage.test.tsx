import { render, screen, fireEvent } from '@testing-library/react'
import { describe, it, expect, vi } from 'vitest'
import { MemoryRouter } from 'react-router-dom'
import BIAPage from './BIAPage'

vi.mock('react-i18next', () => ({
  useTranslation: () => ({ t: (k: string) => k }),
}))

const mockCreate = vi.fn()
const mockUpdate = vi.fn()
const mockDelete = vi.fn()

vi.mock('../hooks/useBIA', () => ({
  useBIAProcesses: vi.fn(),
  useCreateBIAProcess: () => ({ mutate: mockCreate, isPending: false }),
  useUpdateBIAProcess: () => ({ mutate: mockUpdate, isPending: false }),
  useDeleteBIAProcess: () => ({ mutate: mockDelete }),
}))

const { useBIAProcesses } = await import('../hooks/useBIA')
const mockUseBIAProcesses = vi.mocked(useBIAProcesses)

function wrapper({ children }: { children: React.ReactNode }) {
  return <MemoryRouter>{children}</MemoryRouter>
}

describe('BIAPage', () => {
  it('shows spinner while loading', () => {
    mockUseBIAProcesses.mockReturnValue({ data: undefined, isLoading: true, isError: false } as unknown as ReturnType<typeof useBIAProcesses>)
    const { container } = render(<BIAPage />, { wrapper })
    // Spinner renders an svg or a visible indicator
    expect(container.querySelector('.animate-spin') !== null || container.innerHTML.includes('animate')).toBeTruthy()
  })

  it('shows empty state when no processes', () => {
    mockUseBIAProcesses.mockReturnValue({ data: [], isLoading: false, isError: false } as unknown as ReturnType<typeof useBIAProcesses>)
    render(<BIAPage />, { wrapper })
    expect(screen.getByText('bcm.bia.emptyTitle')).toBeTruthy()
  })

  it('shows error state on failure', () => {
    mockUseBIAProcesses.mockReturnValue({ data: undefined, isLoading: false, isError: true } as unknown as ReturnType<typeof useBIAProcesses>)
    render(<BIAPage />, { wrapper })
    expect(screen.getByText('bcm.bia.loadError')).toBeTruthy()
  })

  it('renders process cards when data is available', () => {
    mockUseBIAProcesses.mockReturnValue({
      data: [
        {
          id: '1',
          org_id: 'org1',
          name: 'IT-Infrastruktur-Betrieb',
          description: 'Kritischer Prozess',
          process_owner: 'IT-Leiter',
          criticality: 'high' as const,
          schutzbedarfsklasse: 3 as const,
          rto_hours: 4,
          rpo_hours: 1,
          mbco_percent: 80,
          dependencies: [],
          created_at: '2026-01-01T00:00:00Z',
          updated_at: '2026-01-01T00:00:00Z',
        },
      ],
      isLoading: false,
      isError: false,
    } as unknown as ReturnType<typeof useBIAProcesses>)
    render(<BIAPage />, { wrapper })
    expect(screen.getByText('IT-Infrastruktur-Betrieb')).toBeTruthy()
    expect(screen.getByText('4h')).toBeTruthy()
    expect(screen.getByText('1h')).toBeTruthy()
    expect(screen.getByText('80%')).toBeTruthy()
  })

  it('opens create dialog on button click', () => {
    mockUseBIAProcesses.mockReturnValue({ data: [], isLoading: false, isError: false } as unknown as ReturnType<typeof useBIAProcesses>)
    render(<BIAPage />, { wrapper })
    const addButton = screen.getAllByText('bcm.bia.new')[0]
    fireEvent.click(addButton)
    expect(screen.getByRole('dialog')).toBeTruthy()
  })
})
