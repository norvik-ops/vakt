import { describe, it, expect, vi, beforeEach } from 'vitest'
import { screen, fireEvent, waitFor } from '@testing-library/react'
import { renderWithProviders } from '../../../test-utils'
import DSRPage from './DSRPage'
import {
  useDSRs,
  useCreateDSR,
  useUpdateDSR,
  useDeleteDSR,
  useDSRSummary,
  useResolveDSR,
} from '../hooks/useDSRs'
import type { DSR, DSRSummary } from '../types'

vi.mock('../hooks/useDSRs', () => ({
  useDSRs: vi.fn(),
  useCreateDSR: vi.fn(),
  useUpdateDSR: vi.fn(),
  useDeleteDSR: vi.fn(),
  useDSRSummary: vi.fn(),
  useResolveDSR: vi.fn(),
}))

vi.mock('../../../shared/hooks/useFormatDate', () => ({
  useFormatDate: () => ({ formatDate: (d: string) => d }),
}))

// ── fixtures ──────────────────────────────────────────────────────────────────

const DSR_FIXTURE: DSR = {
  id: 'dsr-1',
  org_id: 'org-1',
  requester_name: 'Max Mustermann',
  requester_email: 'max@example.com',
  type: 'access',
  status: 'open',
  received_at: '2026-01-01T00:00:00Z',
  created_at: '2026-01-01T00:00:00Z',
  updated_at: '2026-01-01T00:00:00Z',
}

const SUMMARY: DSRSummary = {
  open_count: 1,
  overdue_count: 0,
  fulfilled_last_12m: 5,
  rejected_last_12m: 0,
  on_time_rate_pct: 100,
  avg_days_to_complete: 10,
}

type R<T> = T extends (...args: unknown[]) => infer U ? U : never

const mockCreate = vi.fn()

beforeEach(() => {
  vi.mocked(useDSRs).mockReturnValue({ data: [], isLoading: false, isError: false } as unknown as R<typeof useDSRs>)
  vi.mocked(useDSRSummary).mockReturnValue({ data: undefined } as unknown as R<typeof useDSRSummary>)
  vi.mocked(useCreateDSR).mockReturnValue({ mutate: mockCreate, isPending: false } as unknown as R<typeof useCreateDSR>)
  vi.mocked(useUpdateDSR).mockReturnValue({ mutate: vi.fn(), isPending: false } as unknown as R<typeof useUpdateDSR>)
  vi.mocked(useDeleteDSR).mockReturnValue({ mutate: vi.fn(), isPending: false } as unknown as R<typeof useDeleteDSR>)
  vi.mocked(useResolveDSR).mockReturnValue({ mutate: vi.fn(), isPending: false } as unknown as R<typeof useResolveDSR>)
  mockCreate.mockClear()
})

// ── loading state ─────────────────────────────────────────────────────────────

describe('DSRPage — loading state', () => {
  it('shows page title while loading', () => {
    vi.mocked(useDSRs).mockReturnValue({ data: [], isLoading: true, isError: false } as unknown as R<typeof useDSRs>)
    renderWithProviders(<DSRPage />)
    expect(screen.getByText('Datenschutzanfragen (DSR)')).toBeInTheDocument()
  })
})

// ── empty state ───────────────────────────────────────────────────────────────

describe('DSRPage — empty state', () => {
  it('shows empty state and "DSR anlegen" button when no DSRs exist', () => {
    renderWithProviders(<DSRPage />)
    expect(screen.getByText('Keine Datenschutzanfragen')).toBeInTheDocument()
    expect(screen.getAllByRole('button', { name: /dsr anlegen/i }).length).toBeGreaterThan(0)
  })
})

// ── data rendering ────────────────────────────────────────────────────────────

describe('DSRPage — data rendering', () => {
  it('renders DSR requester name and type badge', () => {
    vi.mocked(useDSRs).mockReturnValue({ data: [DSR_FIXTURE], isLoading: false, isError: false } as unknown as R<typeof useDSRs>)
    vi.mocked(useDSRSummary).mockReturnValue({ data: SUMMARY } as unknown as R<typeof useDSRSummary>)
    renderWithProviders(<DSRPage />)
    expect(screen.getByText('Max Mustermann')).toBeInTheDocument()
    expect(screen.getByText('max@example.com')).toBeInTheDocument()
  })
})

// ── create mutation ───────────────────────────────────────────────────────────

describe('DSRPage — create mutation', () => {
  it('calls mutate with requester data on button click', async () => {
    mockCreate.mockImplementation(() => undefined)
    renderWithProviders(<DSRPage />)

    // Open the dialog using the header button
    fireEvent.click(screen.getAllByRole('button', { name: /dsr anlegen/i })[0])
    expect(screen.getByRole('dialog')).toBeInTheDocument()

    fireEvent.change(screen.getByPlaceholderText('z.B. Max Mustermann'), {
      target: { value: 'Lisa Beispiel' },
    })
    fireEvent.change(screen.getByPlaceholderText('max@example.com'), {
      target: { value: 'lisa@example.com' },
    })

    // Submit via the dialog's own "DSR anlegen" button (last one = inside dialog footer)
    const submitButtons = screen.getAllByRole('button', { name: /dsr anlegen/i })
    fireEvent.click(submitButtons[submitButtons.length - 1])

    await waitFor(() => {
      expect(mockCreate).toHaveBeenCalledWith(
        expect.objectContaining({
          requester_name: 'Lisa Beispiel',
          requester_email: 'lisa@example.com',
        }),
        expect.any(Object),
      )
    })
  })

  it('submit button is disabled when required fields are empty', () => {
    renderWithProviders(<DSRPage />)
    fireEvent.click(screen.getAllByRole('button', { name: /dsr anlegen/i })[0])

    // Dialog submit button should be disabled when fields empty (canSubmitCreate = false)
    const submitButtons = screen.getAllByRole('button', { name: /dsr anlegen/i })
    const dialogSubmit = submitButtons[submitButtons.length - 1]
    expect(dialogSubmit).toBeDisabled()
    expect(mockCreate).not.toHaveBeenCalled()
    // No mutate call since button was disabled
  })
})
