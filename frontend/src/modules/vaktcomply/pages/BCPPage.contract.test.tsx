/**
 * ESK-12 regression: die vier BSI-200-4-Felder des BCP-Plans an der
 * apiFetch-Naht.
 *
 * Der Defekt: PR #75 hat rto_hours/rpo_hours/schutzbedarfsklasse/last_tested_at
 * in die Lese-Queries aufgenommen, aber kein Schreibweg setzte sie. Die API
 * lieferte fuer JEDEN Plan jeder Organisation 72/24/2/null — Werte, die wie
 * kuratierte BSI-200-4-Angaben aussehen. Die Seite zeigte keines der vier Felder
 * an und schickte keines mit; ein Betreiber konnte den Zustand also weder sehen
 * noch aendern.
 *
 * Warum an der apiFetch-Naht und nicht am Hook: der Ausgangsdefekt lebte genau
 * dort, wo Request-Body und Antwortform aufeinandertreffen. Ein Test, der den
 * Hook mockt, haette den Body nie gesehen (siehe die Begruendung in
 * scripts/check_fe_be_fields.py).
 *
 * Die Schutzbedarfsklasse ist ein Radix-Select und unter jsdom nicht per
 * fireEvent bedienbar — geprueft werden deshalb der ANGEZEIGTE Zustand und die
 * Zahlenfelder, die es sind.
 */
import { describe, it, expect, vi, beforeEach } from 'vitest'
import { render, screen, fireEvent, waitFor } from '@testing-library/react'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import BCPPage from './BCPPage'

vi.mock('react-i18next', () => ({
  useTranslation: () => ({ t: (k: string) => k, i18n: { language: 'de' } }),
}))

vi.mock('../../../api/client', async (importOriginal) => ({
  ...(await importOriginal<typeof import('../../../api/client')>()),
  apiFetch: vi.fn(),
}))
const { apiFetch } = await import('../../../api/client')
const mockApiFetch = vi.mocked(apiFetch)

// json tags von bcm.BCPPlan — 13 Felder, wie die Go-Antwort sie schickt.
const PLAN_UNSET = {
  id: 'plan-1',
  org_id: 'org-1',
  title: 'IT-Notfallhandbuch',
  scope: 'Kritische Systeme',
  version: '1.0',
  status: 'active',
  owner: 'IT-Leitung',
  rto_hours: null,
  rpo_hours: null,
  schutzbedarfsklasse: null,
  last_tested_at: null,
  created_at: '2026-01-01T00:00:00Z',
  updated_at: '2026-01-01T00:00:00Z',
}

const PLAN_SET = {
  ...PLAN_UNSET,
  id: 'plan-2',
  title: 'RZ-Ausfall',
  rto_hours: 4,
  rpo_hours: 1,
  schutzbedarfsklasse: 3,
  last_tested_at: '2026-05-04',
}

function stubApi(plans: unknown[]) {
  const writes: { path: string; method: string; body: Record<string, unknown> | null }[] = []
  mockApiFetch.mockImplementation((path: string, init?: RequestInit) => {
    const method = init?.method ?? 'GET'
    if (method === 'GET') return Promise.resolve(plans as never)
    writes.push({
      path,
      method,
      body: init?.body ? (JSON.parse(init.body as string) as Record<string, unknown>) : null,
    })
    return Promise.resolve(plans[0] as never)
  })
  return writes
}

function renderPage() {
  const qc = new QueryClient({ defaultOptions: { queries: { retry: false } } })
  return render(
    <QueryClientProvider client={qc}>
      <BCPPage />
    </QueryClientProvider>,
  )
}

describe('BCPPage — BSI-200-4-Felder (ESK-12)', () => {
  beforeEach(() => {
    mockApiFetch.mockReset()
  })

  it('zeigt gesetzte Werte an, statt sie zu verschweigen', async () => {
    stubApi([PLAN_SET])
    renderPage()
    await screen.findByText('RZ-Ausfall')

    // Die konkreten Werte des Plans, nicht irgendeine Zahl.
    expect(screen.getByText('4')).toBeInTheDocument()
    expect(screen.getByText('1')).toBeInTheDocument()
    expect(screen.getByText('3 — bcm.bia.klasse3')).toBeInTheDocument()
    expect(screen.getByText('2026-05-04')).toBeInTheDocument()
  })

  it('weist einen nicht festgelegten Wert als solchen aus statt als Zahl', async () => {
    stubApi([PLAN_UNSET])
    renderPage()
    await screen.findByText('IT-Notfallhandbuch')

    // Vier Felder, vier Mal "nicht festgelegt" — und nirgends 72/24/2, die
    // Migrations-Defaults, die der Defekt als Planangabe ausgab.
    expect(screen.getAllByText('bcp.notSet')).toHaveLength(4)
    expect(screen.queryByText('72')).not.toBeInTheDocument()
    expect(screen.queryByText('24')).not.toBeInTheDocument()
    expect(screen.queryByText('2 — bcm.bia.klasse2')).not.toBeInTheDocument()
  })

  it('schickt eingegebene RTO/RPO im Request-Body mit', async () => {
    const writes = stubApi([])
    renderPage()

    fireEvent.click(screen.getByText('bcp.newPlan'))
    fireEvent.change(screen.getByPlaceholderText('bcp.planTitlePlaceholder'), {
      target: { value: 'Neuer Plan' },
    })
    fireEvent.change(screen.getByLabelText('RTO (h)'), { target: { value: '8' } })
    fireEvent.change(screen.getByLabelText('RPO (h)'), { target: { value: '2' } })
    fireEvent.click(screen.getByText('common.add'))

    await waitFor(() => { expect(writes).toHaveLength(1) })
    expect(writes[0].method).toBe('POST')
    expect(writes[0].body).toMatchObject({ title: 'Neuer Plan', rto_hours: 8, rpo_hours: 2 })
  })

  it('schickt ein leeres Zahlenfeld als null, nicht als 0', async () => {
    const writes = stubApi([])
    renderPage()

    fireEvent.click(screen.getByText('bcp.newPlan'))
    fireEvent.change(screen.getByPlaceholderText('bcp.planTitlePlaceholder'), {
      target: { value: 'Ohne Zahlen' },
    })
    fireEvent.click(screen.getByText('common.add'))

    await waitFor(() => { expect(writes).toHaveLength(1) })
    // 0 wuerde die API mit 422 ablehnen (rto_hours >= 1) und saehe in der
    // Anzeige aus wie "sofort" — null heisst "noch nicht festgelegt".
    expect(writes[0].body).toMatchObject({
      rto_hours: null,
      rpo_hours: null,
      schutzbedarfsklasse: null,
    })
  })

  it('traegt bestehende Werte beim Bearbeiten mit, statt sie zu leeren', async () => {
    const writes = stubApi([PLAN_SET])
    renderPage()
    await screen.findByText('RZ-Ausfall')

    // Der Bearbeiten-Knopf ist ein Icon-Button ohne Text; er ist der erste
    // Knopf in der Kartenkopfzeile.
    const editButton = document.querySelectorAll('.h-7.w-7')[0]
    fireEvent.click(editButton)
    fireEvent.click(screen.getByText('common.save'))

    await waitFor(() => { expect(writes).toHaveLength(1) })
    expect(writes[0].method).toBe('PATCH')
    // Das PATCH mergt (REV-ESK12 B1): ein weggelassenes Feld bleibt, wie es
    // ist. Der Dialog schickt die Werte trotzdem mit — sonst zeigte er den
    // gespeicherten Zustand nicht an, und ein geleertes Feld koennte nicht als
    // ausdrueckliches `null` beim Backend ankommen.
    expect(writes[0].body).toMatchObject({
      rto_hours: 4,
      rpo_hours: 1,
      schutzbedarfsklasse: 3,
      // Hier stand fest `status: 'draft'` — jede Bearbeitung eines aktiven
      // Plans setzte ihn auf Entwurf zurueck.
      status: 'active',
    })
    // last_tested_at ist abgeleitet und darf nicht im Body stehen.
    expect(writes[0].body).not.toHaveProperty('last_tested_at')
  })

  it('schickt ein geleertes Feld als ausdrueckliches null, damit Loeschen moeglich bleibt', async () => {
    // Die Gegenprobe zur mergenden PATCH-Semantik: wenn ein FEHLENDES Feld
    // nichts mehr aendert, muss der Weg zum Loeschen ueber ein ausdrueckliches
    // `null` fuehren — sonst kaeme der Nutzer aus einem einmal gesetzten Wert
    // nicht mehr heraus. Der Schluessel muss also im Body stehen UND null sein;
    // `undefined` verschwaende beim JSON.stringify und hiesse "unveraendert".
    const writes = stubApi([PLAN_SET])
    renderPage()
    await screen.findByText('RZ-Ausfall')

    fireEvent.click(document.querySelectorAll('.h-7.w-7')[0])
    fireEvent.change(screen.getByLabelText('RTO (h)'), { target: { value: '' } })
    fireEvent.click(screen.getByText('common.save'))

    await waitFor(() => { expect(writes).toHaveLength(1) })
    expect(writes[0].method).toBe('PATCH')
    expect(Object.keys(writes[0].body ?? {})).toContain('rto_hours')
    expect(writes[0].body?.rto_hours).toBeNull()
    // Das unberuehrte Nachbarfeld geht unveraendert mit, nicht als null.
    expect(writes[0].body?.rpo_hours).toBe(1)
  })
})
