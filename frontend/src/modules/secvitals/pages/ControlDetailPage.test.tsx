import { describe, it, expect, vi } from 'vitest'
import { render, screen } from '@testing-library/react'
import { MemoryRouter, Route, Routes } from 'react-router-dom'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import ControlDetailPage from './ControlDetailPage'
import type { Control, Framework } from '../types'

// ── helpers ──────────────────────────────────────────────────────────────────

const DORA_CONTROL: Control = {
  id: 'ctrl-dora-1',
  framework_id: 'fw-dora',
  control_id: 'DORA-1.1',
  title: 'ICT-Risikomanagement-Framework',
  description: 'Test description',
  domain: 'ICT-Risikomanagement',
  status: 'missing',
  not_applicable: false,
  iso27001_mapping: 'A.5.30, A.8.6',
}

const TISAX_CONTROL: Control = {
  id: 'ctrl-tisax-1',
  framework_id: 'fw-tisax',
  control_id: 'TISAX-1.1',
  title: 'Informationsklassifizierung',
  description: 'TISAX test control',
  domain: 'Informationssicherheit',
  status: 'missing',
  not_applicable: false,
  maturity_score: 1,
}

const TISAX_FRAMEWORK: Framework = {
  id: 'fw-tisax',
  name: 'TISAX',
  version: '2023',
  created_at: '2024-01-01T00:00:00Z',
}

const ISO_FRAMEWORK: Framework = {
  id: 'fw-iso',
  name: 'ISO 27001',
  version: '2022',
  created_at: '2024-01-01T00:00:00Z',
}

// controlData is a mutable ref so tests can switch between controls
let mockControl: Control = DORA_CONTROL
let mockFramework: Framework | undefined = undefined

vi.mock('../hooks/useControls', () => ({
  useControl: () => ({ data: mockControl, isLoading: false }),
  useAddEvidence: () => ({ mutate: vi.fn(), isPending: false }),
  useUploadEvidence: () => ({ mutate: vi.fn(), isPending: false }),
  useCollectEvidence: () => ({ mutate: vi.fn(), isPending: false }),
  useExportControl: () => () => {},
  useUpdateControl: () => ({ mutate: vi.fn(), isPending: false }),
}))

vi.mock('../hooks/useEvidence', () => ({
  useEvidence: () => ({ data: [], isLoading: false }),
  useReviewEvidence: () => ({ mutate: vi.fn(), isPending: false }),
}))

vi.mock('../hooks/useFrameworks', () => ({
  useFrameworkControls: () => ({ data: [], isLoading: false }),
  useFramework: () => ({ data: mockFramework }),
}))

vi.mock('../hooks/useControlTasks', () => ({
  useControlTasks: () => ({ data: [], isLoading: false }),
  useCreateControlTask: () => ({ mutate: vi.fn(), isPending: false }),
  useToggleControlTask: () => ({ mutate: vi.fn(), isPending: false }),
  useDeleteControlTask: () => ({ mutate: vi.fn(), isPending: false }),
}))

function renderControlDetailPage(controlId = 'ctrl-dora-1', frameworkId = 'fw-dora') {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } })
  return render(
    <QueryClientProvider client={client}>
      <MemoryRouter
        initialEntries={[`/secvitals/controls/${controlId}?frameworkId=${frameworkId}`]}
      >
        <Routes>
          <Route path="/secvitals/controls/:id" element={<ControlDetailPage />} />
        </Routes>
      </MemoryRouter>
    </QueryClientProvider>,
  )
}

// ── Tests ─────────────────────────────────────────────────────────────────────

describe('ControlDetailPage — ISO 27001 mapping badge', () => {
  it('renders the ISO 27001 mapping badge when iso27001_mapping is set', () => {
    mockControl = DORA_CONTROL
    mockFramework = undefined
    renderControlDetailPage()
    expect(screen.getByTestId('iso27001-mapping-badge')).toBeInTheDocument()
    expect(screen.getByText(/A\.5\.30/)).toBeInTheDocument()
  })
})

describe('ControlDetailPage — TISAX maturity UI', () => {
  it('shows maturity radio buttons for TISAX framework', () => {
    mockControl = TISAX_CONTROL
    mockFramework = TISAX_FRAMEWORK
    renderControlDetailPage('ctrl-tisax-1', 'fw-tisax')
    expect(screen.getByTestId('maturity-radio-group')).toBeInTheDocument()
    // All 4 maturity levels visible
    expect(screen.getByTestId('maturity-radio-0')).toBeInTheDocument()
    expect(screen.getByTestId('maturity-radio-1')).toBeInTheDocument()
    expect(screen.getByTestId('maturity-radio-2')).toBeInTheDocument()
    expect(screen.getByTestId('maturity-radio-3')).toBeInTheDocument()
  })

  it('shows status toggle (not maturity radio buttons) for ISO 27001 framework', () => {
    mockControl = DORA_CONTROL
    mockFramework = ISO_FRAMEWORK
    renderControlDetailPage('ctrl-dora-1', 'fw-iso')
    expect(screen.queryByTestId('maturity-radio-group')).not.toBeInTheDocument()
  })
})
