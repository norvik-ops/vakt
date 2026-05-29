import { describe, it, expect, vi } from 'vitest'
import { render, screen } from '@testing-library/react'
import { MemoryRouter, Route, Routes } from 'react-router-dom'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import FrameworkDetailPage from './FrameworkDetailPage'
import type { Framework, ReadinessReport, GapAnalysis } from '../types'

// ── mocks ─────────────────────────────────────────────────────────────────────

const DORA_FRAMEWORK: Framework = {
  id: 'fw-dora',
  name: 'DORA',
  version: '2022',
  created_at: new Date().toISOString(),
}

const MOCK_REPORT: ReadinessReport = {
  framework_id: 'fw-dora',
  framework_name: 'DORA',
  readiness_score: 50,
  total_controls: 18,
  covered: 9,
  partial: 0,
  missing: 9,
  by_domain: [],
}

const MOCK_GAPS: GapAnalysis = { framework_id: 'fw-dora', gaps: [] }

vi.mock('../hooks/useFrameworks', () => ({
  useFramework: () => ({ data: DORA_FRAMEWORK, isLoading: false }),
  useReadinessReport: () => ({ data: MOCK_REPORT, isLoading: false }),
  useGapAnalysis: () => ({ data: MOCK_GAPS, isLoading: false }),
  useFrameworkControls: () => ({ data: [], isLoading: false }),
  useDownloadFrameworkPDF: () => () => {},
}))

vi.mock('../hooks/useAuditorLinks', () => ({
  useAuditorLinks: () => ({ data: [], isLoading: false }),
  useRevokeAuditorLink: () => ({ mutate: vi.fn(), isPending: false }),
  useCreateAuditorLink: () => ({ mutate: vi.fn(), isPending: false }),
}))

vi.mock('../hooks/useControls', () => ({
  useUpdateControl: () => ({ mutate: vi.fn(), isPending: false }),
  // Bulk-Action hook used by ControlsTab. Returns a thenable mutateAsync so the
  // page can `await` it without exploding when no real network exists in tests.
  useBulkUpdateControls: () => ({
    mutate: vi.fn(),
    mutateAsync: vi.fn().mockResolvedValue({ updated: 0 }),
    isPending: false,
  }),
  useExportControl: () => ({ mutate: vi.fn(), isPending: false }),
}))

function renderFrameworkDetailPage(id = 'fw-dora') {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } })
  return render(
    <QueryClientProvider client={client}>
      <MemoryRouter initialEntries={[`/vaktcomply/frameworks/${id}`]}>
        <Routes>
          <Route path="/vaktcomply/frameworks/:id" element={<FrameworkDetailPage />} />
        </Routes>
      </MemoryRouter>
    </QueryClientProvider>,
  )
}

// ── Tests ─────────────────────────────────────────────────────────────────────

describe('FrameworkDetailPage — DORA ISO mapping block', () => {
  it('renders the dora-iso-mapping-block when framework name is DORA', () => {
    renderFrameworkDetailPage('fw-dora')
    expect(screen.getByTestId('dora-iso-mapping-block')).toBeInTheDocument()
  })
})
