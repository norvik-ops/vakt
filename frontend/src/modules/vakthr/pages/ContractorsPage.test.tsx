import { describe, it, expect, vi, beforeEach } from 'vitest'
import { screen, fireEvent, waitFor } from '@testing-library/react'
import { renderWithProviders } from '../../../test-utils'
import ContractorsPage from './ContractorsPage'
import { apiFetch } from '../../../api/client'
import type { Contractor } from '../types'

vi.mock('../../../api/client', async (importOriginal) => {
  const actual = await importOriginal<typeof import('../../../api/client')>()
  return { ...actual, apiFetch: vi.fn() }
})

// ── fixtures ──────────────────────────────────────────────────────────────────

const CONTRACTOR: Contractor = {
  id: 'con-1',
  org_id: 'org-1',
  first_name: 'Karl',
  last_name: 'Freelancer',
  email: 'karl@freelance.de',
  company: 'KF Solutions',
  contract_start: '2026-01-01',
  contract_end: '2026-12-31',
  access_scope: ['vpn'],
  nda_signed: true,
  avv_signed: false,
  status: 'active',
  created_at: '2026-01-01T00:00:00Z',
  updated_at: '2026-01-01T00:00:00Z',
}

beforeEach(() => {
  vi.mocked(apiFetch).mockResolvedValue([])
})

// ── loading state ─────────────────────────────────────────────────────────────

describe('ContractorsPage — loading state', () => {
  it('shows page title immediately', () => {
    vi.mocked(apiFetch).mockReturnValue(new Promise(() => undefined))
    renderWithProviders(<ContractorsPage />)
    expect(screen.getByText('Auftragnehmer & Freelancer')).toBeInTheDocument()
  })
})

// ── empty state ───────────────────────────────────────────────────────────────

describe('ContractorsPage — empty state', () => {
  it('shows empty state and "Auftragnehmer anlegen" button after loading', async () => {
    renderWithProviders(<ContractorsPage />)
    await waitFor(() => {
      expect(screen.getByText('Noch keine Auftragnehmer')).toBeInTheDocument()
    })
    expect(screen.getByRole('button', { name: /auftragnehmer anlegen/i })).toBeInTheDocument()
  })
})

// ── data rendering ────────────────────────────────────────────────────────────

describe('ContractorsPage — data rendering', () => {
  it('renders contractor name and company after loading', async () => {
    vi.mocked(apiFetch).mockResolvedValue([CONTRACTOR])
    renderWithProviders(<ContractorsPage />)
    await waitFor(() => {
      expect(screen.getByText('Karl Freelancer')).toBeInTheDocument()
    })
    expect(screen.getByText(/KF Solutions/)).toBeInTheDocument()
  })
})

// ── create mutation ───────────────────────────────────────────────────────────

describe('ContractorsPage — create mutation', () => {
  it('opens dialog and shows required fields', async () => {
    renderWithProviders(<ContractorsPage />)
    await waitFor(() => screen.getByText('Noch keine Auftragnehmer'))

    fireEvent.click(screen.getAllByRole('button', { name: /auftragnehmer anlegen/i })[0])
    expect(screen.getByRole('dialog')).toBeInTheDocument()
    expect(screen.getAllByText('Auftragnehmer anlegen').length).toBeGreaterThan(0)
    expect(screen.getByText('Vorname *')).toBeInTheDocument()
    expect(screen.getByText('Nachname *')).toBeInTheDocument()
  })

  it('calls POST /vakthr/contractors when form is filled and submitted', async () => {
    // Use a persistent mock that always returns an array to avoid filter errors on refetch
    vi.mocked(apiFetch).mockResolvedValue([])

    renderWithProviders(<ContractorsPage />)
    await waitFor(() => screen.getByText('Noch keine Auftragnehmer'))

    fireEvent.click(screen.getAllByRole('button', { name: /auftragnehmer anlegen/i })[0])

    const dialog = screen.getByRole('dialog')
    const inputs = dialog.querySelectorAll('input[type="text"], input:not([type])')
    fireEvent.change(inputs[0] as HTMLInputElement, { target: { value: 'Max' } })
    fireEvent.change(inputs[1] as HTMLInputElement, { target: { value: 'Mustermann' } })

    const dateInputs = dialog.querySelectorAll('input[type="date"]')
    fireEvent.change(dateInputs[0] as HTMLInputElement, { target: { value: '2026-01-01' } })
    fireEvent.change(dateInputs[1] as HTMLInputElement, { target: { value: '2026-12-31' } })

    const form = dialog.querySelector('form')!
    fireEvent.submit(form)

    await waitFor(() => {
      const calls = vi.mocked(apiFetch).mock.calls
      const postCall = calls.find(c => c[1] !== undefined && (c[1] as RequestInit).method === 'POST')
      expect(postCall).toBeDefined()
    })
  })
})

// ── error state ───────────────────────────────────────────────────────────────

describe('ContractorsPage — mutation error', () => {
  it('shows inline error when contractor creation fails with a 500', async () => {
    vi.mocked(apiFetch).mockImplementation((_url: string, options?: RequestInit) => {
      if (options?.method === 'POST') return Promise.reject(new Error('Internal Server Error'))
      return Promise.resolve([])
    })

    renderWithProviders(<ContractorsPage />)

    await waitFor(() => {
      expect(screen.queryByText(/Auftragnehmer anlegen/)).toBeInTheDocument()
    })

    fireEvent.click(screen.getAllByText(/Auftragnehmer anlegen/)[0])

    const dialog = screen.getByRole('dialog')
    const inputs = dialog.querySelectorAll('input[type="text"], input:not([type])')
    fireEvent.change(inputs[0] as HTMLInputElement, { target: { value: 'Fail' } })
    fireEvent.change(inputs[1] as HTMLInputElement, { target: { value: 'User' } })

    const form = dialog.querySelector('form')!
    fireEvent.submit(form)

    await waitFor(() => {
      expect(screen.getByText(/Internal Server Error/)).toBeInTheDocument()
    })
  })
})
