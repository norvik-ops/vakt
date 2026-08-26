/**
 * K5-14 regression, at the apiFetch seam: the target count on a group row.
 *
 * The defect: `{group.target_count ?? 0} targets`. vaktaware.TargetGroup emits
 * id / org_id / name / source / created_at — there is no target_count on the wire
 * and no tg_ column for it, so `?? 0` turned a missing field into a confident
 * "0 targets" on every row of the list, for every group, forever. A group with
 * 4 000 recipients read "0 targets"; the phishing operator's only pre-send sanity
 * check said the group was empty.
 *
 * Why this file mocks apiFetch and not the hook: every fixture in the 587-test
 * baseline is fed to a mocked hook, so it proves the component renders whatever
 * the test hands it — a shape the test itself invented. That cannot fail on a
 * field-name drift, which is why this class survived a green suite. Here the
 * fixtures below are the Go json tags verbatim and they enter through
 * apiFetch, i.e. the same seam the real drift sits on.
 *
 * Reverting the fix (`{group.target_count ?? 0} targets` back in
 * TargetGroupsPage.tsx, `target_count?: number` back in ../types.ts) makes both
 * tests fail: the collapsed row shows "0 targets" and the expanded row keeps
 * showing "0 targets" instead of the 3 that /targets returned.
 *
 * Named consequence of the fix (REV-K5 R4): collapsed rows now show NO number at
 * all. The count needs the per-group /targets response, and loading that for every
 * row of a collapsed list would be an N+1 against the API. A wrong number is worse
 * than no number, so this is deliberate — and the first test pins it, so nobody
 * "restores" the count from a field the backend does not send.
 */
import { describe, it, expect, vi, beforeEach } from 'vitest'
import { render, screen, fireEvent, waitFor } from '@testing-library/react'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import TargetGroupsPage from './TargetGroupsPage'

vi.mock('react-i18next', () => ({
  useTranslation: () => ({ t: (k: string) => k, i18n: { language: 'de' } }),
}))

vi.mock('../../../api/client', async (importOriginal) => ({
  ...(await importOriginal<typeof import('../../../api/client')>()),
  apiFetch: vi.fn(),
}))
const { apiFetch } = await import('../../../api/client')
const mockApiFetch = vi.mocked(apiFetch)

// json tags of vaktaware.TargetGroup — the whole struct. Note what is NOT here:
// no target_count. That absence is the finding.
const GROUP_ON_THE_WIRE = {
  id: 'tg-1',
  org_id: 'org-1',
  name: 'Alle Mitarbeitenden',
  source: 'manual',
  created_at: '2026-01-01T00:00:00Z',
}

// json tags of vaktaware.Target.
const TARGETS_ON_THE_WIRE = [
  { id: 't-1', email: 'a@example.com', first_name: 'Ada', last_name: 'Lovelace', department: 'IT' },
  { id: 't-2', email: 'b@example.com', first_name: 'Bob', last_name: 'Baumeister', department: 'Bau' },
  { id: 't-3', email: 'c@example.com', first_name: null, last_name: null, department: null },
]

function stubApi() {
  const calls: string[] = []
  mockApiFetch.mockImplementation((path: string, init?: RequestInit) => {
    calls.push(`${init?.method ?? 'GET'} ${path}`)
    if (path.endsWith('/targets')) return Promise.resolve(TARGETS_ON_THE_WIRE as never)
    if (path.endsWith('/vaktaware/groups')) return Promise.resolve([GROUP_ON_THE_WIRE] as never)
    return Promise.resolve([] as never)
  })
  return { calls }
}

function renderPage() {
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false, gcTime: 0 }, mutations: { retry: false } },
  })
  return render(
    <QueryClientProvider client={client}>
      <TargetGroupsPage />
    </QueryClientProvider>,
  )
}

beforeEach(() => { mockApiFetch.mockReset() })

describe('TargetGroupsPage — target count (K5-14)', () => {
  it('does not invent a "0 targets" count for a group the backend sent no count for', async () => {
    const { calls } = stubApi()
    renderPage()
    await screen.findByText('Alle Mitarbeitenden')

    // The old code rendered "0 targets" here, unconditionally, from a field that
    // is not on the wire.
    expect(screen.queryByText(/0 targets/)).toBeNull()
    expect(screen.queryByText(/targets$/)).toBeNull()
    // …and it did so without ever asking for the targets: collapsed, only the
    // group list is fetched (useTargets is enabled: Boolean(groupId), called with
    // '' while collapsed). This is the assertion that keeps the fix from being
    // "fixed" into an N+1 over every row.
    expect(calls).toEqual(['GET /vaktaware/groups'])
  })

  it('shows the real count once /targets has answered, and it matches the rows', async () => {
    stubApi()
    renderPage()
    const row = await screen.findByText('Alle Mitarbeitenden')

    fireEvent.click(row)

    // 3, because the wire said 3 — not because any field claimed a number.
    await waitFor(() => { expect(screen.getByText('3 targets')).toBeInTheDocument() })
    expect(mockApiFetch).toHaveBeenCalledWith('/vaktaware/groups/tg-1/targets')
    // The number and the table cannot disagree: both come from the same response.
    expect(screen.getByText('a@example.com')).toBeInTheDocument()
    expect(screen.getByText('c@example.com')).toBeInTheDocument()
    expect(screen.queryByText(/0 targets/)).toBeNull()
  })
})
