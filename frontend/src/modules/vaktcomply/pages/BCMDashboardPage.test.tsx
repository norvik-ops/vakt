import { render, screen } from '@testing-library/react'
import { describe, it, expect, vi, beforeEach } from 'vitest'
import { MemoryRouter } from 'react-router-dom'
import BCMDashboardPage from './BCMDashboardPage'

vi.mock('react-i18next', () => ({
  useTranslation: () => ({ t: (k: string) => k }),
}))

vi.mock('../hooks/useBCMScore', () => ({
  useBCMReadinessScore: () => ({
    data: {
      score: 80,
      criteria: [
        { key: 'bia', points: 20, met: true, description: 'BIA vorhanden' },
        { key: 'wap', points: 20, met: true, description: 'WAP vorhanden' },
        { key: 'contacts', points: 20, met: true, description: 'Kontakte vorhanden' },
        { key: 'high_bia', points: 20, met: true, description: 'Kritische Prozesse' },
        { key: 'tested', points: 20, met: false, description: 'WAP getestet' },
      ],
      computed_at: '2026-06-15T10:00:00Z',
    },
    isLoading: false,
  }),
}))

vi.mock('../hooks/useBIA', () => ({
  useBIASummary: () => ({
    data: { total: 3, high_critical: 2, avg_rto_hours: 12, avg_rpo_hours: 4 },
  }),
}))

vi.mock('../hooks/useRecoveryPlans', () => ({
  useRecoveryPlans: () => ({
    data: [
      { id: '1', title: 'WAP IT', status: 'tested', rto_hours: 4, steps: [] },
    ],
  }),
}))

vi.mock('../hooks/useEmergencyContacts', () => ({
  useEmergencyContacts: () => ({
    data: [
      { id: '1', name: 'Max Mustermann', escalation_level: 1, role: 'CISO', phone: '', email: '', available_247: true, notes: '' },
      { id: '2', name: 'Erika Muster', escalation_level: 2, role: 'ISB', phone: '', email: '', available_247: false, notes: '' },
    ],
  }),
}))

const mockUseFeature = vi.fn().mockReturnValue({ enabled: true, loading: false })
vi.mock('../../../shared/hooks/useFeature', () => ({
  useFeature: (...args: unknown[]) => mockUseFeature(...args),
}))

function wrapper({ children }: { children: React.ReactNode }) {
  return <MemoryRouter>{children}</MemoryRouter>
}

describe('BCMDashboardPage', () => {
  beforeEach(() => {
    mockUseFeature.mockReturnValue({ enabled: true, loading: false })
  })

  it('renders page title key', () => {
    render(<BCMDashboardPage />, { wrapper })
    expect(screen.getByText('bcm.dashboard.title')).toBeTruthy()
  })

  it('renders readiness score', () => {
    render(<BCMDashboardPage />, { wrapper })
    expect(screen.getByText('80')).toBeTruthy()
  })

  it('renders score criteria descriptions', () => {
    render(<BCMDashboardPage />, { wrapper })
    expect(screen.getByText('BIA vorhanden')).toBeTruthy()
    expect(screen.getByText('WAP getestet')).toBeTruthy()
  })

  it('renders BIA process count KPI', () => {
    render(<BCMDashboardPage />, { wrapper })
    expect(screen.getByText('3')).toBeTruthy()
  })

  it('renders quick links to BIA, WAP and contacts', () => {
    render(<BCMDashboardPage />, { wrapper })
    expect(screen.getByText('bcm.bia.title')).toBeTruthy()
    expect(screen.getByText('bcm.recoveryPlans.title')).toBeTruthy()
    expect(screen.getByText('bcm.emergencyContacts.title')).toBeTruthy()
  })

  it('renders PDF export button', () => {
    render(<BCMDashboardPage />, { wrapper })
    expect(screen.getByText('bcm.dashboard.exportPdf')).toBeTruthy()
  })

  it('renders 1 tested recovery plan', () => {
    render(<BCMDashboardPage />, { wrapper })
    expect(screen.getByText('1')).toBeTruthy()
  })

  it('shows warning when score is low', () => {
    render(<BCMDashboardPage />, { wrapper })
    // score is 80, warning only appears below 60 — should NOT be present
    expect(screen.queryByText('bcm.dashboard.warningTitle')).toBeNull()
  })

  it('shows PDF link when audit_pdf feature is enabled', () => {
    mockUseFeature.mockReturnValue({ enabled: true, loading: false })
    render(<BCMDashboardPage />, { wrapper })
    const link = screen.getByRole('link', { name: /bcm\.dashboard\.exportPdf/ })
    expect(link.getAttribute('href')).toBe('/api/v1/vaktcomply/bcm/report.pdf')
  })

  it('shows ProBadge on PDF button when audit_pdf feature is disabled', () => {
    mockUseFeature.mockReturnValue({ enabled: false, loading: false })
    render(<BCMDashboardPage />, { wrapper })
    expect(screen.getByText('Pro')).toBeTruthy()
    expect(screen.queryByRole('link', { name: /bcm\.dashboard\.exportPdf/ })).toBeNull()
  })
})
