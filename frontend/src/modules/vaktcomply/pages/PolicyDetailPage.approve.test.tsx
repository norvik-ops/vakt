import { describe, it, expect, vi, beforeEach } from 'vitest'
import { render, screen, fireEvent } from '@testing-library/react'
import { MemoryRouter, Route, Routes } from 'react-router-dom'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import PolicyDetailPage from './PolicyDetailPage'
import { useAuthStore } from '../../../shared/stores/auth'
import type { Policy } from '../types'

// UX-20: Eine Freigabe hieß „Status Aktiv auswählen und Speichern"; wer freigegeben
// hat, stand nirgends. Jetzt gibt es den Knopf „Freigeben" für Schreibrollen;
// einen Freigeber zeigt die Seite bewusst nicht an (siehe unten).
// UX-21: nur noch ein Prüftermin auf der Seite. UX-22: Version und Revision
// heißen überall gleich.

const POLICY_ID = '11111111-2222-3333-4444-555555555555'
const mutate = vi.fn()
let policy: Policy

function makePolicy(over: Partial<Policy> = {}): Policy {
  return {
    id: POLICY_ID,
    org_id: 'org-1',
    title: 'Passwortrichtlinie',
    status: 'draft',
    version: '1.0',
    version_num: 2,
    version_note: '',
    last_updated_by: '',
    created_at: '2026-01-01T00:00:00Z',
    updated_at: '2026-01-02T00:00:00Z',
    ...over,
  }
}

vi.mock('../hooks/usePolicies', () => ({
  usePolicy: () => ({ data: policy, isLoading: false, isError: false }),
  useUpdatePolicy: () => ({ mutate, isPending: false }),
}))

vi.mock('../components/PolicyVersionHistory', () => ({
  default: () => null,
}))

function signIn(roles: string[]) {
  useAuthStore.setState({
    user: { id: 'u-1', email: 'erika@example.test', display_name: 'Erika Muster', roles },
    hydrating: false,
  })
}

function renderPage() {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } })
  return render(
    <QueryClientProvider client={client}>
      <MemoryRouter initialEntries={[`/vaktcomply/policies/${POLICY_ID}`]}>
        <Routes>
          <Route path="/vaktcomply/policies/:id" element={<PolicyDetailPage />} />
        </Routes>
      </MemoryRouter>
    </QueryClientProvider>,
  )
}

beforeEach(() => {
  mutate.mockReset()
  policy = makePolicy()
})

describe('PolicyDetailPage — Freigeben (UX-20)', () => {
  it('zeigt Schreibrollen den Knopf und setzt Status und Notiz über den Speicherweg', () => {
    signIn(['SecurityAnalyst'])
    renderPage()

    fireEvent.click(screen.getByRole('button', { name: 'Freigeben' }))

    expect(mutate).toHaveBeenCalledTimes(1)
    expect(mutate.mock.calls[0][0]).toMatchObject({ status: 'active', version_note: 'Freigegeben' })
  })

  // Sicherheitsreview 2026-11-18 (integrity/forgeable-approval-attribution): Der
  // Freigeber darf nie aus Werten kommen, die der Client schickt oder kennt — weder
  // aus dem Freitext „Geändert von" (last_updated_by) noch aus dem angemeldeten
  // Namen im Frontend. Die einzige Quelle sind die Server-Felder approved_by_name
  // und approved_at (Migration 272), siehe die Tests weiter unten.
  it('schreibt den angemeldeten Namen nicht als Freigeber in die Anfrage', () => {
    signIn(['Admin'])
    renderPage()

    fireEvent.click(screen.getByRole('button', { name: 'Freigeben' }))

    expect(JSON.stringify(mutate.mock.calls[0][0])).not.toContain('Erika Muster')
    expect(JSON.stringify(mutate.mock.calls[0][0])).not.toContain('erika@example.test')
  })

  it('zeigt einen vom Client gesetzten Namen nicht als Freigeber an', () => {
    policy = makePolicy({ status: 'active', last_updated_by: 'Gefälschte Geschäftsführung', reviewed_at: '2026-03-04T10:00:00Z' })
    signIn(['Viewer'])
    renderPage()

    expect(screen.queryByText(/freigegeben von|bestätigt von/i)).toBeNull()
    for (const el of screen.queryAllByText(/Gefälschte Geschäftsführung/)) {
      expect(el.textContent).not.toMatch(/freigegeben|bestätigt/i)
    }
  })

  // Migration 272: Der Server liefert den Freigeber aus der angemeldeten Identität
  // (approved_by_name, approved_at). Nur diese Felder füllen „Freigegeben von";
  // der Freitext bleibt daneben als „Bearbeitet von" stehen.
  it('zeigt den Freigeber aus den Server-Feldern, den Freitext nur als „Bearbeitet von"', () => {
    policy = makePolicy({
      status: 'active',
      last_updated_by: 'Gefälschte Geschäftsführung',
      approved_by: '99999999-9999-9999-9999-999999999999',
      approved_by_name: 'Alice Freigabe',
      approved_at: '2026-11-18T09:30:00Z',
    })
    signIn(['Viewer'])
    renderPage()

    const line = screen.getByTestId('policy-approved-by')
    expect(line.textContent).toMatch(/^Freigegeben von Alice Freigabe am /)
    expect(line.textContent).toContain('18.11.2026')
    expect(line.textContent).not.toContain('Gefälschte Geschäftsführung')
    expect(screen.getByText(/Bearbeitet von:/).textContent).toContain('Gefälschte Geschäftsführung')
  })

  it('zeigt keinen Freigeber, wenn der Server keinen liefert', () => {
    policy = makePolicy({ status: 'active', approved_by: null, approved_by_name: null, approved_at: null })
    signIn(['Admin'])
    renderPage()

    expect(screen.queryByTestId('policy-approved-by')).toBeNull()
    expect(screen.queryByText(/freigegeben von/i)).toBeNull()
  })

  it.each([['Viewer'], ['AuditorReadOnly'], ['InternalAuditor']])('versteckt den Knopf für %s', (role) => {
    signIn([role])
    renderPage()

    expect(screen.queryByRole('button', { name: 'Freigeben' })).toBeNull()
  })

  it('bietet keine Freigabe für eine schon aktive Richtlinie an', () => {
    policy = makePolicy({ status: 'active' })
    signIn(['Admin'])
    renderPage()

    expect(screen.queryByRole('button', { name: 'Freigeben' })).toBeNull()
  })
})

describe('PolicyDetailPage — ein Prüftermin (UX-21)', () => {
  it('zeigt „Nächste Überprüfung" genau einmal und schreibt beide Spalten gleich', () => {
    policy = makePolicy({ review_date: '2026-09-01T00:00:00Z', next_review_due: '2026-12-01' })
    signIn(['Admin'])
    renderPage()

    expect(screen.getAllByText('Nächste Überprüfung')).toHaveLength(1)
    expect(screen.queryByText('Review-Datum')).toBeNull()
    const input = screen.getByLabelText('Nächste Überprüfung')
    expect(input).toHaveValue('2026-09-01')

    fireEvent.change(input, { target: { value: '2027-01-15' } })
    fireEvent.click(screen.getByRole('button', { name: 'Speichern' }))
    expect(mutate.mock.calls[0][0]).toMatchObject({ review_date: '2027-01-15', next_review_due: '2027-01-15' })
  })

  it('liest einen alten Datensatz, der nur next_review_due kennt', () => {
    policy = makePolicy({ next_review_due: '2026-12-01' })
    signIn(['Admin'])
    renderPage()

    expect(screen.getByLabelText('Nächste Überprüfung')).toHaveValue('2026-12-01')
  })
})

describe('PolicyDetailPage — Versionsanzeige (UX-22)', () => {
  it('nennt Version und Revision mit denselben Begriffen wie Liste und Historie', () => {
    policy = makePolicy({ category: 'IT-Sicherheit' })
    signIn(['Viewer'])
    renderPage()

    expect(screen.getByText('Version 1.0 · Revision 2 · IT-Sicherheit')).toBeInTheDocument()
    expect(screen.queryByText(/\bv1\.0\b/)).toBeNull()
  })
})
