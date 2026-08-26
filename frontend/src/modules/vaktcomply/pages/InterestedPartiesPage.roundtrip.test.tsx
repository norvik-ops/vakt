/**
 * K5-07 regression: interested parties (ISO 27001 clause 4.2) against the real
 * wire format, at the apiFetch seam.
 *
 * Three defects out of one root — the TS interface was invented, not derived from
 * vaktcomply.InterestedParty / CreateInterestedPartyInput:
 *   create   the default category was 'external', which violates both the `oneof`
 *            tag and the ck_interested_parties CHECK → 422 for every attempt,
 *            with no selectable category that would have worked;
 *   display  the "needs" and "monitoring" columns read fields the backend never
 *            sends, so both were blank for every row — including the six seeded
 *            default entries, which ARE populated server-side;
 *   edit     openEdit() filled the form from those non-existent fields and the PUT
 *            sent them back, so every save blanked requirements and concerns.
 *
 * The edit case is why this asserts a ROUND TRIP: load a populated party, save
 * without touching anything, and require the PUT to still carry the values.
 */
import { describe, it, expect, vi, beforeEach } from 'vitest'
import { render, screen, fireEvent, waitFor, within } from '@testing-library/react'
import { MemoryRouter } from 'react-router-dom'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import InterestedPartiesPage from './InterestedPartiesPage'

vi.mock('react-i18next', () => ({
  useTranslation: () => ({ t: (k: string) => k }),
}))

vi.mock('../../../api/client', () => ({ apiFetch: vi.fn() }))
const { apiFetch } = await import('../../../api/client')
const mockApiFetch = vi.mocked(apiFetch)

// json tags of vaktcomply.InterestedParty.
const PARTY_ON_THE_WIRE = {
  id: 'ip-1',
  org_id: 'org-1',
  name: 'Bundesnetzagentur',
  category: 'regulator',
  requirements: 'NIS2UmsuCG §31 Meldepflichten binnen 24 h',
  concerns: 'Nachweisbare Reaktionsfähigkeit',
  review_date: '2026-09-30',
  review_overdue: false,
  is_system_default: true,
  created_at: '2026-01-01T00:00:00Z',
  updated_at: '2026-01-01T00:00:00Z',
}

function stubApi(parties = [PARTY_ON_THE_WIRE]) {
  const writes: { path: string; method: string; body: unknown }[] = []
  mockApiFetch.mockImplementation((path: string, init?: RequestInit) => {
    const method = init?.method ?? 'GET'
    if (method === 'GET') return Promise.resolve(parties as never)
    writes.push({ path, method, body: init?.body ? JSON.parse(init.body as string) : null })
    return Promise.resolve(PARTY_ON_THE_WIRE as never)
  })
  return { writes }
}

function renderPage() {
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false, gcTime: 0 }, mutations: { retry: false } },
  })
  return render(
    <QueryClientProvider client={client}>
      <MemoryRouter><InterestedPartiesPage /></MemoryRouter>
    </QueryClientProvider>,
  )
}

beforeEach(() => { mockApiFetch.mockReset() })

describe('InterestedPartiesPage — display (K5-07 part 2)', () => {
  it('shows the requirements column and the review date the backend sent', async () => {
    stubApi()
    renderPage()
    expect(await screen.findByText('Bundesnetzagentur')).toBeTruthy()
    expect(screen.getByText('NIS2UmsuCG §31 Meldepflichten binnen 24 h')).toBeTruthy()
    expect(screen.getByText('2026-09-30')).toBeTruthy()
  })

  it('flags an overdue review', async () => {
    stubApi([{ ...PARTY_ON_THE_WIRE, review_overdue: true }])
    renderPage()
    await screen.findByText('Bundesnetzagentur')
    expect(screen.getByText('vaktcomply.interestedParties.badgeOverdue')).toBeTruthy()
  })
})

describe('InterestedPartiesPage — create (K5-07 part 1)', () => {
  it('defaults to a category the backend accepts', async () => {
    const { writes } = stubApi([])
    renderPage()
    await screen.findByText('vaktcomply.interestedParties.emptyTitle')

    fireEvent.click(screen.getAllByText('vaktcomply.interestedParties.addParty')[0])
    fireEvent.change(screen.getAllByRole('textbox')[0], { target: { value: 'Betriebsrat' } })
    fireEvent.click(screen.getByText('common.save'))

    await waitFor(() => { expect(writes).toHaveLength(1) })
    const b = writes[0].body as Record<string, unknown>
    expect(writes[0].method).toBe('POST')
    // The whole point: 'external' (the old default) is rejected by
    // validate:"oneof=customer regulator employee shareholder supplier insurer
    // it_provider other" AND by the DDL CHECK.
    const ALLOWED = ['customer', 'regulator', 'employee', 'shareholder', 'supplier', 'insurer', 'it_provider', 'other']
    expect(ALLOWED).toContain(b.category)
    for (const dead of ['description', 'needs_and_expectations', 'relevant_requirements', 'monitoring_frequency', 'owner']) {
      expect(b).not.toHaveProperty(dead)
    }
  })

  it('offers only categories the backend accepts', async () => {
    stubApi([])
    renderPage()
    await screen.findByText('vaktcomply.interestedParties.emptyTitle')
    fireEvent.click(screen.getAllByText('vaktcomply.interestedParties.addParty')[0])
    // 'internal', 'external' and 'regulatory' are outside the oneof.
    expect(screen.queryByText('vaktcomply.interestedParties.categoryInternal')).toBeNull()
    expect(screen.queryByText('vaktcomply.interestedParties.categoryExternal')).toBeNull()
  })
})

describe('InterestedPartiesPage — edit round trip (K5-07 part 3)', () => {
  it('does not blank requirements and concerns on a no-op save', async () => {
    const { writes } = stubApi()
    renderPage()
    const row = (await screen.findByText('Bundesnetzagentur')).closest('tr')
    fireEvent.click(within(row as HTMLElement).getAllByRole('button')[0]) // pencil
    fireEvent.click(await screen.findByText('common.save'))

    await waitFor(() => { expect(writes).toHaveLength(1) })
    const b = writes[0].body as Record<string, unknown>
    expect(writes[0].path).toBe('/vaktcomply/interested-parties/ip-1')
    expect(b.requirements).toBe('NIS2UmsuCG §31 Meldepflichten binnen 24 h')
    expect(b.concerns).toBe('Nachweisbare Reaktionsfähigkeit')
    expect(b.review_date).toBe('2026-09-30')
    expect(b.category).toBe('regulator')
  })
})
