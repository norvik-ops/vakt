/**
 * K5-04/05/06 regression: the ISO 27001 clause 9.2 audit programme against the
 * real wire format, at the apiFetch seam.
 *
 * K5-05 was NOT just a wrong field name — the create dialog failed for three
 * independent reasons at once, and only one of them was a rename:
 *   1. `audit_type: 'internal'` is outside `oneof=isms_internal compliance_check
 *      supplier_audit process_audit` (and outside the DDL CHECK). None of the five
 *      types the UI offered was inside it.
 *   2. `scope` is validate:"required" and the dialog had no such field at all.
 *   3. the date went out as `scheduled_date`; the binder wants `planned_date`.
 * So this file asserts the whole POST payload, not a spelling: a rename-only fix
 * would still have produced a 422.
 *
 * K5-06 (complete) is the same shape: audit_report (min 10) + actual_date, while
 * the dialog sent summary/overall_rating/completed_date — none of which exist.
 * K5-04 covers the summary tiles, four of five of which rendered `undefined`.
 */
import { describe, it, expect, vi, beforeEach } from 'vitest'
import { render, screen, fireEvent, waitFor } from '@testing-library/react'
import { MemoryRouter } from 'react-router-dom'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import AuditProgramPage from './AuditProgramPage'

vi.mock('react-i18next', () => ({
  useTranslation: () => ({ t: (k: string) => k }),
}))

vi.mock('../../../api/client', () => ({ apiFetch: vi.fn() }))
const { apiFetch } = await import('../../../api/client')
const mockApiFetch = vi.mocked(apiFetch)

// json tags of audit.AuditProgramSummary.
const SUMMARY_ON_THE_WIRE = {
  audits_planned_this_year: 7,
  audits_completed: 3,
  open_findings: 5,
  overdue_capas_from_audits: 2,
}

// json tags of audit.AuditProgramAudit.
const AUDIT_ON_THE_WIRE = {
  id: 'a-1',
  org_id: 'org-1',
  audit_plan_id: null,
  title: 'Internes ISMS-Audit 2026',
  audit_type: 'isms_internal',
  scope: 'Rechenzentrum Bonn, Kernprozesse',
  methodology: 'combined',
  planned_date: '2026-05-04',
  actual_date: null,
  lead_auditor_id: null,
  auditor_ids: [],
  supplier_id: null,
  status: 'planned',
  audit_report: '',
  findings_count: 0,
  created_at: '2026-01-01T00:00:00Z',
  updated_at: '2026-01-01T00:00:00Z',
}

const VALID_AUDIT_TYPES = ['isms_internal', 'compliance_check', 'supplier_audit', 'process_audit']
const VALID_SEVERITIES = ['major_nc', 'minor_nc', 'observation', 'ofi']

function stubApi(audits = [AUDIT_ON_THE_WIRE]) {
  const writes: { path: string; method: string; body: unknown }[] = []
  mockApiFetch.mockImplementation((path: string, init?: RequestInit) => {
    const method = init?.method ?? 'GET'
    if (method === 'GET') {
      if (path.endsWith('/summary')) return Promise.resolve(SUMMARY_ON_THE_WIRE as never)
      if (path.endsWith('/findings')) return Promise.resolve([] as never)
      return Promise.resolve(audits as never)
    }
    writes.push({ path, method, body: init?.body ? JSON.parse(init.body as string) : null })
    return Promise.resolve(AUDIT_ON_THE_WIRE as never)
  })
  return { writes }
}

function renderPage() {
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false, gcTime: 0 }, mutations: { retry: false } },
  })
  return render(
    <QueryClientProvider client={client}>
      <MemoryRouter><AuditProgramPage /></MemoryRouter>
    </QueryClientProvider>,
  )
}

beforeEach(() => { mockApiFetch.mockReset() })

describe('AuditProgramPage — summary tiles (K5-04)', () => {
  it('renders every number audit.AuditProgramSummary actually sends', async () => {
    stubApi()
    renderPage()
    await screen.findByText('Internes ISMS-Audit 2026')
    for (const n of ['7', '3', '5', '2']) {
      expect(screen.getByText(n)).toBeTruthy()
    }
  })

  it('renders the planned date the backend sent', async () => {
    stubApi()
    renderPage()
    await screen.findByText('Internes ISMS-Audit 2026')
    expect(screen.getByText(/2026-05-04/)).toBeTruthy()
  })
})

describe('AuditProgramPage — create (K5-05, three independent causes)', () => {
  it('posts an audit_type inside the oneof, a non-empty scope and planned_date', async () => {
    const { writes } = stubApi()
    renderPage()
    await screen.findByText('Internes ISMS-Audit 2026')

    fireEvent.click(screen.getAllByText('vaktcomply.auditProgram.createBtn')[0])
    const boxes = screen.getAllByRole('textbox')
    fireEvent.change(boxes[0], { target: { value: 'Q3-Audit Lieferkette' } })
    // The scope textarea — the field the dialog used not to have at all.
    fireEvent.change(screen.getAllByRole('textbox').slice(-1)[0], { target: { value: 'Lieferantenprozesse DACH' } })
    fireEvent.change(document.querySelector('input[type="date"]') as HTMLInputElement, {
      target: { value: '2026-08-01' },
    })
    fireEvent.click(screen.getByText('vaktcomply.auditProgram.createSubmitBtn'))

    await waitFor(() => { expect(writes).toHaveLength(1) })
    const b = writes[0].body as Record<string, unknown>
    expect(writes[0].method).toBe('POST')
    expect(VALID_AUDIT_TYPES).toContain(b.audit_type)   // cause 1
    expect(b.scope).toBe('Lieferantenprozesse DACH')     // cause 2
    expect(b.planned_date).toBe('2026-08-01')            // cause 3
    for (const dead of ['scheduled_date', 'lead_auditor', 'plan_id']) {
      expect(b).not.toHaveProperty(dead)
    }
  })

  it('keeps submit disabled until scope and planned_date are filled', async () => {
    stubApi()
    renderPage()
    await screen.findByText('Internes ISMS-Audit 2026')
    fireEvent.click(screen.getAllByText('vaktcomply.auditProgram.createBtn')[0])
    fireEvent.change(screen.getAllByRole('textbox')[0], { target: { value: 'Nur ein Titel' } })
    // A title alone used to be enough to submit — and to get a 422.
    expect(screen.getByText('vaktcomply.auditProgram.createSubmitBtn').closest('button')).toBeDisabled()
  })
})

describe('AuditProgramPage — complete (K5-06)', () => {
  it('patches audit_report and actual_date, not summary/overall_rating', async () => {
    const { writes } = stubApi()
    renderPage()
    await screen.findByText('Internes ISMS-Audit 2026')

    fireEvent.click(screen.getByText('vaktcomply.auditProgram.completeBtn'))
    const report = 'Keine wesentlichen Abweichungen festgestellt; zwei OFI dokumentiert.'
    fireEvent.change(await screen.findByRole('textbox'), { target: { value: report } })
    fireEvent.click(screen.getByText('vaktcomply.auditProgram.completeSubmitBtn'))

    await waitFor(() => { expect(writes).toHaveLength(1) })
    const b = writes[0].body as Record<string, unknown>
    expect(writes[0].method).toBe('PATCH')
    expect(writes[0].path).toBe('/vaktcomply/audit-program/a-1/complete')
    expect(b.audit_report).toBe(report)
    expect(String(b.actual_date)).toMatch(/^\d{4}-\d{2}-\d{2}$/)
    for (const dead of ['summary', 'overall_rating', 'completed_date']) {
      expect(b).not.toHaveProperty(dead)
    }
  })

  it('refuses to submit an audit_report shorter than the backend minimum', async () => {
    stubApi()
    renderPage()
    await screen.findByText('Internes ISMS-Audit 2026')
    fireEvent.click(screen.getByText('vaktcomply.auditProgram.completeBtn'))
    fireEvent.change(await screen.findByRole('textbox'), { target: { value: 'zu kurz' } })
    // validate:"min=10" — submitting this would be a guaranteed 422.
    expect(screen.getByText('vaktcomply.auditProgram.completeSubmitBtn').closest('button')).toBeDisabled()
  })
})

describe('AuditProgramPage — finding severity', () => {
  it('posts a severity inside the oneof', async () => {
    const { writes } = stubApi()
    renderPage()
    await screen.findByText('Internes ISMS-Audit 2026')

    fireEvent.click(screen.getByText('vaktcomply.auditProgram.findingBtn'))
    fireEvent.change((await screen.findAllByRole('textbox'))[0], { target: { value: 'Fehlende Freigabe' } })
    fireEvent.click(screen.getByText('vaktcomply.auditProgram.findingSubmitBtn'))

    await waitFor(() => { expect(writes).toHaveLength(1) })
    const b = writes[0].body as Record<string, unknown>
    // `opportunity` (the old default label value) is outside
    // validate:"oneof=major_nc minor_nc observation ofi".
    expect(VALID_SEVERITIES).toContain(b.severity)
  })
})
