import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest'
import { render, screen, waitFor, renderHook } from '@testing-library/react'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { MemoryRouter } from 'react-router-dom'
import type { ReactNode } from 'react'
import PrivacyDesignPage from '../modules/vaktprivacy/pages/PrivacyDesignPage'
import { useHasEvidence, useHasVvt } from '../shared/components/GettingStartedChecklist'
import { fetchAllPages, fetchAllPaginated } from './fetchAllPages'
import { mockApiServer, type ApiServerHandle } from '../test-utils/apiServer'
import type { ApiResponse } from './contract'

/**
 * R1-W2C-01 and R1-11-D02 — the SHAPE_MISMATCH class.
 *
 * A hook or page declared the shape of a list response by hand and got it
 * wrong: an envelope read as an array, an array read as an envelope. Nothing
 * threw. The page rendered a permanent empty state, which looks like an empty
 * system rather than a defect, and the onboarding step stayed grey forever.
 *
 * Two things make these tests carry the claim rather than restate the fixture:
 *
 *   1. They stub `fetch`, so the real hook and the real apiFetch run. The
 *      suite's usual `vi.mock('../hooks/useX')` cannot reach this boundary —
 *      it replaces exactly the code that misread the body.
 *   2. Every fixture below is typed `ApiResponse<path, method>`, which resolves
 *      through generated.ts to the published OpenAPI contract. Writing a
 *      fixture in the shape the broken code expected is a compile error, so
 *      the test cannot be quietly "fixed" by adjusting the fixture to match a
 *      wrong reading.
 *
 * The residual limit is real and worth naming: openapi.yaml is hand-written and
 * lives in backend/. These types pin the frontend to the published contract,
 * not to the Go struct. For each endpoint here the two were compared by hand —
 * /vaktprivacy/vvt goes through pagination.Wrap (vaktprivacy/handler.go:93) and
 * /vaktcomply/evidence/auto returns a bare slice (evidence_auto/handler.go:110),
 * both as documented. During the sweep four OTHER endpoints turned out to have
 * a spec that disagrees with the handler while the frontend was right; those
 * are reported against backend/, not silently pinned here.
 */

const VVT_PAGE: ApiResponse<'/vaktprivacy/vvt', 'get'> = {
  data: [
    { id: 'v1', name: 'Bewerbermanagement' },
    { id: 'v2', name: 'Lohnabrechnung' },
  ],
  pagination: { page: 1, limit: 100, total: 2, total_pages: 1 },
}

const EVIDENCE: ApiResponse<'/vaktcomply/evidence/auto', 'get'> = [
  { id: 'e1', title: 'TLS-Konfiguration', auto_source_type: 'vaktscan' },
]

function wrapper({ children }: { children: ReactNode }) {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false, gcTime: 0 } } })
  return <QueryClientProvider client={client}>{children}</QueryClientProvider>
}

describe('GettingStartedChecklist list probes (R1-11-D02)', () => {
  let server: ApiServerHandle
  afterEach(() => { server.restore(); vi.unstubAllGlobals() })

  it('useHasEvidence sees the bare array /vaktcomply/evidence/auto actually returns', async () => {
    server = mockApiServer({ 'GET /vaktcomply/evidence/auto': { body: EVIDENCE } })
    const { result } = renderHook(() => useHasEvidence(), { wrapper })
    await waitFor(() => { expect(result.current.isSuccess).toBe(true) })
    // Before the fix this read `.count ?? .data.length` off an array — both
    // undefined — and answered false with evidence present.
    expect(result.current.data).toBe(true)
  })

  it('useHasEvidence stays false on an empty array', async () => {
    server = mockApiServer({ 'GET /vaktcomply/evidence/auto': { body: [] } })
    const { result } = renderHook(() => useHasEvidence(), { wrapper })
    await waitFor(() => { expect(result.current.isSuccess).toBe(true) })
    expect(result.current.data).toBe(false)
  })

  it('useHasVvt reads pagination.total, not the length of a one-row page', async () => {
    const oneOfMany: ApiResponse<'/vaktprivacy/vvt', 'get'> = {
      data: [{ id: 'v1', name: 'Bewerbermanagement' }],
      pagination: { page: 1, limit: 1, total: 42, total_pages: 42 },
    }
    server = mockApiServer({ 'GET /vaktprivacy/vvt': { body: oneOfMany } })
    const { result } = renderHook(() => useHasVvt(), { wrapper })
    await waitFor(() => { expect(result.current.isSuccess).toBe(true) })
    expect(result.current.data).toBe(true)
  })
})

describe('PrivacyDesignPage (R1-W2C-01)', () => {
  let server: ApiServerHandle

  beforeEach(() => {
    const summary: Record<string, number> = {
      total_activities: 2, with_assessment: 0, compliant: 0,
      partially: 0, not_assessed: 2, pending_count: 2, pct_compliant: 0,
    }
    server = mockApiServer({
      'GET /license': { body: { tier: 'pro', is_pro: true, features: ['vaktprivacy_advanced'] } },
      'GET /vaktprivacy/privacy-design/summary': { body: summary },
      'GET /vaktprivacy/vvt': { body: VVT_PAGE },
    })
  })
  afterEach(() => { server.restore(); vi.unstubAllGlobals() })

  it('lists the processing activities the server sent instead of a permanent empty state', async () => {
    render(
      <QueryClientProvider client={new QueryClient({ defaultOptions: { queries: { retry: false } } })}>
        <MemoryRouter>
          <PrivacyDesignPage />
        </MemoryRouter>
      </QueryClientProvider>,
    )

    // The names come from the envelope's `data` array. Reading the envelope as
    // an array made `vvtEntries.length` undefined, so this table was never
    // rendered and the empty state was shown with two records in the database.
    expect(await screen.findByText('Bewerbermanagement')).toBeTruthy()
    expect(screen.getByText('Lohnabrechnung')).toBeTruthy()
  })
})

describe('fetchAllPages — no silent truncation', () => {
  let server: ApiServerHandle
  afterEach(() => { server?.restore(); vi.unstubAllGlobals() })

  it('walks every page instead of trusting one over-large limit', async () => {
    // The server caps at 100 and DISCARDS a larger ?limit in favour of 25, so
    // a single ?limit=200 request returns a quarter of the rows and says so
    // only in pagination.total, which nobody read.
    server = mockApiServer({
      'GET /vaktprivacy/vvt': (req) => {
        const page = Number(new URL(req.url, 'http://x').searchParams.get('page') ?? '1')
        const all = Array.from({ length: 250 }, (_, i) => ({ id: `v${String(i)}`, name: `A${String(i)}` }))
        const slice = all.slice((page - 1) * 100, page * 100)
        return { body: { data: slice, pagination: { page, limit: 100, total: 250, total_pages: 3 } } }
      },
    })

    const res = await fetchAllPaginated<{ id: string; name: string }>('/vaktprivacy/vvt')
    expect(res.items).toHaveLength(250)
    expect(res.complete).toBe(true)
    expect(server.requests).toHaveLength(3)
    expect(server.requests[0].url).toContain('limit=100')
  })

  it('reports complete=false rather than truncating quietly at maxItems', async () => {
    const res = await fetchAllPages(
      (page) => Promise.resolve({
        items: Array.from({ length: 100 }, (_, i) => (page - 1) * 100 + i),
        total: 1000,
      }),
      { pageSize: 100, maxItems: 250 },
    )
    expect(res.items).toHaveLength(250)
    expect(res.complete).toBe(false)
    expect(res.total).toBe(1000)
  })

  it('stops on a short page even when the server over-reports total', async () => {
    let calls = 0
    const res = await fetchAllPages(
      () => { calls++; return Promise.resolve({ items: [1, 2], total: 9999 }) },
      { pageSize: 100 },
    )
    expect(calls).toBe(1)
    expect(res.items).toEqual([1, 2])
  })
})
