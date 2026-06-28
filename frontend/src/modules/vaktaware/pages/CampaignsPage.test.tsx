import { describe, it, expect, vi, beforeEach } from 'vitest'
import { screen, fireEvent } from '@testing-library/react'
import { renderWithProviders } from '../../../test-utils'
import CampaignsPage from './CampaignsPage'
import { useCampaigns, useCreateCampaign } from '../hooks/useCampaigns'
import { useTemplates } from '../hooks/useTemplates'
import { useTargetGroups } from '../hooks/useTargetGroups'
import type { Campaign, Template, TargetGroup } from '../types'

vi.mock('../hooks/useCampaigns', () => ({
  useCampaigns: vi.fn(),
  useCreateCampaign: vi.fn(),
  useLaunchCampaign: vi.fn(() => ({ mutate: vi.fn(), isPending: false })),
  useAbortCampaign: vi.fn(() => ({ mutate: vi.fn(), isPending: false })),
  useDownloadCampaignReport: vi.fn(() => () => {}),
}))
vi.mock('../hooks/useTemplates', () => ({ useTemplates: vi.fn() }))
vi.mock('../hooks/useTargetGroups', () => ({ useTargetGroups: vi.fn() }))

// ── fixtures ──────────────────────────────────────────────────────────────────

const CAMPAIGN: Campaign = {
  id: 'c-1',
  name: 'Q1 Awareness',
  status: 'draft',
  template_id: 't-1',
  target_group_id: 'tg-1',
  from_name: 'Security Team',
  from_email: 'security@example.com',
  subject: 'Wichtige Info',
  track_opens: false,
  betriebsrat_mode: true,
  created_at: '2026-01-10T08:00:00Z',
}

const TEMPLATE: Template = {
  id: 't-1',
  name: 'DocuSign Fake',
  subject: 'Bitte unterschreiben',
  from_name: 'Legal',
  from_email: 'legal@example.com',
  html_body: '<p>test</p>',
  attack_type: 'credential_harvesting',
  is_preset: true,
  created_at: '2026-01-01T00:00:00Z',
}

const TARGET_GROUP: TargetGroup = {
  id: 'tg-1',
  name: 'Alle Mitarbeitenden',
  source: 'manual',
  target_count: 42,
  created_at: '2026-01-01T00:00:00Z',
}

const mockMutate = vi.fn()

beforeEach(() => {
  vi.mocked(useCampaigns).mockReturnValue({ data: [], isLoading: false, error: null } as any)
  vi.mocked(useCreateCampaign).mockReturnValue({ mutate: mockMutate, isPending: false } as any)
  vi.mocked(useTemplates).mockReturnValue({ data: [TEMPLATE] } as any)
  vi.mocked(useTargetGroups).mockReturnValue({ data: [TARGET_GROUP] } as any)
  mockMutate.mockClear()
})

// ── loading state ─────────────────────────────────────────────────────────────

describe('CampaignsPage — loading state', () => {
  it('shows page header but not empty state while loading', () => {
    vi.mocked(useCampaigns).mockReturnValue({ data: [], isLoading: true, error: null } as any)
    renderWithProviders(<CampaignsPage />)
    expect(screen.getByText('Kampagnen')).toBeInTheDocument()
    expect(screen.queryByText('Keine Kampagnen')).not.toBeInTheDocument()
  })
})

// ── empty state ───────────────────────────────────────────────────────────────

describe('CampaignsPage — empty state', () => {
  it('shows empty state when no campaigns exist', () => {
    renderWithProviders(<CampaignsPage />)
    expect(screen.getByText('Keine Kampagnen')).toBeInTheDocument()
    expect(screen.getByText('Starten Sie Ihre erste Phishing-Simulation.')).toBeInTheDocument()
  })
})

// ── error state ───────────────────────────────────────────────────────────────

describe('CampaignsPage — error state', () => {
  it('passes error to ProGate — page header remains visible', () => {
    vi.mocked(useCampaigns).mockReturnValue({
      data: [],
      isLoading: false,
      error: new Error('Network error'),
    } as any)
    renderWithProviders(<CampaignsPage />)
    expect(screen.getByText('Kampagnen')).toBeInTheDocument()
  })
})

// ── data rendering ────────────────────────────────────────────────────────────

describe('CampaignsPage — data rendering', () => {
  it('renders a table row with campaign name and status', () => {
    vi.mocked(useCampaigns).mockReturnValue({ data: [CAMPAIGN], isLoading: false, error: null } as any)
    renderWithProviders(<CampaignsPage />)
    expect(screen.getByText('Q1 Awareness')).toBeInTheDocument()
    expect(screen.getByText('Entwurf')).toBeInTheDocument()
  })
})

// ── create mutation ───────────────────────────────────────────────────────────

describe('CampaignsPage — create mutation', () => {
  it('opens dialog, fills name, and calls mutate on form submit', () => {
    renderWithProviders(<CampaignsPage />)

    fireEvent.click(screen.getByText('Neue Kampagne'))
    expect(screen.getByRole('dialog')).toBeInTheDocument()

    fireEvent.change(screen.getByLabelText('Kampagnenname'), {
      target: { value: 'Q2 Phishing Test' },
    })

    const dialog = screen.getByRole('dialog')
    const form = dialog.querySelector('form')!
    fireEvent.submit(form)

    expect(mockMutate).toHaveBeenCalled()
    const payload = mockMutate.mock.calls[0][0] as { name: string }
    expect(payload.name).toBe('Q2 Phishing Test')
  })

  it('shows "Creating…" on the submit button while mutation is pending', () => {
    vi.mocked(useCreateCampaign).mockReturnValue({ mutate: vi.fn(), isPending: true } as any)
    renderWithProviders(<CampaignsPage />)
    fireEvent.click(screen.getByText('Neue Kampagne'))
    expect(screen.getByText('Creating…')).toBeInTheDocument()
  })
})
