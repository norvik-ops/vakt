import { describe, it, expect, vi } from 'vitest'
import { render, screen } from '@testing-library/react'
import { MemoryRouter } from 'react-router-dom'

import ISO27001ChecklistPage from './ISO27001ChecklistPage'
import BSIGrundschutzPage from './BSIGrundschutzPage'

vi.mock('../hooks/useFrameworks', () => ({
  useFrameworks: () => ({ data: [] }),
  useDownloadSoAPDF: () => ({ mutate: vi.fn(), isPending: false }),
  useDownloadAuditPackage: () => ({ mutate: vi.fn(), isPending: false }),
}))

const ARCHIVED_LABELS = ['Vakt Scan', 'Vakt Vault', 'Vakt Aware', 'Vakt Privacy', 'Vakt HR']

function hrefs(): string[] {
  return screen.getAllByRole('link').map((a) => a.getAttribute('href') ?? '')
}

// Scan, Vault, Aware, Privacy und HR sind archiviert: jede Checklisten-Zeile verweist auf eine
// Seite in Vakt Comply.
describe('Checklisten verweisen nur auf Vakt Comply', () => {
  it('ISO 27001: alle 93 Controls zeigen auf /vaktcomply', () => {
    render(<MemoryRouter><ISO27001ChecklistPage /></MemoryRouter>)
    for (const label of ARCHIVED_LABELS) expect(screen.queryByText(label)).not.toBeInTheDocument()
    const controlLinks = hrefs().filter((h) => h.startsWith('/vaktcomply'))
    expect(controlLinks).toHaveLength(93)
    expect(hrefs().filter((h) => !h.startsWith('/vaktcomply'))).toEqual([])
  })

  it('ISO 27001: A.5.9 Inventar führt zum Asset-Inventar', () => {
    render(<MemoryRouter><ISO27001ChecklistPage /></MemoryRouter>)
    const row = screen.getByText('Inventar von IS-Assets').closest('div')?.parentElement
    expect(row?.querySelector('a')?.getAttribute('href')).toBe('/vaktcomply/assets')
  })

  it('BSI-Grundschutz: kein Baustein verweist auf ein archiviertes Modul', () => {
    render(<MemoryRouter><BSIGrundschutzPage /></MemoryRouter>)
    for (const label of ARCHIVED_LABELS) expect(screen.queryByText(label)).not.toBeInTheDocument()
    expect(hrefs().filter((h) => /^\/(vaktscan|vaktvault|vaktaware|vaktprivacy|vakthr)/.test(h))).toEqual([])
  })
})

describe('NIS2-Checkliste', () => {
  it('jede Anforderung verlinkt in Vakt Comply oder die Einstellungen', async () => {
    const { default: NIS2ChecklistPage } = await import('./NIS2ChecklistPage')
    render(<MemoryRouter><NIS2ChecklistPage /></MemoryRouter>)
    for (const label of ARCHIVED_LABELS) expect(screen.queryByText(label)).not.toBeInTheDocument()
    const reqLinks = hrefs().filter((h) => h !== '/vaktcomply/nis2-assistant')
    expect(reqLinks).toHaveLength(10)
    expect(reqLinks.filter((h) => !/^\/(vaktcomply|settings|account)(\/|$)/.test(h))).toEqual([])
  })
})
