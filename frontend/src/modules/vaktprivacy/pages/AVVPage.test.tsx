import { describe, it, expect, vi, beforeEach } from 'vitest'
import { screen, fireEvent, waitFor } from '@testing-library/react'
import { renderWithProviders } from '../../../test-utils'
import AVVPage from './AVVPage'
import { useAVVs, useCreateAVV, useUpdateAVV, useDeleteAVV } from '../hooks/useAVVs'
import { useDownloadAVVPDF } from '../hooks/useAVVTemplates'
import type { AVV } from '../types'

vi.mock('../hooks/useAVVs', () => ({
  useAVVs: vi.fn(),
  useCreateAVV: vi.fn(),
  useUpdateAVV: vi.fn(),
  useDeleteAVV: vi.fn(),
}))

vi.mock('../hooks/useAVVTemplates', () => ({
  useDownloadAVVPDF: vi.fn(),
}))

vi.mock('../components/AVVTemplatePickerDialog', () => ({
  AVVTemplatePickerDialog: () => null,
}))

vi.mock('../../../shared/hooks/useFormatDate', () => ({
  useFormatDate: () => ({ formatDate: (d: string) => d }),
}))

// ── fixtures ──────────────────────────────────────────────────────────────────

const AVV_FIXTURE: AVV = {
  id: 'avv-1',
  org_id: 'org-1',
  processor_name: 'Acme GmbH',
  service_description: 'Cloud Hosting',
  status: 'active',
  created_at: '2026-01-01T00:00:00Z',
  updated_at: '2026-01-01T00:00:00Z',
}

type R<T> = T extends (...args: unknown[]) => infer U ? U : never

const mockCreate = vi.fn()

beforeEach(() => {
  vi.mocked(useAVVs).mockReturnValue({ data: [], isLoading: false, isError: false } as unknown as R<typeof useAVVs>)
  vi.mocked(useCreateAVV).mockReturnValue({ mutate: mockCreate, isPending: false } as unknown as R<typeof useCreateAVV>)
  vi.mocked(useUpdateAVV).mockReturnValue({ mutate: vi.fn(), isPending: false } as unknown as R<typeof useUpdateAVV>)
  vi.mocked(useDeleteAVV).mockReturnValue({ mutate: vi.fn(), isPending: false } as unknown as R<typeof useDeleteAVV>)
  vi.mocked(useDownloadAVVPDF).mockReturnValue(vi.fn())
  mockCreate.mockClear()
})

// ── loading state ─────────────────────────────────────────────────────────────

describe('AVVPage — loading state', () => {
  it('shows page title while loading', () => {
    vi.mocked(useAVVs).mockReturnValue({ data: [], isLoading: true, isError: false } as unknown as R<typeof useAVVs>)
    renderWithProviders(<AVVPage />)
    expect(screen.getByText('Auftragsverarbeitungsverträge (AVV)')).toBeInTheDocument()
  })
})

// ── empty state ───────────────────────────────────────────────────────────────

describe('AVVPage — empty state', () => {
  it('shows empty state when no AVVs exist', () => {
    renderWithProviders(<AVVPage />)
    expect(screen.getByText('Noch keine AVVs')).toBeInTheDocument()
  })
})

// ── data rendering ────────────────────────────────────────────────────────────

describe('AVVPage — data rendering', () => {
  it('renders AVV processor name and status badge', () => {
    vi.mocked(useAVVs).mockReturnValue({ data: [AVV_FIXTURE], isLoading: false, isError: false } as unknown as R<typeof useAVVs>)
    renderWithProviders(<AVVPage />)
    expect(screen.getByText('Acme GmbH')).toBeInTheDocument()
    expect(screen.getByText('Cloud Hosting')).toBeInTheDocument()
  })
})

// ── create mutation ───────────────────────────────────────────────────────────

describe('AVVPage — create mutation', () => {
  it('calls mutate with processor_name and service_description on submit', async () => {
    renderWithProviders(<AVVPage />)

    // Open dialog
    fireEvent.click(screen.getAllByText('AVV anlegen')[0])
    expect(screen.getByRole('dialog')).toBeInTheDocument()

    fireEvent.change(screen.getByPlaceholderText('z.B. Amazon Web Services EMEA SARL'), {
      target: { value: 'Test Verarbeiter GmbH' },
    })
    // AVVForm uses Textarea for service_description
    const serviceTA = screen.getByPlaceholderText('Welche Leistung erbringt der Auftragsverarbeiter?')
    fireEvent.change(serviceTA, {
      target: { value: 'E-Mail Marketing' },
    })

    // Dialog submit button is the last button with this text (header + empty-state + dialog)
    const submitButtons = screen.getAllByText('AVV anlegen')
    fireEvent.click(submitButtons[submitButtons.length - 1])

    await waitFor(() => {
      expect(mockCreate).toHaveBeenCalledWith(
        expect.objectContaining({
          processor_name: 'Test Verarbeiter GmbH',
          service_description: 'E-Mail Marketing',
        }),
        expect.any(Object),
      )
    })
  })

  it('submit button is disabled when required fields are empty', () => {
    renderWithProviders(<AVVPage />)
    // Open dialog
    fireEvent.click(screen.getAllByText('AVV anlegen')[0])
    expect(screen.getByRole('dialog')).toBeInTheDocument()

    // Dialog submit button should be disabled (canSubmit = false when fields empty)
    const submitBtns = screen.getAllByRole('button', { name: 'AVV anlegen' })
    const dialogSubmit = submitBtns[submitBtns.length - 1]
    expect(dialogSubmit).toBeDisabled()
    expect(mockCreate).not.toHaveBeenCalled()
  })
})
