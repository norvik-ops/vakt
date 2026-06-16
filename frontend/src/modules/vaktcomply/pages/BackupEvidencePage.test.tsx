import { render, screen, fireEvent } from '@testing-library/react'
import { describe, it, expect, vi } from 'vitest'
import { MemoryRouter } from 'react-router-dom'
import BackupEvidencePage from './BackupEvidencePage'

vi.mock('react-i18next', () => ({
  useTranslation: () => ({ t: (k: string) => k }),
}))

const mockCreate = vi.fn()
const mockDelete = vi.fn()

vi.mock('../hooks/useBackupJobs', () => ({
  useBackupJobs: vi.fn(),
  useBackupSummary: () => ({ data: { total_jobs: 1, overdue_backups: 0, overdue_restores: 0, tested_jobs: 1 } }),
  useCreateBackupJob: () => ({ mutate: mockCreate, isPending: false }),
  useUpdateBackupJob: () => ({ mutate: vi.fn(), isPending: false }),
  useDeleteBackupJob: () => ({ mutate: mockDelete }),
  useCreateRestoreTest: () => ({ mutate: vi.fn(), isPending: false }),
}))

const { useBackupJobs } = await import('../hooks/useBackupJobs')
const mockUseBackupJobs = vi.mocked(useBackupJobs)

function wrapper({ children }: { children: React.ReactNode }) {
  return <MemoryRouter>{children}</MemoryRouter>
}

const job = {
  id: '1', org_id: 'o', name: 'Postgres Nightly', source: 'db', destination: 'storage',
  frequency: 'daily' as const, encrypted: true, last_success_at: '2026-06-15T04:00:00Z',
  last_status: 'success' as const, restore_max_age_days: 365, notes: '',
  last_restore_test_at: '2026-05-01', backup_status: 'on_track' as const,
  restore_status: 'on_track' as const, created_at: '', updated_at: '',
}

describe('BackupEvidencePage', () => {
  it('shows empty state when no jobs', () => {
    mockUseBackupJobs.mockReturnValue({ data: [], isLoading: false, isError: false } as unknown as ReturnType<typeof useBackupJobs>)
    render(<BackupEvidencePage />, { wrapper })
    expect(screen.getByText('backup.emptyTitle')).toBeTruthy()
  })

  it('shows error state on failure', () => {
    mockUseBackupJobs.mockReturnValue({ data: undefined, isLoading: false, isError: true } as unknown as ReturnType<typeof useBackupJobs>)
    render(<BackupEvidencePage />, { wrapper })
    expect(screen.getByText('backup.loadError')).toBeTruthy()
  })

  it('renders job card with staleness badges', () => {
    mockUseBackupJobs.mockReturnValue({ data: [job], isLoading: false, isError: false } as unknown as ReturnType<typeof useBackupJobs>)
    render(<BackupEvidencePage />, { wrapper })
    expect(screen.getByText('Postgres Nightly')).toBeTruthy()
    // status.on_track key rendered in both backup + restore badges
    expect(screen.getAllByText(/backup.status.on_track/).length).toBeGreaterThan(0)
  })

  it('opens restore-test dialog', () => {
    mockUseBackupJobs.mockReturnValue({ data: [job], isLoading: false, isError: false } as unknown as ReturnType<typeof useBackupJobs>)
    render(<BackupEvidencePage />, { wrapper })
    fireEvent.click(screen.getByText('backup.restoreTest.document'))
    expect(screen.getByRole('dialog')).toBeTruthy()
  })

  it('opens create dialog', () => {
    mockUseBackupJobs.mockReturnValue({ data: [], isLoading: false, isError: false } as unknown as ReturnType<typeof useBackupJobs>)
    render(<BackupEvidencePage />, { wrapper })
    fireEvent.click(screen.getAllByText('backup.new')[0])
    expect(screen.getByRole('dialog')).toBeTruthy()
  })
})
