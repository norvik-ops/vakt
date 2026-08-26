/**
 * K5-01/02/03 regression: the Statement of Applicability against the real wire
 * format, at the apiFetch seam.
 *
 * The SoA page had three separate defects that a hook-level test cannot see, all
 * from the same root — the TS interface was invented rather than derived from
 * policy.SoADedicatedEntry / policy.SoASummary:
 *   K5-01  the group filter read `group` (backend: `control_group`), so the table
 *          was empty for all four Annex-A tabs no matter how many of the 93
 *          measures were initialised;
 *   K5-02  every number in the header was `undefined`;
 *   K5-03  the PUT is a FULL overwrite (repository_soa_dedicated.go writes every
 *          column unconditionally), and the dialog sent field names the binder
 *          discards — so each save wrote the empty string over the
 *          justification, the exclusion reason, the evidence reference AND the
 *          notes, and nulled ck_control_id.
 *
 * K5-03 is why this file asserts a ROUND TRIP rather than a spelling: load a
 * fully populated entry, save it without editing anything, and require the PUT
 * body to still carry every value. A rename alone would not have caught the
 * `notes`/`ck_control_id` loss, because those fields were never in the dialog.
 */
import { describe, it, expect, vi, beforeEach } from 'vitest'
import { render, screen, fireEvent, waitFor, within } from '@testing-library/react'
import { MemoryRouter } from 'react-router-dom'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import SoAPage from './SoAPage'

vi.mock('react-i18next', () => ({
  useTranslation: () => ({ t: (k: string) => k }),
}))

vi.mock('../../../api/client', () => ({ apiFetch: vi.fn() }))
const { apiFetch } = await import('../../../api/client')
const mockApiFetch = vi.mocked(apiFetch)

// json tags of policy.SoADedicatedEntry. `notes` and `ck_control_id` are here on
// purpose: they are the two fields the dialog does not edit but MUST carry
// through, because the backend UPDATE overwrites them either way.
const ENTRY_ON_THE_WIRE = {
  id: 'e-1',
  org_id: 'org-1',
  version: 2,
  control_ref: 'A.5.1',
  control_name: 'Informationssicherheitsrichtlinien',
  control_group: '5',
  applicable: true,
  justification: 'Gilt vollständig, Richtlinie 4711 in Kraft',
  exclusion_reason: '',
  implementation_status: 'partial',
  manually_set: true,
  ck_control_id: 'ck-42',
  evidence_reference: 'DOC-2026-017',
  is_approved: false,
  notes: 'Review durch ISB im Q3',
  created_at: '2026-01-01T00:00:00Z',
  updated_at: '2026-02-01T00:00:00Z',
}

// An excluded entry WITHOUT an exclusion reason — the case the auditor warning
// and the approve block exist for.
const EXCLUDED_NO_REASON = {
  ...ENTRY_ON_THE_WIRE,
  id: 'e-2',
  control_ref: 'A.5.2',
  control_name: 'Rollen und Verantwortlichkeiten',
  applicable: false,
  justification: '',
  exclusion_reason: '',
  ck_control_id: null,
}

// json tags of policy.SoASummary.
const SUMMARY_ON_THE_WIRE = {
  version: 2,
  status: 'draft',
  applicable_count: 1,
  excluded_count: 1,
  implemented_count: 0,
  partial_count: 1,
  planned_count: 0,
  not_started_count: 0,
  implementation_pct: 0,
}

function stubApi(entries = [ENTRY_ON_THE_WIRE, EXCLUDED_NO_REASON]) {
  const writes: { path: string; method: string; body: unknown }[] = []
  mockApiFetch.mockImplementation((path: string, init?: RequestInit) => {
    const method = init?.method ?? 'GET'
    if (method === 'GET') {
      if (path.endsWith('/soa/summary')) return Promise.resolve(SUMMARY_ON_THE_WIRE as never)
      return Promise.resolve(entries as never)
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

/** Click the pencil in the row for `ref` — the only button in that row. */
function openEditFor(ref: string) {
  const row = screen.getByText(ref).closest('tr')
  if (!row) throw new Error(`no row for ${ref}`)
  fireEvent.click(within(row as HTMLElement).getByRole('button'))
}

beforeEach(() => { mockApiFetch.mockReset() })

describe('SoAPage — table grouping (K5-01)', () => {
  it('lists the A.5 entries on the default A5 tab', async () => {
    stubApi()
    renderPage()
    expect(await screen.findByText('Informationssicherheitsrichtlinien')).toBeTruthy()
    expect(screen.getByText('A.5.1')).toBeTruthy()
  })

  it('shows no rows on a tab whose control_group has no entries', async () => {
    stubApi()
    renderPage()
    await screen.findByText('A.5.1')
    fireEvent.click(screen.getByText('vaktcomply.soaPage.groupA7'))
    expect(screen.getByText('vaktcomply.soaPage.noEntries')).toBeTruthy()
  })
})

describe('SoAPage — header numbers (K5-02)', () => {
  it('derives the total from applicable_count + excluded_count', async () => {
    stubApi()
    renderPage()
    await screen.findByText('A.5.1')
    // 1 applicable + 1 excluded = 2. The old code read `summary.total`
    // (undefined) and fell back to the hardcoded 93.
    expect(screen.getByText('2')).toBeTruthy()
    expect(screen.queryByText('93')).toBeNull()
  })

  it('counts excluded entries without a reason and blocks approve', async () => {
    stubApi()
    renderPage()
    await screen.findByText('A.5.1')
    // status === 'draft' → the approve button renders; one excluded entry has no
    // reason → it must be disabled.
    const approve = screen.getByText('vaktcomply.soaPage.approveBtn').closest('button')
    expect(approve).toBeDisabled()
  })

  it('enables approve once every exclusion carries a reason', async () => {
    stubApi([ENTRY_ON_THE_WIRE, { ...EXCLUDED_NO_REASON, exclusion_reason: 'Kein Cloud-Betrieb' }])
    renderPage()
    await screen.findByText('A.5.1')
    const approve = screen.getByText('vaktcomply.soaPage.approveBtn').closest('button')
    expect(approve).not.toBeDisabled()
  })
})

describe('SoAPage — edit round trip (K5-03)', () => {
  it('carries every persisted field back in the PUT, including ones the dialog does not edit', async () => {
    const { writes } = stubApi()
    renderPage()
    await screen.findByText('A.5.1')

    openEditFor('A.5.1')
    // Save straight away — nothing edited, so nothing may be lost.
    fireEvent.click(await screen.findByText('common.save'))

    await waitFor(() => { expect(writes).toHaveLength(1) })
    const { path, method, body } = writes[0]
    expect(method).toBe('PUT')
    expect(path).toBe('/vaktcomply/soa/entries/A.5.1')

    const b = body as Record<string, unknown>
    expect(b.justification).toBe('Gilt vollständig, Richtlinie 4711 in Kraft')
    expect(b.evidence_reference).toBe('DOC-2026-017')
    // Not editable in the dialog, and therefore the fields the old code silently
    // blanked on every save:
    expect(b.notes).toBe('Review durch ISB im Q3')
    expect(b.ck_control_id).toBe('ck-42')
    // A save through the dialog is a manual decision — without this the nightly
    // evidence sync overwrites the status the user just chose.
    expect(b.manually_set).toBe(true)
    expect(b.implementation_status).toBe('partial')
    // None of the invented names may reappear.
    for (const dead of ['justification_included', 'justification_excluded', 'evidence_note', 'owner']) {
      expect(b).not.toHaveProperty(dead)
    }
  })

  it('offers only implementation_status values the backend accepts', async () => {
    stubApi()
    renderPage()
    await screen.findByText('A.5.1')
    openEditFor('A.5.1')
    await screen.findByText('common.save')
    // `in_progress` and `not_applicable` are rejected by
    // validate:"oneof=not_started planned partial implemented" with 422.
    expect(screen.queryByText('vaktcomply.soaPage.implInProgress')).toBeNull()
    expect(screen.queryByText('vaktcomply.soaPage.implNotApplicable')).toBeNull()
  })
})
