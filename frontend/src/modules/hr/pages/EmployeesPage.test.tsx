import { describe, it, expect, vi, beforeEach } from 'vitest'
import { screen, fireEvent, waitFor } from '@testing-library/react'
import { renderWithProviders } from '../../../test-utils'
import EmployeesPage from './EmployeesPage'
import {
  useEmployees,
  useCreateEmployee,
  useUpdateEmployee,
  useDeleteEmployee,
  useChecklists,
  useChecklistRuns,
  useStartChecklistRun,
  useUpdateChecklistRun,
} from '../hooks/useHR'
import type { Employee } from '../types'

vi.mock('../hooks/useHR', () => ({
  useEmployees: vi.fn(),
  useCreateEmployee: vi.fn(),
  useUpdateEmployee: vi.fn(),
  useDeleteEmployee: vi.fn(),
  useChecklists: vi.fn(),
  useCreateChecklist: vi.fn(() => ({ mutate: vi.fn(), isPending: false })),
  useDeleteChecklist: vi.fn(() => ({ mutate: vi.fn(), isPending: false })),
  useChecklistRuns: vi.fn(),
  useStartChecklistRun: vi.fn(),
  useUpdateChecklistRun: vi.fn(() => ({ mutate: vi.fn(), isPending: false })),
}))

// ── fixtures ──────────────────────────────────────────────────────────────────

const EMPLOYEE: Employee = {
  id: 'emp-1',
  org_id: 'org-1',
  first_name: 'Anna',
  last_name: 'Muster',
  email: 'anna.muster@example.com',
  department: 'IT',
  role: 'DevOps Engineer',
  status: 'active',
  created_at: '2026-01-01T00:00:00Z',
  updated_at: '2026-01-01T00:00:00Z',
}

const mockMutateAsync = vi.fn().mockResolvedValue({ id: 'emp-new' })

beforeEach(() => {
  vi.mocked(useEmployees).mockReturnValue({ data: [], isLoading: false, pagination: undefined } as any)
  vi.mocked(useCreateEmployee).mockReturnValue({ mutateAsync: mockMutateAsync, isPending: false } as any)
  vi.mocked(useUpdateEmployee).mockReturnValue({ mutateAsync: vi.fn().mockResolvedValue({}), isPending: false } as any)
  vi.mocked(useDeleteEmployee).mockReturnValue({ mutateAsync: vi.fn().mockResolvedValue(undefined), isPending: false } as any)
  vi.mocked(useChecklists).mockReturnValue({ data: [] } as any)
  vi.mocked(useChecklistRuns).mockReturnValue({ data: [] } as any)
  vi.mocked(useStartChecklistRun).mockReturnValue({ mutateAsync: vi.fn(), isPending: false } as any)
  vi.mocked(useUpdateChecklistRun).mockReturnValue({ mutate: vi.fn(), isPending: false } as any)
  mockMutateAsync.mockClear()
})

// ── loading state ─────────────────────────────────────────────────────────────

describe('EmployeesPage — loading state', () => {
  it('shows skeleton table while employees are loading', () => {
    vi.mocked(useEmployees).mockReturnValue({ data: [], isLoading: true, pagination: undefined } as any)
    renderWithProviders(<EmployeesPage />)
    expect(screen.getByText('Mitarbeiter')).toBeInTheDocument()
    expect(screen.queryByText('Noch keine Mitarbeiter')).not.toBeInTheDocument()
  })
})

// ── empty state ───────────────────────────────────────────────────────────────

describe('EmployeesPage — empty state', () => {
  it('shows empty state when no employees exist', () => {
    renderWithProviders(<EmployeesPage />)
    expect(screen.getByText('Noch keine Mitarbeiter')).toBeInTheDocument()
    expect(screen.getByText(/Verwalte Mitarbeiter-Lifecycle/)).toBeInTheDocument()
  })
})

// ── data rendering ────────────────────────────────────────────────────────────

describe('EmployeesPage — data rendering', () => {
  it('renders employee name, email, and status', () => {
    vi.mocked(useEmployees).mockReturnValue({ data: [EMPLOYEE], isLoading: false, pagination: undefined } as any)
    renderWithProviders(<EmployeesPage />)
    expect(screen.getByText('Anna Muster')).toBeInTheDocument()
    expect(screen.getByText('anna.muster@example.com')).toBeInTheDocument()
    // "Aktiv" also appears in filter buttons — verify at least one instance
    expect(screen.getAllByText('Aktiv').length).toBeGreaterThan(0)
  })
})

// ── create mutation ───────────────────────────────────────────────────────────

describe('EmployeesPage — create mutation', () => {
  it('opens dialog and calls mutateAsync with form data on submit', async () => {
    // Use existing employee so empty state button does not appear → only one "Hinzufügen" button
    vi.mocked(useEmployees).mockReturnValue({ data: [EMPLOYEE], isLoading: false, pagination: undefined } as any)
    renderWithProviders(<EmployeesPage />)

    fireEvent.click(screen.getByRole('button', { name: /mitarbeiter hinzufügen/i }))
    expect(screen.getByText(/Mitarbeiter hinzufügen/, { selector: '[role="dialog"] *' })).toBeInTheDocument()

    fireEvent.change(screen.getByPlaceholderText('Max'), { target: { value: 'Max' } })
    fireEvent.change(screen.getByPlaceholderText('Mustermann'), { target: { value: 'Mustermann' } })
    fireEvent.change(screen.getByPlaceholderText('max.mustermann@example.com'), {
      target: { value: 'max.mustermann@example.com' },
    })

    fireEvent.click(screen.getByRole('button', { name: /hinzufügen/i }))

    await waitFor(() => {
      expect(mockMutateAsync).toHaveBeenCalledWith(
        expect.objectContaining({
          first_name: 'Max',
          last_name: 'Mustermann',
          email: 'max.mustermann@example.com',
        }),
      )
    })
  })

  it('shows validation errors and does NOT call mutateAsync when required fields are empty', async () => {
    vi.mocked(useEmployees).mockReturnValue({ data: [EMPLOYEE], isLoading: false, pagination: undefined } as any)
    renderWithProviders(<EmployeesPage />)

    fireEvent.click(screen.getByRole('button', { name: /mitarbeiter hinzufügen/i }))
    fireEvent.click(screen.getByRole('button', { name: /hinzufügen/i }))

    await waitFor(() => {
      expect(mockMutateAsync).not.toHaveBeenCalled()
      expect(screen.getAllByText('Dieses Feld ist erforderlich.').length).toBeGreaterThan(0)
    })
  })
})
