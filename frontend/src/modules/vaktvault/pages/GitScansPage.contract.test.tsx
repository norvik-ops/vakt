/**
 * K5-11/12 regression: git secret scanning against the real wire format, at the
 * apiFetch seam.
 *
 * The sibling GitScansPage.test.tsx is the sharpest illustration of why a
 * hook-level test is not a contract test: it set the fixture to `result_count: 2`
 * and asserted `getByText('2 findings')`. Green — for a badge that could never
 * render, because vaktvault.GitScan emits `finding_count`. A security finding was
 * being suppressed in production while a test certified the opposite.
 *
 * K5-12 is worse than a blank field: `dismissed` does not exist on
 * vaktvault.ScanResult, and `!undefined === true`, so EVERY result counted as
 * active. Clicking Dismiss persisted status='dismissed' server-side, the list
 * reloaded, and the finding stayed in the red block — indistinguishable from a
 * failure. These tests drive that split from `status`, the real discriminator.
 */
import { describe, it, expect, vi, beforeEach } from 'vitest'
import { render, screen, fireEvent } from '@testing-library/react'
import { MemoryRouter } from 'react-router-dom'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import GitScansPage from './GitScansPage'

// Partial mock: ProGate needs the real FeatureLockedError class for its
// instanceof check, only apiFetch is swapped.
vi.mock('../../../api/client', async (importOriginal) => ({
  ...(await importOriginal<typeof import('../../../api/client')>()),
  apiFetch: vi.fn(),
}))
const { apiFetch } = await import('../../../api/client')
const mockApiFetch = vi.mocked(apiFetch)

// json tags of vaktvault.GitScan.
const SCAN_ON_THE_WIRE = {
  id: 'scan-1',
  org_id: 'org-1',
  repo_url: 'https://github.com/acme/backend',
  branch: 'main',
  status: 'completed',
  finding_count: 2,
  open_count: 1,
  dismissed_count: 1,
  scanned_at: '2026-01-15T10:05:00Z',
  created_at: '2026-01-15T10:00:00Z',
}

// json tags of vaktvault.ScanResult — one open, one dismissed.
const RESULTS_ON_THE_WIRE = [
  {
    id: 'r-1',
    org_id: 'org-1',
    scan_id: 'scan-1',
    repo_url: 'https://github.com/acme/backend',
    commit_hash: 'deadbee',
    file_path: 'config/settings.py',
    line_number: 42,
    pattern_name: 'AWS_ACCESS_KEY',
    match_preview: 'AKIA…7Q4B',
    severity: 'critical',
    status: 'open',
    dismiss_count: 0,
    created_at: '2026-01-15T10:05:00Z',
  },
  {
    id: 'r-2',
    org_id: 'org-1',
    scan_id: 'scan-1',
    repo_url: 'https://github.com/acme/backend',
    file_path: 'test/fixtures.py',
    line_number: 7,
    pattern_name: 'GENERIC_SECRET',
    match_preview: 'test…1234',
    severity: 'low',
    status: 'dismissed',
    dismiss_reason: 'Testfixture',
    dismiss_count: 1,
    created_at: '2026-01-15T10:05:00Z',
  },
]

function stubApi() {
  mockApiFetch.mockImplementation((path: string, init?: RequestInit) => {
    if ((init?.method ?? 'GET') !== 'GET') return Promise.resolve(undefined as never)
    if (path.endsWith('/results')) return Promise.resolve(RESULTS_ON_THE_WIRE as never)
    return Promise.resolve([SCAN_ON_THE_WIRE] as never)
  })
}

function renderPage() {
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false, gcTime: 0 }, mutations: { retry: false } },
  })
  return render(
    <QueryClientProvider client={client}>
      <MemoryRouter><GitScansPage /></MemoryRouter>
    </QueryClientProvider>,
  )
}

beforeEach(() => { mockApiFetch.mockReset() })

describe('GitScansPage — findings badge (K5-11)', () => {
  it('shows the badge for a scan the backend reports finding_count for', async () => {
    stubApi()
    renderPage()
    expect(await screen.findByText('2 findings')).toBeTruthy()
  })

  it('shows no badge when the scan found nothing', async () => {
    mockApiFetch.mockImplementation((path: string) =>
      path.endsWith('/results')
        ? Promise.resolve([] as never)
        : Promise.resolve([{ ...SCAN_ON_THE_WIRE, finding_count: 0, open_count: 0, dismissed_count: 0 }] as never))
    renderPage()
    await screen.findByText(/github\.com\/acme\/backend/)
    expect(screen.queryByText(/finding/)).toBeNull()
  })
})

describe('GitScansPage — dismissed split (K5-12)', () => {
  it('separates open from dismissed results by status, and shows type and preview', async () => {
    stubApi()
    renderPage()
    fireEvent.click(await screen.findByText(/github\.com\/acme\/backend/))

    // The open finding is rendered with its real pattern_name and match_preview.
    expect(await screen.findByText('AWS_ACCESS_KEY')).toBeTruthy()
    expect(screen.getByText('AKIA…7Q4B')).toBeTruthy()
    expect(screen.getByText('config/settings.py:42')).toBeTruthy()

    // The dismissed one is NOT in the red block — under the old `!r.dismissed`
    // test it was, because the field does not exist and `!undefined` is true.
    expect(screen.queryByText('GENERIC_SECRET')).toBeNull()
    // …and it is counted as dismissed instead (1 → singular key).
    expect(screen.getByText('1 dismissed finding')).toBeTruthy()
  })
})
