import { render, screen, fireEvent } from '@testing-library/react'
import { describe, it, expect, vi } from 'vitest'
import { MemoryRouter } from 'react-router-dom'
import EmergencyContactsPage from './EmergencyContactsPage'

vi.mock('react-i18next', () => ({
  useTranslation: () => ({ t: (k: string) => k }),
}))

const mockCreate = vi.fn()
const mockDelete = vi.fn()

vi.mock('../hooks/useEmergencyContacts', () => ({
  useEmergencyContacts: vi.fn(),
  useCreateEmergencyContact: () => ({ mutate: mockCreate, isPending: false }),
  useUpdateEmergencyContact: () => ({ mutate: vi.fn(), isPending: false }),
  useDeleteEmergencyContact: () => ({ mutate: mockDelete }),
}))

const { useEmergencyContacts } = await import('../hooks/useEmergencyContacts')
const mockUseEmergencyContacts = vi.mocked(useEmergencyContacts)

function wrapper({ children }: { children: React.ReactNode }) {
  return <MemoryRouter>{children}</MemoryRouter>
}

describe('EmergencyContactsPage', () => {
  it('shows empty state when no contacts', () => {
    mockUseEmergencyContacts.mockReturnValue({ data: [], isLoading: false, isError: false } as unknown as ReturnType<typeof useEmergencyContacts>)
    render(<EmergencyContactsPage />, { wrapper })
    expect(screen.getByText('bcm.emergencyContacts.emptyTitle')).toBeTruthy()
  })

  it('shows error state on failure', () => {
    mockUseEmergencyContacts.mockReturnValue({ data: undefined, isLoading: false, isError: true } as unknown as ReturnType<typeof useEmergencyContacts>)
    render(<EmergencyContactsPage />, { wrapper })
    expect(screen.getByText('bcm.emergencyContacts.loadError')).toBeTruthy()
  })

  it('renders contact names', () => {
    mockUseEmergencyContacts.mockReturnValue({
      data: [
        { id: '1', org_id: 'o', name: 'Max Mustermann', role: 'CISO', phone: '+49 123', email: 'max@example.com', escalation_level: 1 as const, available_247: true, notes: '', created_at: '', updated_at: '' },
        { id: '2', org_id: 'o', name: 'Erika Muster', role: 'ISB', phone: '', email: 'erika@example.com', escalation_level: 2 as const, available_247: false, notes: '', created_at: '', updated_at: '' },
      ],
      isLoading: false,
      isError: false,
    } as unknown as ReturnType<typeof useEmergencyContacts>)
    render(<EmergencyContactsPage />, { wrapper })
    expect(screen.getByText('Max Mustermann')).toBeTruthy()
    expect(screen.getByText('Erika Muster')).toBeTruthy()
  })

  it('renders level section labels', () => {
    mockUseEmergencyContacts.mockReturnValue({
      data: [
        { id: '1', org_id: 'o', name: 'Max', role: '', phone: '', email: '', escalation_level: 1 as const, available_247: false, notes: '', created_at: '', updated_at: '' },
      ],
      isLoading: false,
      isError: false,
    } as unknown as ReturnType<typeof useEmergencyContacts>)
    render(<EmergencyContactsPage />, { wrapper })
    // level description key is rendered via t('bcm.emergencyContacts.levelDesc.1')
    expect(screen.getByText('bcm.emergencyContacts.levelDesc.1')).toBeTruthy()
  })

  it('renders phone link for contacts with phone number', () => {
    mockUseEmergencyContacts.mockReturnValue({
      data: [
        { id: '1', org_id: 'o', name: 'Max', role: '', phone: '+49 228 999', email: '', escalation_level: 1 as const, available_247: false, notes: '', created_at: '', updated_at: '' },
      ],
      isLoading: false,
      isError: false,
    } as unknown as ReturnType<typeof useEmergencyContacts>)
    render(<EmergencyContactsPage />, { wrapper })
    const phoneLink = screen.getByRole('link', { name: /\+49 228 999/ })
    expect(phoneLink).toBeTruthy()
    expect(phoneLink.getAttribute('href')).toBe('tel:+49 228 999')
  })

  it('opens create dialog on button click', () => {
    mockUseEmergencyContacts.mockReturnValue({ data: [], isLoading: false, isError: false } as unknown as ReturnType<typeof useEmergencyContacts>)
    render(<EmergencyContactsPage />, { wrapper })
    fireEvent.click(screen.getAllByText('bcm.emergencyContacts.new')[0])
    expect(screen.getByRole('dialog')).toBeTruthy()
  })
})
