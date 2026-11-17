/**
 * Eine Quelle für den Umsetzungsstatus (Kernweg 6, UX-Durchgang 2026-11-17).
 *
 * Vorher hatte die SoA einen eigenen, von Hand gepflegten Status: A.5.2 stand auf
 * der Kontrollseite auf „In Bearbeitung“, in der SoA auf „Nicht begonnen“. Jetzt
 * liefert das Backend den Status der Kontrolle (policy/soa_status.go), und die
 * Seite
 *   - zeigt ihn mit denselben Labels wie die Kontrollseite (controlDetailPage.status*),
 *   - verlinkt die Plakette auf die Kontrollseite,
 *   - speichert eine Statusänderung im Dialog an der Kontrolle (PATCH
 *     /vaktcomply/controls/:id) und nie im SoA-Eintrag,
 *   - sperrt die Auswahl, wenn eine Freigabe nötig ist (wie die Kontrollseite).
 */
import { describe, it, expect, vi, beforeEach } from 'vitest'
import { render, screen, fireEvent, waitFor, within } from '@testing-library/react'
import { MemoryRouter } from 'react-router-dom'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import SoAPage from './SoAPage'

vi.mock('react-i18next', () => ({
  useTranslation: () => ({ t: (k: string) => k }),
}))

vi.mock('../../../api/client', () => ({ apiFetch: vi.fn(), registerUnauthorizedHandler: vi.fn() }))
const { apiFetch } = await import('../../../api/client')
const mockApiFetch = vi.mocked(apiFetch)

const { useAuthStore } = await import('../../../shared/stores/auth')

// json tags of policy.SoADedicatedEntry, as the backend now sends them.
const BASE = {
  org_id: 'org-1',
  version: 1,
  control_group: '5',
  applicable: true,
  justification: '',
  exclusion_reason: '',
  manually_set: false,
  ck_control_id: null,
  evidence_reference: '',
  is_approved: false,
  notes: '',
  created_at: '2026-01-01T00:00:00Z',
  updated_at: '2026-01-01T00:00:00Z',
}
const ENTRIES = [
  { ...BASE, id: 'e-1', control_ref: 'A.5.1', control_name: 'Richtlinien zur Informationssicherheit', implementation_status: 'implemented', status_source: 'control', linked_control_id: 'ck-1', linked_framework_id: 'fw-1' },
  { ...BASE, id: 'e-2', control_ref: 'A.5.2', control_name: 'Rollen und Verantwortlichkeiten', implementation_status: 'partial', status_source: 'control', linked_control_id: 'ck-2', linked_framework_id: 'fw-1' },
  { ...BASE, id: 'e-3', control_ref: 'A.5.3', control_name: 'Aufgabentrennung', implementation_status: 'not_started', status_source: 'control', linked_control_id: 'ck-3', linked_framework_id: 'fw-1' },
  { ...BASE, id: 'e-4', control_ref: 'A.5.4', control_name: 'Verantwortlichkeiten der Leitung', implementation_status: 'planned', status_source: 'snapshot', linked_control_id: null, linked_framework_id: null },
]
const SUMMARY = {
  version: 1, status: 'draft', applicable_count: 4, excluded_count: 0,
  implemented_count: 1, partial_count: 1, planned_count: 1, not_started_count: 1, implementation_pct: 37.5,
}
// json tags of policy.Control for GET /vaktcomply/controls/ck-2: one piece of
// evidence, no manual status → effective status "partial" (In Bearbeitung).
const CONTROL_A52 = {
  id: 'ck-2', framework_id: 'fw-1', control_id: 'A.5.2', title: 'Rollen und Verantwortlichkeiten',
  status: 'partial', not_applicable: false, not_applicable_reason: '', manual_status: '', owner: 'ISB',
}

function stubApi(opts: { approvalRequired?: boolean; summary?: typeof SUMMARY } = {}) {
  const writes: { path: string; method: string; body: unknown }[] = []
  mockApiFetch.mockImplementation((path: string, init?: RequestInit) => {
    const method = init?.method ?? 'GET'
    if (method === 'GET') {
      if (path.endsWith('/soa/summary')) return Promise.resolve((opts.summary ?? SUMMARY) as never)
      if (path.endsWith('/soa/entries')) return Promise.resolve(ENTRIES as never)
      if (path.endsWith('/org/approval-setting')) return Promise.resolve({ approval_required: opts.approvalRequired ?? false } as never)
      if (path === '/vaktcomply/controls/ck-2') return Promise.resolve(CONTROL_A52 as never)
      return Promise.reject(new Error(`unexpected GET ${path}`))
    }
    writes.push({ path, method, body: init?.body ? JSON.parse(init.body as string) : null })
    return Promise.resolve(undefined as never)
  })
  return { writes }
}

function renderPage() {
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false, gcTime: 0 }, mutations: { retry: false } },
  })
  return render(
    <QueryClientProvider client={client}>
      <MemoryRouter><SoAPage /></MemoryRouter>
    </QueryClientProvider>,
  )
}

function row(ref: string): HTMLElement {
  const r = screen.getByText(ref).closest('tr')
  if (!r) throw new Error(`no row for ${ref}`)
  return r
}

beforeEach(() => {
  mockApiFetch.mockReset()
  useAuthStore.setState({ user: null })
})

describe('SoAPage — Status der Kontrolle', () => {
  it('zeigt dieselben Labels wie die Kontrollseite', async () => {
    stubApi()
    renderPage()
    await screen.findByText('A.5.1')
    expect(within(row('A.5.1')).getByTestId('soa-status').textContent).toBe('vaktcomply.controlDetailPage.statusImplemented')
    expect(within(row('A.5.2')).getByTestId('soa-status').textContent).toBe('vaktcomply.controlDetailPage.statusInProgress')
    expect(within(row('A.5.3')).getByTestId('soa-status').textContent).toBe('vaktcomply.controlDetailPage.statusMissing')
    // Nur alte freigegebene Schnappschüsse tragen noch „Geplant“.
    expect(within(row('A.5.4')).getByTestId('soa-status').textContent).toBe('vaktcomply.soaPage.implPlanned')
    // Die alten SoA-eigenen Begriffe tauchen nicht mehr auf.
    expect(screen.queryByText('vaktcomply.soaPage.implNotStarted')).toBeNull()
    expect(screen.queryByText('vaktcomply.soaPage.implPartial')).toBeNull()
  })

  it('verlinkt die Plakette auf die Kontrollseite', async () => {
    stubApi()
    renderPage()
    await screen.findByText('A.5.2')
    const link = within(row('A.5.2')).getByRole('link')
    expect(link.getAttribute('href')).toBe('/vaktcomply/controls/ck-2?frameworkId=fw-1')
    // Ohne verknüpfte Kontrolle gibt es keinen Link.
    expect(within(row('A.5.4')).queryByRole('link')).toBeNull()
  })

  it('speichert eine Statusänderung an der Kontrolle, nicht im SoA-Eintrag', async () => {
    const { writes } = stubApi()
    renderPage()
    await screen.findByText('A.5.2')
    fireEvent.click(within(row('A.5.2')).getByRole('button'))

    const trigger = await screen.findByTestId('soa-control-status')
    await waitFor(() => { expect(trigger).not.toBeDisabled() })
    expect(trigger.textContent).toContain('vaktcomply.controlDetailPage.statusInProgress')
    fireEvent.click(trigger)
    fireEvent.click(screen.getByRole('option', { name: 'vaktcomply.controlDetailPage.statusImplemented' }))
    fireEvent.click(screen.getByText('common.save'))

    await waitFor(() => { expect(writes).toHaveLength(2) })
    expect(writes[0].method).toBe('PATCH')
    expect(writes[0].path).toBe('/vaktcomply/controls/ck-2')
    expect(writes[0].body).toEqual({
      not_applicable: false, reason: '', manual_status: 'implemented', owner: 'ISB',
    })
    expect(writes[1].method).toBe('PUT')
    expect(writes[1].path).toBe('/vaktcomply/soa/entries/A.5.2')
    expect(writes[1].body).not.toHaveProperty('implementation_status')
  })

  it('schreibt die Kontrolle nicht, wenn der Status unverändert bleibt', async () => {
    const { writes } = stubApi()
    renderPage()
    await screen.findByText('A.5.2')
    fireEvent.click(within(row('A.5.2')).getByRole('button'))
    const trigger = await screen.findByTestId('soa-control-status')
    await waitFor(() => { expect(trigger).not.toBeDisabled() })
    fireEvent.click(screen.getByText('common.save'))
    await waitFor(() => { expect(writes).toHaveLength(1) })
    expect(writes[0].method).toBe('PUT')
  })

  it('sperrt die Auswahl, wenn eine Freigabe nötig ist (kein Admin)', async () => {
    useAuthStore.setState({ user: { id: 'u-1', email: 'e@x.test', roles: ['Editor'] } as never })
    stubApi({ approvalRequired: true })
    renderPage()
    await screen.findByText('A.5.2')
    fireEvent.click(within(row('A.5.2')).getByRole('button'))
    const hint = await screen.findByTestId('soa-status-hint')
    await waitFor(() => { expect(hint.textContent).toContain('vaktcomply.soaPage.statusApprovalRequired') })
    expect(screen.getByTestId('soa-control-status')).toBeDisabled()
  })

  it('weist ohne verknüpfte Kontrolle darauf hin', async () => {
    stubApi()
    renderPage()
    await screen.findByText('A.5.4')
    fireEvent.click(within(row('A.5.4')).getByRole('button'))
    const hint = await screen.findByTestId('soa-status-hint')
    expect(hint.textContent).toContain('vaktcomply.soaPage.statusNoControl')
    expect(screen.getByTestId('soa-control-status')).toBeDisabled()
  })

  it('erklärt bei einer freigegebenen Version, dass der Status eingefroren ist', async () => {
    stubApi({ summary: { ...SUMMARY, status: 'approved' } })
    renderPage()
    expect(await screen.findByTestId('soa-snapshot-hint')).toBeTruthy()
  })
})
