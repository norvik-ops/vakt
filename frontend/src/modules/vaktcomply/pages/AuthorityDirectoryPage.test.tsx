import { render, screen } from '@testing-library/react'
import { describe, it, expect, vi } from 'vitest'
import AuthorityDirectoryPage from './AuthorityDirectoryPage'

vi.mock('../hooks/useOrgSector', () => ({
  useAuthorities: () => ({
    data: [
      {
        name: 'BSI',
        portal: 'https://www.bsi.bund.de/meldestelle',
        phone: '+49 228 99 9582-5777',
        submit_note: 'Meldung über das BSI-Meldeportal oder per Telefon.',
      },
      {
        name: 'BaFin',
        portal: 'https://www.bafin.de/meldung',
        phone: '+49 228 4108-0',
        submit_note: 'Meldung per BAIT-Meldeformular.',
      },
    ],
    isLoading: false,
  }),
  useOrgSector: () => ({
    data: { sector: 'energy', federal_state: 'Bayern' },
  }),
}))

describe('AuthorityDirectoryPage', () => {
  it('renders configured sector', () => {
    render(<AuthorityDirectoryPage />)
    expect(screen.getByTestId('sector-display')).toHaveTextContent('Energie')
  })

  it('renders authority list', () => {
    render(<AuthorityDirectoryPage />)
    const list = screen.getByTestId('authority-list')
    expect(list).toBeInTheDocument()
    expect(list.querySelectorAll('.rounded-lg, [class*="card"]').length).toBeGreaterThanOrEqual(0)
    expect(screen.getByText('BSI')).toBeInTheDocument()
    expect(screen.getByText('BaFin')).toBeInTheDocument()
  })

  it('renders authority portal links', () => {
    render(<AuthorityDirectoryPage />)
    const bsiLink = screen.getByTestId('authority-portal-BSI')
    expect(bsiLink).toHaveAttribute('href', 'https://www.bsi.bund.de/meldestelle')
  })

  it('shows federal_state in GDPR note', () => {
    render(<AuthorityDirectoryPage />)
    expect(screen.getByText(/Bayern/)).toBeInTheDocument()
  })
})
