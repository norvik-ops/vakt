/**
 * K5-10 regression: a phishing campaign created through the UI must carry a
 * target group. Asserted at the apiFetch seam.
 *
 * This is a core Vakt Aware feature and it was 100 % broken through the UI: the
 * payload used `target_group_id`, CreateCampaignInput binds `group_id` (*string,
 * no validate tag), so echo's binder dropped the value silently. Result: 201
 * Created, `group_id = NULL`, the user is navigated to the detail page as if it
 * worked — and the send fails much later in service.go with "campaign has no
 * target group". calcPhishingClickRate then counts recipients via
 * `sr_targets WHERE group_id = c.group_id`, so the click-rate KPI is structurally
 * undefined too.
 *
 * NOTE ON OVERLAP: R1-14-D05 describes the same *symptom* from a different root
 * (`_ = u.Scan()` discarding a malformed UUID). Fixing D05 does not fix this —
 * here no group_id arrives at all, so there is nothing to scan. Both need to hold.
 *
 * The target-group picker is a Radix Select, which fireEvent cannot drive under
 * jsdom, so the assertion is on the payload KEY — which is exactly what drifted.
 * Reverting the fix puts `target_group_id` back in the body and fails this test.
 */
import { describe, it, expect, vi, beforeEach } from 'vitest'
import { render, screen, fireEvent, waitFor } from '@testing-library/react'
import { MemoryRouter } from 'react-router-dom'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import CampaignsPage from './CampaignsPage'

vi.mock('react-i18next', () => ({
  useTranslation: () => ({ t: (k: string) => k, i18n: { language: 'de' } }),
}))

vi.mock('../../../api/client', async (importOriginal) => ({
  ...(await importOriginal<typeof import('../../../api/client')>()),
  apiFetch: vi.fn(),
}))
const { apiFetch } = await import('../../../api/client')
const mockApiFetch = vi.mocked(apiFetch)

// json tags of vaktaware.Campaign.
const CAMPAIGN_ON_THE_WIRE = {
  id: 'c-1',
  org_id: 'org-1',
  name: 'Q1 Awareness',
  status: 'draft',
  template_id: 't-1',
  group_id: 'tg-1',
  from_name: 'Security Team',
  from_email: 'security@example.com',
  subject: 'Wichtige Info',
  track_opens: false,
  betriebsrat_mode: true,
  created_at: '2026-01-10T08:00:00Z',
  updated_at: '2026-01-10T08:00:00Z',
}

const GROUP_ON_THE_WIRE = {
  id: 'tg-1',
  org_id: 'org-1',
  name: 'Alle Mitarbeitenden',
  source: 'manual',
  created_at: '2026-01-01T00:00:00Z',
}

function stubApi(campaigns: unknown[] = [CAMPAIGN_ON_THE_WIRE]) {
  const writes: { path: string; method: string; body: unknown }[] = []
  mockApiFetch.mockImplementation((path: string, init?: RequestInit) => {
    const method = init?.method ?? 'GET'
    if (method === 'GET') {
      if (path.includes('/target-groups')) return Promise.resolve([GROUP_ON_THE_WIRE] as never)
      if (path.includes('/templates')) return Promise.resolve([] as never)
      return Promise.resolve(campaigns as never)
    }
    writes.push({ path, method, body: init?.body ? JSON.parse(init.body as string) : null })
    return Promise.resolve(CAMPAIGN_ON_THE_WIRE as never)
  })
  return { writes }
}

function renderPage() {
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false, gcTime: 0 }, mutations: { retry: false } },
  })
  return render(
    <QueryClientProvider client={client}>
      <MemoryRouter><CampaignsPage /></MemoryRouter>
    </QueryClientProvider>,
  )
}

beforeEach(() => { mockApiFetch.mockReset() })

describe('CampaignsPage — create payload (K5-10)', () => {
  it('posts the target group under group_id, never target_group_id', async () => {
    const { writes } = stubApi([])
    renderPage()
    await screen.findByText('vaktaware.campaignsPage.noCampaigns')

    fireEvent.click(screen.getAllByText('vaktaware.campaignsPage.createCampaign')[0])
    const dialog = await screen.findByRole('dialog')
    fireEvent.change(screen.getByLabelText('vaktaware.campaignsPage.labelCampaignName'), {
      target: { value: 'Q3 Phishing-Simulation' },
    })
    fireEvent.submit(dialog.querySelector('form') as HTMLFormElement)

    await waitFor(() => { expect(writes).toHaveLength(1) })
    const b = writes[0].body as Record<string, unknown>
    expect(writes[0].method).toBe('POST')
    expect(writes[0].path).toBe('/vaktaware/campaigns')
    // The binder reads `group_id`. Anything else is dropped without a word.
    expect(b).toHaveProperty('group_id')
    expect(b).not.toHaveProperty('target_group_id')
    expect(b.name).toBe('Q3 Phishing-Simulation')
  })
})
