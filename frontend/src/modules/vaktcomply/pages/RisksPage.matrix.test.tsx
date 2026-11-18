/**
 * UX-28 (Kernweg 8): Das Risikoregister hatte zwei Matrizen — „Risikomatrix" in der
 * Listenansicht und „Heatmap" als eigene Ansicht, verschieden gezeichnet. Jetzt gibt
 * es eine (RiskHeatmap) über der Liste; der Zellklick listet die Risiken der Zelle.
 * UX-27: Restrisiko auf der Karte und in der Matrix umschaltbar.
 */
import { describe, it, expect, vi, beforeEach } from 'vitest'
import { render, screen, fireEvent, within } from '@testing-library/react'
import { MemoryRouter } from 'react-router-dom'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import RisksPage from './RisksPage'
import { signInAs } from '../../../test-utils/signInAs'

const BASE = {
  org_id: 'o-1', description: '', category: 'IT', owner: '', status: 'open', treatment: 'mitigate',
  created_at: '2026-11-17T10:00:00Z', updated_at: '2026-11-17T10:00:00Z',
}
const RISKS = [
  { ...BASE, id: 'r-1', title: 'Ransomware', likelihood: 4, impact: 5, risk_score: 20, residual_likelihood: 2, residual_impact: 2, residual_score: 4 },
  { ...BASE, id: 'r-2', title: 'Stromausfall', likelihood: 2, impact: 5, risk_score: 10 },
]

vi.mock('../../../api/client', () => ({
  apiFetch: (path: string) => Promise.resolve(
    path.startsWith('/vaktcomply/risks')
      ? { data: RISKS, pagination: { page: 1, limit: 25, total: 2, total_pages: 1 } }
      : [],
  ),
  registerUnauthorizedHandler: vi.fn(),
}))

function renderPage() {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } })
  return render(
    <QueryClientProvider client={client}>
      <MemoryRouter initialEntries={['/vaktcomply/risks']}>
        <RisksPage />
      </MemoryRouter>
    </QueryClientProvider>,
  )
}

function cell(likelihood: string, impact: string): HTMLElement {
  return screen.getByRole('button', { name: new RegExp(`^Wahrscheinlichkeit ${likelihood}, Auswirkung ${impact},`) })
}

describe('RisksPage — eine Matrix (UX-28)', () => {
  beforeEach(() => { signInAs('Admin') })

  it('genau eine Matrix, kein Umschalter Liste/Heatmap', async () => {
    renderPage()
    await screen.findByText('Ransomware')
    expect(screen.getAllByRole('heading', { name: /Risikomatrix/ })).toHaveLength(1)
    expect(screen.queryByRole('button', { name: 'Heatmap' })).toBeNull()
    expect(screen.queryByText(/Risiko-Heatmap/)).toBeNull()
  })

  it('Legende „Stufe (von–bis)" steht im Elternelement der Überschrift (Kernweg 8, R1-KW-17)', async () => {
    renderPage()
    await screen.findByText('Ransomware')
    // Wie der Kernweg: direkt das Elternelement der Überschrift (xpath=..).
    const parent = screen.getByRole('heading', { name: /Risikomatrix/ }).parentElement!
    expect(parent).toHaveTextContent(/Hoch \(10–14\)/)
    expect(parent).toHaveTextContent(/Kritisch \(15–25\)/)
  })

  it('Achsen wie RiskHeatmap: Stufennamen statt Zahlen', async () => {
    renderPage()
    await screen.findByText('Ransomware')
    expect(screen.getByText('Sehr wahrscheinlich')).toBeInTheDocument()
    expect(screen.getAllByText('Kritisch').length).toBeGreaterThan(0)
  })

  it('Klick auf eine Zelle listet ihre Risiken', async () => {
    renderPage()
    await screen.findByText('Ransomware')
    fireEvent.click(cell('Wahrscheinlich', 'Kritisch'))
    const panel = screen.getByRole('dialog')
    expect(within(panel).getByText('Ransomware')).toBeInTheDocument()
  })

  it('Matrix umschaltbar auf Restrisiko; Risiken ohne Restbewertung werden genannt', async () => {
    renderPage()
    await screen.findByText('Ransomware')
    expect(cell('Wahrscheinlich', 'Kritisch')).toHaveAccessibleName(/1 Risiken$/)
    fireEvent.click(screen.getByRole('button', { name: 'Restrisiko' }))
    expect(screen.getByRole('button', { name: 'Restrisiko' })).toHaveAttribute('aria-pressed', 'true')
    expect(cell('Wahrscheinlich', 'Kritisch')).toHaveAccessibleName(/0 Risiken$/)
    expect(cell('Unwahrscheinlich', 'Gering')).toHaveAccessibleName(/1 Risiken$/)
    expect(screen.getByTestId('heatmap-unassessed')).toHaveTextContent('1 Risiko ohne Restrisiko-Bewertung')
  })
})

describe('RisksPage — Restrisiko auf der Karte (UX-27)', () => {
  beforeEach(() => { signInAs('Admin') })

  it('Karte zeigt Ausgangs- und Restwert; ohne Restbewertung nur den Ausgangswert', async () => {
    renderPage()
    await screen.findByText('Ransomware')
    const badges = screen.getAllByTestId('risk-card-residual')
    expect(badges).toHaveLength(1)
    expect(badges[0]).toHaveTextContent('Rest 4')
    expect(badges[0]).toHaveAttribute('title', 'Restrisiko nach Behandlung')
    expect(screen.getByText('Score 20')).toBeInTheDocument()
    expect(screen.getByText('Score 10')).toBeInTheDocument()
  })
})
