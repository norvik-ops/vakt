/**
 * K5-08 regression, at the apiFetch seam: the BIA tiles on the BCM dashboard.
 *
 * The defect: the frontend's BIASummary declared total / high_critical /
 * avg_rto_hours / avg_rpo_hours. bcm.BIASummary emits total_processes /
 * critical_count / shortest_rto_hours / klasse_breakdown (models.go:50-55) — ZERO
 * field overlap. Both tiles read "—" on every load, for every org, and the
 * "N hochkritisch" badge could not appear even with a business process at
 * Schutzbedarfsklasse 3.
 *
 * Two things make this one worth an apiFetch-level test specifically:
 *  1. `openapi.yaml` carried the FRONTEND's invention verbatim, all four fields
 *     with `required:` — so generating types from the spec would have certified the
 *     fiction. The spec is corrected in the same commit; the oracle used here is
 *     the Go json tag (ADR-0080).
 *  2. There is no average RTO anywhere in the backend. The aggregate query is
 *     sqlc-generated and frozen (ADR-0078) and computes the SHORTEST RTO, so the
 *     tile shows and LABELS the shortest. The old label was a second, quieter
 *     defect: had the field name matched, the number would still have been
 *     mislabelled "Ø RTO".
 *
 * BCMDashboardPage.test.tsx next to this file mocks the four hooks — it proves the
 * component renders what the test hands it, which is exactly why this drift
 * survived a green suite. Here everything, including useFeature's /license call,
 * arrives through apiFetch.
 *
 * Reverting the fix (`summary?.total` / `avg_rto_hours` back in the page,
 * total/high_critical/avg_rto_hours/avg_rpo_hours back in ../types.ts) fails both
 * tests: the tiles fall back to "—" and the badge disappears.
 */
import { describe, it, expect, vi, beforeEach } from 'vitest'
import { render, screen } from '@testing-library/react'
import { MemoryRouter } from 'react-router-dom'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import BCMDashboardPage from './BCMDashboardPage'

vi.mock('react-i18next', () => ({
  useTranslation: () => ({ t: (k: string) => k, i18n: { language: 'de' } }),
}))

vi.mock('../../../api/client', async (importOriginal) => ({
  ...(await importOriginal<typeof import('../../../api/client')>()),
  apiFetch: vi.fn(),
}))
const { apiFetch } = await import('../../../api/client')
const mockApiFetch = vi.mocked(apiFetch)

// json tags of bcm.BIASummary, all four, nothing else. `klasse_breakdown` is a
// Go map[int]int, so its keys arrive stringified.
const BIA_SUMMARY_ON_THE_WIRE = {
  total_processes: 12,
  critical_count: 3,
  shortest_rto_hours: 4,
  klasse_breakdown: { '1': 5, '2': 4, '3': 3 },
}

function stubApi() {
  mockApiFetch.mockImplementation((path: string) => {
    if (path.includes('/bia/summary')) return Promise.resolve(BIA_SUMMARY_ON_THE_WIRE as never)
    if (path.includes('/readiness-score')) {
      return Promise.resolve({ score: 62, criteria: [] } as never)
    }
    if (path === '/license') {
      return Promise.resolve({ tier: 'pro', is_pro: true, features: ['audit_pdf'] } as never)
    }
    return Promise.resolve([] as never)   // recovery-plans, emergency-contacts
  })
}

function renderPage() {
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false, gcTime: 0 }, mutations: { retry: false } },
  })
  return render(
    <QueryClientProvider client={client}>
      <MemoryRouter><BCMDashboardPage /></MemoryRouter>
    </QueryClientProvider>,
  )
}

beforeEach(() => { mockApiFetch.mockReset() })

describe('BCMDashboardPage — BIA tiles (K5-08)', () => {
  it('renders the process count and the critical badge the backend sent', async () => {
    stubApi()
    renderPage()

    // total_processes, not `total` — which was undefined and rendered as "—".
    expect(await screen.findByText('12')).toBeInTheDocument()
    // critical_count, not `high_critical`: `(undefined ?? 0) > 0` is false, so the
    // badge was unreachable regardless of how critical the org's processes were.
    expect(await screen.findByText(/3 bcm\.dashboard\.highCritical/)).toBeInTheDocument()
  })

  it('renders the shortest RTO, and labels it shortest rather than average', async () => {
    stubApi()
    renderPage()

    expect(await screen.findByText('4h')).toBeInTheDocument()
    // The backend computes no average — the sqlc aggregate returns the shortest
    // RTO. Labelling it "Ø RTO" would be a second defect on top of the field name.
    expect(screen.getByText('bcm.dashboard.shortestRto')).toBeInTheDocument()
    expect(screen.queryByText('bcm.dashboard.avgRto')).toBeNull()
  })
})
