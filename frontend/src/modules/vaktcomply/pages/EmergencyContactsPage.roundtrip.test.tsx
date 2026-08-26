/**
 * K5-09 round-trip regression: 24/7 availability must survive a save.
 *
 * WHY THIS FILE EXISTS AND WHY IT DOES NOT MOCK THE HOOK. The sibling
 * EmergencyContactsPage.test.tsx mocks `useEmergencyContacts`, so it never
 * crosses the FE↔BE boundary — it proved only that the page renders whatever the
 * frontend hands itself. That is exactly how `available_247` (a name the backend
 * never sends or binds) stayed green for a full release: the badge never showed
 * in production and the checkbox reset the flag to false on every save, while the
 * suite was green.
 *
 * These tests mock `apiFetch` instead — the one seam where the wire format is
 * real. The GET returns a BACKEND-SHAPED contact (`available_24_7`, the json tag
 * of bcm.EmergencyContact) and the assertion is on the PUT body: the flag has to
 * still be true after a save that did not touch it. That is a round trip, not a
 * spelling check, and it fails if the field name drifts in either direction.
 */
import { describe, it, expect, vi, beforeEach } from 'vitest'
import { render, screen, fireEvent, waitFor } from '@testing-library/react'
import { MemoryRouter } from 'react-router-dom'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import EmergencyContactsPage from './EmergencyContactsPage'

vi.mock('react-i18next', () => ({
  useTranslation: () => ({ t: (k: string) => k }),
}))

vi.mock('../../../api/client', () => ({ apiFetch: vi.fn() }))
const { apiFetch } = await import('../../../api/client')
const mockApiFetch = vi.mocked(apiFetch)

// Exactly what bcm.EmergencyContact serialises to.
const CONTACT_ON_THE_WIRE = {
  id: 'ec-1',
  org_id: 'org-1',
  name: 'Max Mustermann',
  role: 'CISO',
  phone: '+49 228 12345',
  email: 'max@example.com',
  escalation_level: 1,
  available_24_7: true,
  notes: 'Rufbereitschaft',
  created_at: '2026-01-01T00:00:00Z',
  updated_at: '2026-01-01T00:00:00Z',
}

function renderPage() {
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false, gcTime: 0 }, mutations: { retry: false } },
  })
  return render(
    <QueryClientProvider client={client}>
      <MemoryRouter><EmergencyContactsPage /></MemoryRouter>
    </QueryClientProvider>,
  )
}

/** Resolve GETs with the wire fixture, record every write, resolve writes. */
function stubApi(): { writes: { path: string; body: unknown }[] } {
  const writes: { path: string; body: unknown }[] = []
  mockApiFetch.mockImplementation((path: string, init?: RequestInit) => {
    if (!init?.method || init.method === 'GET') {
      return Promise.resolve([CONTACT_ON_THE_WIRE] as never)
    }
    writes.push({ path, body: JSON.parse(init.body as string) })
    return Promise.resolve(CONTACT_ON_THE_WIRE as never)
  })
  return { writes }
}

beforeEach(() => { mockApiFetch.mockReset() })

describe('EmergencyContactsPage — 24/7 flag round trip (K5-09)', () => {
  it('renders the 24/7 badge for a contact the backend marks available_24_7', async () => {
    stubApi()
    renderPage()
    // The badge text is a literal, not a translation key.
    expect(await screen.findByText('24/7')).toBeTruthy()
  })

  it('keeps available_24_7 = true in the PUT body when the switch is untouched', async () => {
    const { writes } = stubApi()
    renderPage()
    await screen.findByText('Max Mustermann')

    fireEvent.click(screen.getAllByRole('button')[1]) // pencil / edit
    await screen.findByText('bcm.emergencyContacts.edit')
    fireEvent.click(screen.getByText('common.save'))

    await waitFor(() => { expect(writes).toHaveLength(1) })
    const body = writes[0].body as Record<string, unknown>
    expect(writes[0].path).toBe('/vaktcomply/bcm/emergency-contacts/ec-1')
    // The point of the test: the flag survived. Under the old `available_247`
    // spelling this is `undefined` — a silent reset to false on the server.
    expect(body.available_24_7).toBe(true)
    expect(body).not.toHaveProperty('available_247')
  })

  it('sends available_24_7 = true when the switch is turned on for a new contact', async () => {
    const { writes } = stubApi()
    renderPage()
    await screen.findByText('Max Mustermann')

    fireEvent.click(screen.getByText('bcm.emergencyContacts.new'))
    fireEvent.change(screen.getAllByRole('textbox')[0], { target: { value: 'Neue Person' } })
    fireEvent.click(screen.getByLabelText('bcm.emergencyContacts.available247'))
    fireEvent.click(screen.getByText('common.add'))

    await waitFor(() => { expect(writes).toHaveLength(1) })
    const body = writes[0].body as Record<string, unknown>
    expect(body.available_24_7).toBe(true)
  })
})
