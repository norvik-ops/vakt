import { describe, it, expect, vi } from 'vitest'
import { render, screen } from '@testing-library/react'
import { MemoryRouter, Route, Routes } from 'react-router-dom'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import FrameworkDetailPage from './FrameworkDetailPage'
import type { Control, Framework, ReadinessReport } from '../types'

// R1-KW-6 (Subreview PR #182): der Reiter „Kontrollen" rechnete covered/total —
// nicht anwendbare Kontrollen im Nenner, teilweise umgesetzte gar nicht. Die
// Kopfzahl kommt aus dem Backend (ADR-0086). Beide müssen dieselbe Zahl zeigen.
//
// 4 Kontrollen: umgesetzt, in Arbeit, nicht anwendbar, offen.
// ADR-0086: (1 + 0,5) / 3 anwendbare = 50 %.  Alte Reiter-Formel: 1 / 4 = 25 %.

const ISO: Framework = {
  id: 'fw-iso', org_id: 'org-1', name: 'ISO27001', is_builtin: true, version: '2022',
  created_at: new Date().toISOString(),
}

const REPORT: ReadinessReport = {
  framework_id: 'fw-iso', framework_name: 'ISO27001', readiness_score: 50,
  total_controls: 4, covered: 1, partial: 1, missing: 1, by_domain: [],
}

function ctrl(id: string, status: Control['status']): Control {
  return { id, framework_id: 'fw-iso', control_id: id, title: `Kontrolle ${id}`, domain: 'A.5', status } as Control
}

const CONTROLS: Control[] = [
  ctrl('A.5.1', 'implemented'),
  ctrl('A.5.2', 'in_progress'),
  ctrl('A.5.3', 'not_applicable'),
  ctrl('A.5.4', 'missing'),
]

vi.mock('../hooks/useFrameworks', () => ({
  useFramework: () => ({ data: ISO, isLoading: false }),
  useReadinessReport: () => ({ data: REPORT, isLoading: false }),
  useGapAnalysis: () => ({ data: { framework_id: 'fw-iso', gaps: [] }, isLoading: false }),
  useFrameworkControls: () => ({ data: CONTROLS, isLoading: false }),
  useDownloadFrameworkPDF: () => () => {},
}))

vi.mock('../hooks/useAuditorLinks', () => ({
  useAuditorLinks: () => ({ data: [], isLoading: false }),
  useRevokeAuditorLink: () => ({ mutate: vi.fn(), isPending: false }),
  useCreateAuditorLink: () => ({ mutate: vi.fn(), isPending: false }),
}))

vi.mock('../hooks/useControls', () => ({
  useUpdateControl: () => ({ mutate: vi.fn(), isPending: false }),
  useBulkUpdateControls: () => ({ mutate: vi.fn(), mutateAsync: vi.fn().mockResolvedValue({ updated: 0 }), isPending: false }),
  useExportControl: () => ({ mutate: vi.fn(), isPending: false }),
}))

describe('FrameworkDetailPage — Reiter und Kopf zeigen denselben Umsetzungsgrad (R1-KW-6)', () => {
  it('rechnet nicht selbst: nicht anwendbar fällt aus dem Nenner, „in Arbeit" zählt halb', () => {
    const client = new QueryClient({ defaultOptions: { queries: { retry: false } } })
    render(
      <QueryClientProvider client={client}>
        <MemoryRouter initialEntries={['/vaktcomply/frameworks/fw-iso']}>
          <Routes>
            <Route path="/vaktcomply/frameworks/:id" element={<FrameworkDetailPage />} />
          </Routes>
        </MemoryRouter>
      </QueryClientProvider>,
    )
    // Kopf (Readiness Score) und Fortschrittsbalken im Reiter.
    expect(screen.getAllByText('50%')).toHaveLength(2)
    expect(screen.queryByText('25%')).not.toBeInTheDocument()
  })
})
