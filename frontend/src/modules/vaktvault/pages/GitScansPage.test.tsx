import { describe, it, expect, vi, beforeEach } from 'vitest'
import { screen, fireEvent } from '@testing-library/react'
import { renderWithProviders } from '../../../test-utils'
import GitScansPage from './GitScansPage'
import { useGitScans, useTriggerGitScan, useGitScanResults, useDismissScanResult } from '../hooks/useGitScans'
import type { GitScan } from '../types'

vi.mock('../hooks/useGitScans', () => ({
  useGitScans: vi.fn(),
  useTriggerGitScan: vi.fn(),
  useGitScanResults: vi.fn(),
  useDismissScanResult: vi.fn(),
}))

// ── fixtures ──────────────────────────────────────────────────────────────────

const SCAN: GitScan = {
  id: 'scan-1',
  repo_url: 'https://github.com/acme/backend',
  status: 'completed',
  result_count: 2,
  created_at: '2026-01-15T10:00:00Z',
}

const mockMutate = vi.fn()

beforeEach(() => {
  vi.mocked(useGitScans).mockReturnValue({ data: [], isLoading: false } as any)
  vi.mocked(useTriggerGitScan).mockReturnValue({ mutate: mockMutate, isPending: false } as any)
  vi.mocked(useGitScanResults).mockReturnValue({ data: [], isLoading: false } as any)
  vi.mocked(useDismissScanResult).mockReturnValue({ mutate: vi.fn(), isPending: false } as any)
  mockMutate.mockClear()
})

// ── loading state ─────────────────────────────────────────────────────────────

describe('GitScansPage — loading state', () => {
  it('shows page header but not empty state while loading', () => {
    vi.mocked(useGitScans).mockReturnValue({ data: [], isLoading: true } as any)
    renderWithProviders(<GitScansPage />)
    // i18n is not initialised in tests — t() returns the raw key.
    expect(screen.getByText('vault.gitScans.title')).toBeInTheDocument()
    expect(screen.queryByText('vault.gitScans.noScans')).not.toBeInTheDocument()
  })
})

// ── empty state ───────────────────────────────────────────────────────────────

describe('GitScansPage — empty state', () => {
  it('shows empty state when no scans exist', () => {
    renderWithProviders(<GitScansPage />)
    expect(screen.getByText('vault.gitScans.noScans')).toBeInTheDocument()
    expect(screen.getByText('vault.gitScans.noScansHint')).toBeInTheDocument()
  })
})

// ── data rendering ────────────────────────────────────────────────────────────

describe('GitScansPage — data rendering', () => {
  it('renders a row for each scan with repo URL and status badge', () => {
    vi.mocked(useGitScans).mockReturnValue({ data: [SCAN], isLoading: false } as any)
    renderWithProviders(<GitScansPage />)
    expect(screen.getByText(/github\.com\/acme\/backend/)).toBeInTheDocument()
    expect(screen.getByText('completed')).toBeInTheDocument()
    expect(screen.getByText('2 findings')).toBeInTheDocument()
  })
})

// ── create mutation ───────────────────────────────────────────────────────────

describe('GitScansPage — create mutation', () => {
  it('opens dialog, fills URL, and calls mutate on form submit', () => {
    renderWithProviders(<GitScansPage />)

    fireEvent.click(screen.getByText('vault.gitScans.newScan'))
    expect(screen.getByText('Scan Repository')).toBeInTheDocument()

    fireEvent.change(screen.getByLabelText('Repository URL'), {
      target: { value: 'https://github.com/acme/new-repo' },
    })

    const dialog = screen.getByRole('dialog')
    const form = dialog.querySelector('form')!
    fireEvent.submit(form)

    expect(mockMutate).toHaveBeenCalledWith(
      { repo_url: 'https://github.com/acme/new-repo' },
      expect.any(Object),
    )
  })

  it('shows "Starting…" on the submit button while mutation is pending', () => {
    vi.mocked(useTriggerGitScan).mockReturnValue({ mutate: vi.fn(), isPending: true } as any)
    renderWithProviders(<GitScansPage />)
    fireEvent.click(screen.getByText('vault.gitScans.newScan'))
    expect(screen.getByText('Starting…')).toBeInTheDocument()
  })
})
