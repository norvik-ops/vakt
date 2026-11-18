/**
 * UX-26 (Kernweg 8): Das Risiko-Detail fragte Strategie, Verantwortlichen,
 * Maßnahmen/Plan und Restrisiko je zweimal ab — in „Behandlung", „Risikobehandlung"
 * und „Residualrisiko-Berechnung", mit drei Speichern-Knöpfen. Jetzt steht jede
 * Angabe einmal da; ein Speichern schreibt alle zugehörigen Spalten.
 * UX-27: Das Restrisiko steht oben neben dem Ausgangswert.
 */
import { describe, it, expect, vi, beforeEach } from 'vitest'
import { render, screen, fireEvent, waitFor, within } from '@testing-library/react'
import { MemoryRouter, Route, Routes } from 'react-router-dom'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import RiskDetailPage from './RiskDetailPage'
import { signInAs } from '../../../test-utils/signInAs'

const RISK = {
  id: 'r-1', org_id: 'o-1', title: 'Ransomware verschlüsselt den Fileserver', description: '', category: 'Cyber',
  likelihood: 4, impact: 5, risk_score: 20, owner: 'CISO', status: 'open', treatment: 'mitigate',
  treatment_notes: '', treatment_option: 'mitigate', treatment_plan: 'Offline-Backups täglich',
  treatment_owner: '', treatment_due_date: null, treatment_status: 'pending',
  residual_likelihood: 2, residual_impact: 2, residual_score: 4,
  created_at: '2026-11-17T10:00:00Z', updated_at: '2026-11-17T10:00:00Z',
}

const calls: { path: string; method: string; body: unknown }[] = []

vi.mock('../../../api/client', () => ({
  apiFetch: (path: string, init?: { method?: string; body?: string }) => {
    calls.push({ path, method: init?.method ?? 'GET', body: init?.body ? JSON.parse(init.body) : undefined })
    if (path === '/vaktcomply/risks/r-1') return Promise.resolve(RISK)
    return Promise.resolve([])
  },
  registerUnauthorizedHandler: vi.fn(),
}))

function renderPage() {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } })
  return render(
    <QueryClientProvider client={client}>
      <MemoryRouter initialEntries={['/vaktcomply/risks/r-1']}>
        <Routes>
          <Route path="/vaktcomply/risks/:id" element={<RiskDetailPage />} />
        </Routes>
      </MemoryRouter>
    </QueryClientProvider>,
  )
}

describe('RiskDetailPage — jede Angabe nur einmal (UX-26)', () => {
  beforeEach(() => { calls.length = 0; signInAs('Admin') })

  it('Verantwortlicher, Strategie, Maßnahmen und Restrisiko-Eingaben stehen je einmal da', async () => {
    renderPage()
    await screen.findByRole('heading', { name: RISK.title, level: 1 })
    expect(screen.getAllByText('Verantwortlicher')).toHaveLength(1)
    expect(screen.getAllByRole('radiogroup', { name: 'Behandlungsstrategie' })).toHaveLength(1)
    expect(screen.queryByText('Strategie', { exact: true })).toBeNull()
    expect(screen.getAllByRole('textbox', { name: 'Behandlungsplan' })).toHaveLength(1)
    expect(screen.queryByText('Maßnahmen', { exact: true })).toBeNull()
    // Restrisiko: nur die zwei Schieberegler, keine Zahlenfelder „Netto-…"/„Brutto-…" mehr.
    expect(screen.getAllByRole('slider')).toHaveLength(2)
    expect(screen.queryByText(/Netto-Wahrscheinlichkeit|Brutto-Wahrscheinlichkeit/)).toBeNull()
    // Registerstatus und Umsetzungsstand sind verschiedene Dinge — beide da, klar benannt.
    expect(screen.getByRole('combobox', { name: 'Risikostatus' })).toBeInTheDocument()
    expect(screen.getByRole('combobox', { name: 'Umsetzungsstand der Maßnahmen' })).toBeInTheDocument()
  })

  it('Restrisiko steht oben neben dem Ausgangswert (UX-27)', async () => {
    renderPage()
    const summary = await screen.findByTestId('risk-residual-summary')
    expect(summary).toHaveTextContent(/^Restrisiko.*4 \/ 25$/)
    expect(screen.getByText(/Risiko-Score/).parentElement).toHaveTextContent(/Risiko-Score\s*20 \/ 25/)
  })

  it('ein Speichern schreibt Grunddaten, Behandlung und Restrisiko — Paare gleich', async () => {
    renderPage()
    await screen.findByRole('heading', { name: RISK.title, level: 1 })
    fireEvent.change(screen.getByRole('textbox', { name: 'Behandlungsplan' }), { target: { value: 'EDR auf allen Clients' } })
    fireEvent.click(screen.getByRole('radio', { name: /Übertragen/ }))
    fireEvent.click(screen.getByRole('button', { name: 'Behandlung speichern' }))

    await waitFor(() => { expect(calls.filter((c) => c.method === 'PATCH')).toHaveLength(3) })
    const patches = calls.filter((c) => c.method === 'PATCH')
    expect(patches.map((c) => c.path)).toEqual([
      '/vaktcomply/risks/r-1',
      '/vaktcomply/risks/r-1/treatment',
      '/vaktcomply/risks/r-1/residual',
    ])
    const [update, treatment, residual] = patches.map((c) => c.body as Record<string, unknown>)
    expect(update.treatment).toBe('transfer')
    expect(treatment.treatment_option).toBe('transfer')
    expect(update.treatment_notes).toBe('EDR auf allen Clients')
    expect(treatment.treatment_plan).toBe('EDR auf allen Clients')
    expect(update.owner).toBe('CISO')
    expect(treatment.treatment_owner).toBe('CISO')
    expect(residual).toMatchObject({ inherent_likelihood: 4, inherent_impact: 5, residual_likelihood: 2, residual_impact: 2 })
  })

  it('Kopfzeile und Behandlungsblock speichern dasselbe', async () => {
    renderPage()
    await screen.findByRole('heading', { name: RISK.title, level: 1 })
    const header = screen.getByRole('button', { name: 'Speichern' })
    expect(header).toBeDisabled()
    fireEvent.change(screen.getByRole('textbox', { name: 'Verantwortlicher' }), { target: { value: 'IT-Leitung' } })
    expect(header).toBeEnabled()
    expect(screen.getByRole('button', { name: 'Behandlung speichern' })).toBeEnabled()
    fireEvent.click(header)
    await waitFor(() => { expect(calls.filter((c) => c.method === 'PATCH')).toHaveLength(3) })
    const treatment = calls.find((c) => c.path.endsWith('/treatment'))!.body as Record<string, unknown>
    expect(treatment.treatment_owner).toBe('IT-Leitung')
    expect(within(document.body).queryByText('Werte speichern')).toBeNull()
  })
})
