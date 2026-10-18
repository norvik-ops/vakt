import { describe, it, expect, vi } from 'vitest'
import { render, screen } from '@testing-library/react'
import { MemoryRouter } from 'react-router-dom'

import ISO27001ChecklistPage from './ISO27001ChecklistPage'
import { useEnabledModules } from '../../../shared/moduleVisibility'

vi.mock('../hooks/useFrameworks', () => ({
  useFrameworks: () => ({ data: [] }),
  useDownloadSoAPDF: () => ({ mutate: vi.fn(), isPending: false }),
  useDownloadAuditPackage: () => ({ mutate: vi.fn(), isPending: false }),
}))

function renderWith(...modules: string[]) {
  useEnabledModules.setState({ modules: new Set(modules) })
  render(
    <MemoryRouter>
      <ISO27001ChecklistPage />
    </MemoryRouter>,
  )
}

// R1-KW-15: Controls abgeschalteter Module verweisen auf Comply, nicht auf eine fehlende Seite.
describe('ISO27001ChecklistPage — abgeschaltete Module', () => {
  it('zeigt nur mit Comply keine Verweise auf Scan, Vault, Aware oder Privacy', () => {
    renderWith('vaktcomply')
    for (const label of ['Vakt Scan', 'Vakt Vault', 'Vakt Aware', 'Vakt Privacy']) {
      expect(screen.queryByText(label)).not.toBeInTheDocument()
    }
    const hrefs = screen.getAllByRole('link').map((a) => a.getAttribute('href') ?? '')
    expect(hrefs.filter((h) => /^\/(vaktscan|vaktvault|vaktaware|vaktprivacy)/.test(h))).toEqual([])
  })

  it('verweist mit eingeschaltetem Scan weiter auf Vakt Scan', () => {
    renderWith('vaktcomply', 'vaktscan')
    expect(screen.getAllByText('Vakt Scan').length).toBeGreaterThan(0)
  })
})
