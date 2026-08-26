/**
 * K5-13 regression: scheduled reports against the real wire format, at the
 * apiFetch seam.
 *
 * Two defects, only one of which was a rename:
 *   1. the payload carried `type`; CreateScheduledReportInput validates
 *      `report_type` as required → 422 on every create AND every edit, and the
 *      list rendered `reportTypeLabels[undefined]`;
 *   2. `active` was never sent at all. service.go Create writes
 *      `input.Active` straight into the row, so every report the UI created was
 *      inactive and the scheduler never picked it up. A rename alone would have
 *      turned a hard 422 into a silent no-op — worse, not better.
 *
 * The type/schedule/format inputs are Radix Selects, which cannot be driven with
 * fireEvent under jsdom. The assertions are therefore on the KEYS of the request
 * body (which is what drifted) plus the default values the form starts with; the
 * value domains are pinned by the TS union types and by
 * scripts/check_fe_be_fields.py.
 */
import { describe, it, expect, vi, beforeEach } from 'vitest'
import { render, screen, fireEvent, waitFor } from '@testing-library/react'
import { MemoryRouter } from 'react-router-dom'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import ScheduledReportsPage from './ScheduledReportsPage'

vi.mock('react-i18next', () => ({
  // useFormatDate reads i18n.language, so the stub has to carry it.
  useTranslation: () => ({ t: (k: string) => k, i18n: { language: 'de' } }),
}))

vi.mock('../api/client', async (importOriginal) => ({
  ...(await importOriginal<typeof import('../api/client')>()),
  apiFetch: vi.fn(),
}))
const { apiFetch } = await import('../api/client')
const mockApiFetch = vi.mocked(apiFetch)

// json tags of scheduledreports.ScheduledReport.
const REPORT_ON_THE_WIRE = {
  id: 'sr-1',
  org_id: 'org-1',
  name: 'Monatlicher Compliance-Bericht',
  report_type: 'compliance',
  schedule: 'monthly',
  recipients: ['ciso@example.com'],
  format: 'pdf',
  active: true,
  last_run_at: null,
  next_run_at: '2026-08-01T06:00:00Z',
  created_at: '2026-01-01T00:00:00Z',
}

const VALID_TYPES = ['compliance', 'findings', 'risk', 'board_report']

function stubApi(reports = [REPORT_ON_THE_WIRE]) {
  const writes: { path: string; method: string; body: unknown }[] = []
  mockApiFetch.mockImplementation((path: string, init?: RequestInit) => {
    const method = init?.method ?? 'GET'
    if (method === 'GET') return Promise.resolve(reports as never)
    writes.push({ path, method, body: init?.body ? JSON.parse(init.body as string) : null })
    return Promise.resolve(REPORT_ON_THE_WIRE as never)
  })
  return { writes }
}

function renderPage() {
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false, gcTime: 0 }, mutations: { retry: false } },
  })
  return render(
    <QueryClientProvider client={client}>
      <MemoryRouter><ScheduledReportsPage /></MemoryRouter>
    </QueryClientProvider>,
  )
}

beforeEach(() => { mockApiFetch.mockReset() })

describe('ScheduledReportsPage — list (K5-13)', () => {
  it('labels a report from report_type, not from a non-existent `type`', async () => {
    stubApi()
    renderPage()
    await screen.findByText('Monatlicher Compliance-Bericht')
    // Resolves to a real label key; with `report.type` it was
    // reportTypeLabels[undefined] → nothing rendered.
    expect(screen.getByText('scheduledReports.types.compliance')).toBeTruthy()
  })
})

describe('ScheduledReportsPage — create payload (K5-13)', () => {
  it('posts report_type and active, never `type`', async () => {
    const { writes } = stubApi([])
    renderPage()
    await screen.findByText('scheduledReports.noReports')

    fireEvent.click(screen.getByText('scheduledReports.addButton'))
    fireEvent.change(screen.getAllByRole('textbox')[0], { target: { value: 'Wochenbericht Findings' } })
    // ChipsInput commits on Enter.
    const email = document.querySelector('input[type="email"]') as HTMLInputElement
    fireEvent.change(email, { target: { value: 'board@example.com' } })
    fireEvent.keyDown(email, { key: 'Enter' })
    fireEvent.click(screen.getByText('scheduledReports.dialog.save'))

    await waitFor(() => { expect(writes).toHaveLength(1) })
    const b = writes[0].body as Record<string, unknown>
    expect(writes[0].method).toBe('POST')
    expect(b).not.toHaveProperty('type')
    expect(VALID_TYPES).toContain(b.report_type)
    // Without this the report is created inactive and never runs.
    expect(b.active).toBe(true)
    expect(b.recipients).toEqual(['board@example.com'])
  })

  it('round-trips report_type and active through the edit dialog', async () => {
    const { writes } = stubApi()
    renderPage()
    await screen.findByText('Monatlicher Compliance-Bericht')

    fireEvent.click(screen.getByTitle('scheduledReports.edit'))
    fireEvent.click(await screen.findByText('scheduledReports.dialog.save'))

    await waitFor(() => { expect(writes).toHaveLength(1) })
    const b = writes[0].body as Record<string, unknown>
    expect(writes[0].method).toBe('PUT')
    expect(b.report_type).toBe('compliance')
    expect(b.active).toBe(true)
    expect(b).not.toHaveProperty('type')
  })
})
