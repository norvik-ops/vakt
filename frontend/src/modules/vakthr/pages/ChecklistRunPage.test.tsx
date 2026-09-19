import { describe, it, expect, vi, beforeEach } from 'vitest'
import { screen, fireEvent, waitFor } from '@testing-library/react'
import { Routes, Route } from 'react-router-dom'
import { renderWithProviders } from '../../../test-utils'
import ChecklistRunPage from './ChecklistRunPage'
import { apiFetch } from '../../../api/client'

vi.mock('../../../api/client', () => ({ apiFetch: vi.fn() }))

const RUN = {
  id: 'run-1',
  org_id: 'org-1',
  employee_id: 'emp-1',
  checklist_id: 'cl-1',
  status: 'in_progress',
  completed_items: [] as string[],
  completed_at: null,
  created_at: '2026-01-01T00:00:00Z',
  updated_at: '2026-01-01T00:00:00Z',
}

const CHECKLIST = {
  id: 'cl-1',
  name: 'Standard Offboarding',
  type: 'offboarding',
  items: [{ id: 's1', label: 'Alle IT-Zugänge widerrufen', required: true }],
}

const EMPLOYEE = {
  id: 'emp-1',
  org_id: 'org-1',
  first_name: 'Anna',
  last_name: 'Muster',
  email: 'anna.muster@example.com',
  status: 'offboarding',
  created_at: '2026-01-01T00:00:00Z',
  updated_at: '2026-01-01T00:00:00Z',
}

function renderRunPage() {
  return renderWithProviders(
    <Routes>
      <Route path="/vakthr/checklist-runs/:id" element={<ChecklistRunPage />} />
    </Routes>,
    { initialPath: '/vakthr/checklist-runs/run-1' },
  )
}

beforeEach(() => {
  vi.mocked(apiFetch).mockReset()
  vi.mocked(apiFetch).mockImplementation((url: string, opts?: { method?: string }) => {
    const method = opts?.method ?? 'GET'
    if (url.includes('/steps/') && method === 'POST') {
      // The per-step completion endpoint: server records hr_run_events and, since
      // this closes the only required step, auto-completes the run.
      return Promise.resolve({ ...RUN, completed_items: ['s1'], status: 'completed' })
    }
    if (url === '/vakthr/checklist-runs/run-1' && method === 'GET') return Promise.resolve(RUN)
    if (url.startsWith('/vakthr/checklists/')) return Promise.resolve(CHECKLIST)
    if (url.startsWith('/vakthr/employees/')) return Promise.resolve(EMPLOYEE)
    return Promise.reject(new Error(`unexpected apiFetch: ${method} ${url}`))
  })
})

// R1-36c-03 Teil C: ticking a step must write an hr_run_events row. That only
// happens on the per-step endpoint (POST …/steps/:id), never on the bulk PUT the
// page used before. This test asserts the check path hits the step endpoint and
// never the PUT.
describe('ChecklistRunPage — completing a step', () => {
  it('POSTs to the per-step endpoint (not the bulk PUT) when a step is checked', async () => {
    renderRunPage()

    const stepButton = await screen.findByRole('checkbox')
    fireEvent.click(stepButton)

    await waitFor(() => {
      expect(vi.mocked(apiFetch)).toHaveBeenCalledWith(
        '/vakthr/checklist-runs/run-1/steps/s1',
        expect.objectContaining({ method: 'POST' }),
      )
    })

    // The check path must NOT go through the bulk PUT (that path writes no event).
    const putCalls = vi.mocked(apiFetch).mock.calls.filter(
      ([, opts]) => (opts)?.method === 'PUT',
    )
    expect(putCalls).toHaveLength(0)
  })
})
