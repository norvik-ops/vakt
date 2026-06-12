import { describe, it, expect, vi, beforeEach } from 'vitest'
import { screen, fireEvent, waitFor } from '@testing-library/react'
import { renderWithProviders } from '../../../test-utils'
import AccessReviewsPage from './AccessReviewsPage'
import { apiFetch } from '../../../api/client'
import type { AccessReview } from '../types'

vi.mock('../../../api/client', async (importOriginal) => {
  const actual = await importOriginal<typeof import('../../../api/client')>()
  return { ...actual, apiFetch: vi.fn() }
})

// ── fixtures ──────────────────────────────────────────────────────────────────

const REVIEW: AccessReview = {
  id: 'rev-1',
  org_id: 'org-1',
  period_label: 'Q1 2026',
  status: 'open',
  total_entries: 5,
  stale_entries: 2,
  revoked_entries: 0,
  created_at: '2026-01-01T00:00:00Z',
}

beforeEach(() => {
  vi.mocked(apiFetch).mockResolvedValue([])
})

// ── loading state ─────────────────────────────────────────────────────────────

describe('AccessReviewsPage — loading state', () => {
  it('shows page header title immediately', () => {
    vi.mocked(apiFetch).mockReturnValue(new Promise(() => undefined))
    renderWithProviders(<AccessReviewsPage />)
    expect(screen.getByText('Quartalsweise Zugriffsreviews')).toBeInTheDocument()
  })
})

// ── empty state ───────────────────────────────────────────────────────────────

describe('AccessReviewsPage — empty state', () => {
  it('shows empty state and "Review starten" button after loading', async () => {
    renderWithProviders(<AccessReviewsPage />)
    await waitFor(() => {
      expect(screen.getByText('Noch keine Access-Reviews')).toBeInTheDocument()
    })
    expect(screen.getByRole('button', { name: /review starten/i })).toBeInTheDocument()
  })
})

// ── data rendering ────────────────────────────────────────────────────────────

describe('AccessReviewsPage — data rendering', () => {
  it('renders the review period_label and stale_entries count', async () => {
    vi.mocked(apiFetch).mockResolvedValue([REVIEW])
    renderWithProviders(<AccessReviewsPage />)
    await waitFor(() => {
      expect(screen.getByText('Q1 2026')).toBeInTheDocument()
    })
    expect(screen.getByText(/2 veraltet/)).toBeInTheDocument()
  })
})

// ── create mutation ───────────────────────────────────────────────────────────

describe('AccessReviewsPage — create mutation', () => {
  it('calls POST /vaktvault/access-reviews when "Review starten" is clicked', async () => {
    // Use persistent mock returning an array to avoid map-on-non-array after refetch
    vi.mocked(apiFetch).mockResolvedValue([])

    renderWithProviders(<AccessReviewsPage />)
    await waitFor(() => screen.getByText('Noch keine Access-Reviews'))

    fireEvent.click(screen.getByRole('button', { name: /review starten/i }))

    await waitFor(() => {
      const calls = vi.mocked(apiFetch).mock.calls
      const postCall = calls.find(c => c[1] !== undefined && (c[1] as RequestInit).method === 'POST')
      expect(postCall).toBeDefined()
    })
  })

  it('disables "Review starten" button while mutation is pending', async () => {
    vi.mocked(apiFetch).mockImplementation((_url: string, options?: RequestInit) => {
      if (options?.method === 'POST') {
        return new Promise<never>(() => undefined)
      }
      return Promise.resolve([])
    })

    renderWithProviders(<AccessReviewsPage />)
    await waitFor(() => screen.getByText('Noch keine Access-Reviews'))

    fireEvent.click(screen.getByRole('button', { name: /review starten/i }))

    await waitFor(() => {
      expect(screen.getByText('Erstellen…')).toBeInTheDocument()
    })
  })
})
