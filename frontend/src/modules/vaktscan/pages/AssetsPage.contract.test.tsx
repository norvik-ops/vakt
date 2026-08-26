/**
 * K5-16 regression, at the apiFetch seam: an asset's target address.
 *
 * The defect: the frontend read and wrote `target`. vaktscan.Asset emits
 * `external_url` (models.go:17, `*string` with omitempty) and
 * vaktscan.CreateAssetInput binds `external_url` (models.go:49). Two halves, both
 * broken by one name:
 *   READ  — the asset table's target column and the detail page's target row read
 *           `row.target`, which is never on the wire, so every asset showed the
 *           em-dash placeholder no matter what URL or IP it actually had. For a
 *           surface-scanning module the target address is the identifying field.
 *   WRITE — "New asset" posted `{ target: … }`. `external_url` is not `required`,
 *           so the create returned 201 with an empty external_url: the asset
 *           existed and had nothing to scan.
 *
 * Why apiFetch and not the hook: the 587-test baseline mocks hooks, i.e. it feeds
 * components fixtures the tests invented, which is why sixteen field-name drifts
 * lived under a green suite. The fixture below is the json tags of
 * vaktscan.Asset verbatim, delivered through the transport.
 *
 * Reverting the fix (`target` back in AssetsPage.tsx's column + form and in
 * CreateAssetInput) fails both tests: the table cell falls back to "—" and the
 * POST body carries `target`.
 */
import { describe, it, expect, vi, beforeAll, beforeEach } from 'vitest'
import { render, screen, fireEvent, waitFor } from '@testing-library/react'
import { MemoryRouter } from 'react-router-dom'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import AssetsPage from './AssetsPage'

vi.mock('react-i18next', () => ({
  useTranslation: () => ({ t: (k: string) => k, i18n: { language: 'de' } }),
}))

// test-setup.ts stubs matchMedia to `matches: false` for everything, so
// ResponsiveTable renders its MobileCardList — and the target column is
// `mobileHide: true`, i.e. not rendered there at all. The drifted column only
// exists on the desktop table, so that is the viewport under test.
beforeAll(() => {
  window.matchMedia = (query: string) => ({
    matches: query.includes('min-width: 1024px'),
    media: query,
    onchange: null,
    addEventListener: () => undefined,
    removeEventListener: () => undefined,
    addListener: () => undefined,
    removeListener: () => undefined,
    dispatchEvent: () => false,
  })
})

vi.mock('../../../api/client', async (importOriginal) => ({
  ...(await importOriginal<typeof import('../../../api/client')>()),
  apiFetch: vi.fn(),
}))
const { apiFetch } = await import('../../../api/client')
const mockApiFetch = vi.mocked(apiFetch)

// json tags of vaktscan.Asset. `external_url`, not `target` — and nothing named
// `target` exists on this struct at all.
const ASSET_ON_THE_WIRE = {
  id: 'a-1',
  org_id: 'org-1',
  name: 'Kundenportal',
  type: 'web_app',
  criticality: 'critical',
  environment: 'prod',
  classification: 'confidential',
  tags: ['extern'],
  owner_id: null,
  external_url: 'https://portal.example.com',
  protection_need_id: null,
  created_at: '2026-02-01T10:00:00Z',
  updated_at: '2026-02-01T10:00:00Z',
}

function stubApi() {
  const writes: { path: string; method: string; body: unknown }[] = []
  mockApiFetch.mockImplementation((path: string, init?: RequestInit) => {
    const method = init?.method ?? 'GET'
    if (method !== 'GET') {
      writes.push({ path, method, body: init?.body ? JSON.parse(init.body as string) : null })
      return Promise.resolve(ASSET_ON_THE_WIRE as never)
    }
    if (path.includes('/classification-summary')) {
      return Promise.resolve({
        total_count: 1, classified_count: 1, unclassified_count: 0,
        by_level: { public: 0, internal: 0, confidential: 1, restricted: 0 },
      } as never)
    }
    if (path.includes('/vaktscan/assets')) {
      return Promise.resolve({
        data: [ASSET_ON_THE_WIRE],
        pagination: { page: 1, limit: 20, total: 1, total_pages: 1 },
      } as never)
    }
    return Promise.resolve([] as never)
  })
  return { writes }
}

function renderPage() {
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false, gcTime: 0 }, mutations: { retry: false } },
  })
  return render(
    <QueryClientProvider client={client}>
      <MemoryRouter><AssetsPage /></MemoryRouter>
    </QueryClientProvider>,
  )
}

beforeEach(() => { mockApiFetch.mockReset() })

describe('AssetsPage — target address (K5-16)', () => {
  it('renders the address the backend actually sent', async () => {
    stubApi()
    renderPage()
    await screen.findByText('Kundenportal')

    // Reading `row.target` off this response yields undefined, and the column
    // falls back to the em-dash placeholder — a scan target that reads "—".
    expect(screen.getByText('https://portal.example.com')).toBeInTheDocument()
  })

  it('posts the address under external_url, never target', async () => {
    const { writes } = stubApi()
    renderPage()
    await screen.findByText('Kundenportal')

    fireEvent.click(screen.getByText('vaktscan.assetsPage.newAsset'))
    const dialog = await screen.findByRole('dialog')
    fireEvent.change(screen.getByLabelText('vaktscan.assetsPage.labelName'), {
      target: { value: 'Zahlungs-API' },
    })
    fireEvent.change(screen.getByPlaceholderText('https://example.com or 192.168.1.1'), {
      target: { value: 'https://pay.example.com' },
    })
    fireEvent.submit(dialog.querySelector('form') as HTMLFormElement)

    await waitFor(() => { expect(writes).toHaveLength(1) })
    const b = writes[0].body as Record<string, unknown>
    expect(writes[0].method).toBe('POST')
    // CreateAssetInput binds external_url. `target` is bound by nothing, and since
    // external_url carries no `required` tag the create succeeded with an empty one.
    expect(b.external_url).toBe('https://pay.example.com')
    expect(b).not.toHaveProperty('target')
    expect(b.name).toBe('Zahlungs-API')
  })
})
