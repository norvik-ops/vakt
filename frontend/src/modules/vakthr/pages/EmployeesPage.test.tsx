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
  useStartOffboarding,
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
  useStartOffboarding: vi.fn(),
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

type R<T> = T extends (...args: unknown[]) => infer U ? U : never

beforeEach(() => {
  vi.mocked(useEmployees).mockReturnValue({ data: [], isLoading: false, pagination: undefined } as unknown as R<typeof useEmployees>)
  vi.mocked(useCreateEmployee).mockReturnValue({ mutateAsync: mockMutateAsync, isPending: false } as unknown as R<typeof useCreateEmployee>)
  vi.mocked(useUpdateEmployee).mockReturnValue({ mutateAsync: vi.fn().mockResolvedValue({}), isPending: false } as unknown as R<typeof useUpdateEmployee>)
  vi.mocked(useDeleteEmployee).mockReturnValue({ mutateAsync: vi.fn().mockResolvedValue(undefined), isPending: false } as unknown as R<typeof useDeleteEmployee>)
  vi.mocked(useChecklists).mockReturnValue({ data: [] } as unknown as R<typeof useChecklists>)
  vi.mocked(useChecklistRuns).mockReturnValue({ data: [] } as unknown as R<typeof useChecklistRuns>)
  vi.mocked(useStartChecklistRun).mockReturnValue({ mutateAsync: vi.fn().mockResolvedValue({ id: 'run-1' }), isPending: false } as unknown as R<typeof useStartChecklistRun>)
  vi.mocked(useStartOffboarding).mockReturnValue({ mutateAsync: vi.fn().mockResolvedValue({ id: 'run-off' }), isPending: false } as unknown as R<typeof useStartOffboarding>)
  vi.mocked(useUpdateChecklistRun).mockReturnValue({ mutate: vi.fn(), isPending: false } as unknown as R<typeof useUpdateChecklistRun>)
  mockMutateAsync.mockClear()
})

// ── loading state ─────────────────────────────────────────────────────────────

describe('EmployeesPage — loading state', () => {
  it('shows skeleton table while employees are loading', () => {
    vi.mocked(useEmployees).mockReturnValue({ data: [], isLoading: true, pagination: undefined } as unknown as R<typeof useEmployees>)
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
    vi.mocked(useEmployees).mockReturnValue({ data: [EMPLOYEE], isLoading: false, pagination: undefined } as unknown as R<typeof useEmployees>)
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
    vi.mocked(useEmployees).mockReturnValue({ data: [EMPLOYEE], isLoading: false, pagination: undefined } as unknown as R<typeof useEmployees>)
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
    vi.mocked(useEmployees).mockReturnValue({ data: [EMPLOYEE], isLoading: false, pagination: undefined } as unknown as R<typeof useEmployees>)
    renderWithProviders(<EmployeesPage />)

    fireEvent.click(screen.getByRole('button', { name: /mitarbeiter hinzufügen/i }))
    fireEvent.click(screen.getByRole('button', { name: /hinzufügen/i }))

    await waitFor(() => {
      expect(mockMutateAsync).not.toHaveBeenCalled()
      expect(screen.getAllByText('Dieses Feld ist erforderlich.').length).toBeGreaterThan(0)
    })
  })
})

// ── offboarding trigger sets employee status (R1-36c-03 Teil A) ──────────────────

describe('EmployeesPage — starting a checklist run', () => {
  const ONBOARD = { id: 'cl-on', name: 'Standard Onboarding', type: 'onboarding' } as unknown as import('../types').Checklist
  const OFFBOARD = { id: 'cl-off', name: 'Standard Offboarding', type: 'offboarding' } as unknown as import('../types').Checklist

  function wireStartMocks() {
    const startRun = vi.fn().mockResolvedValue({ id: 'run-1' })
    const startOffboarding = vi.fn().mockResolvedValue({ id: 'run-off' })
    vi.mocked(useEmployees).mockReturnValue({ data: [EMPLOYEE], isLoading: false, pagination: undefined } as unknown as R<typeof useEmployees>)
    vi.mocked(useChecklistRuns).mockReturnValue({ data: [] } as unknown as R<typeof useChecklistRuns>)
    vi.mocked(useChecklists).mockReturnValue({ data: [ONBOARD, OFFBOARD] } as unknown as R<typeof useChecklists>)
    vi.mocked(useStartChecklistRun).mockReturnValue({ mutateAsync: startRun, isPending: false } as unknown as R<typeof useStartChecklistRun>)
    vi.mocked(useStartOffboarding).mockReturnValue({ mutateAsync: startOffboarding, isPending: false } as unknown as R<typeof useStartOffboarding>)
    return { startRun, startOffboarding }
  }

  it('routes an offboarding checklist through the offboard endpoint (sets status)', async () => {
    const { startRun, startOffboarding } = wireStartMocks()
    renderWithProviders(<EmployeesPage />)

    fireEvent.click(screen.getByRole('button', { name: /checkliste starten/i }))
    fireEvent.click(screen.getByRole('combobox'))
    fireEvent.click(screen.getByRole('option', { name: /Standard Offboarding/ }))
    fireEvent.click(screen.getByRole('button', { name: /^Starten$/ }))

    await waitFor(() => {
      expect(startOffboarding).toHaveBeenCalledWith('emp-1')
    })
    expect(startRun).not.toHaveBeenCalled()
  })

  it('keeps the generic start for an onboarding checklist', async () => {
    const { startRun, startOffboarding } = wireStartMocks()
    renderWithProviders(<EmployeesPage />)

    fireEvent.click(screen.getByRole('button', { name: /checkliste starten/i }))
    fireEvent.click(screen.getByRole('combobox'))
    fireEvent.click(screen.getByRole('option', { name: /Standard Onboarding/ }))
    fireEvent.click(screen.getByRole('button', { name: /^Starten$/ }))

    await waitFor(() => {
      expect(startRun).toHaveBeenCalledWith(expect.objectContaining({ employee_id: 'emp-1', checklist_id: 'cl-on' }))
    })
    expect(startOffboarding).not.toHaveBeenCalled()
  })
})
