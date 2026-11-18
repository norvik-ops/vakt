import { describe, it, expect, vi } from 'vitest'
import { render, screen, within } from '@testing-library/react'
import { MemoryRouter, Route, Routes } from 'react-router-dom'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import FrameworkDetailPage from './FrameworkDetailPage'
import type { Control, Framework, ReadinessReport } from '../types'

// R1-KW-6 (Subreview PR #182): der Reiter „Kontrollen" rechnete covered/total —
// nicht anwendbare Kontrollen im Nenner, teilweise umgesetzte gar nicht. Die
// Kopfzahl kommt aus dem Backend (ADR-0086).
// UX-16: Kopf und Reiter zeigten dieselben Zahlen zweimal untereinander, einmal
// „Teilweise", einmal „In Bearbeitung". Es gibt jetzt nur noch den Kopf.
// UX-15: der Kopf nennt, dass „in Bearbeitung" halb zählt.
//
// 4 Kontrollen: umgesetzt, in Arbeit, nicht anwendbar, offen.
// ADR-0086: (1 + 0,5) / 3 anwendbare = 50 %.  Alte Reiter-Formel: 1 / 4 = 25 %.
// A.5.5 steht nur über einen Nachweis auf „in Bearbeitung" (UX-17) und zählt
// im Bericht nicht mit — der Bericht ist hier ein fester Mock.

const ISO: Framework = {
  id: 'fw-iso', org_id: 'org-1', name: 'ISO27001', is_builtin: true, version: '2022',
  created_at: new Date().toISOString(),
}

const REPORT: ReadinessReport = {
  framework_id: 'fw-iso', framework_name: 'ISO27001', readiness_score: 50,
  total_controls: 4, covered: 1, partial: 1, missing: 1, not_applicable: 1, by_domain: [],
}

function ctrl(id: string, status: Control['status'], extra: Partial<Control> = {}): Control {
  return { id, framework_id: 'fw-iso', control_id: id, title: `Kontrolle ${id}`, domain: 'A.5', status, ...extra } as Control
}

const CONTROLS: Control[] = [
  ctrl('A.5.1', 'implemented', { manual_status: 'implemented' }),
  ctrl('A.5.2', 'in_progress', { manual_status: 'in_progress', evidence_count: 1 }),
  ctrl('A.5.3', 'not_applicable', { not_applicable: true }),
  ctrl('A.5.4', 'missing'),
  ctrl('A.5.5', 'partial', { manual_status: '', evidence_count: 1 }),
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

function renderPage() {
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
}

describe('FrameworkDetailPage — ein Kopf, eine Zählung (R1-KW-6, UX-15, UX-16)', () => {
  it('zeigt den Umsetzungsgrad aus dem Backend genau einmal, nicht die alte Reiter-Formel', () => {
    renderPage()
    expect(screen.getAllByText('50%')).toHaveLength(1)
    expect(screen.queryByText('25%')).not.toBeInTheDocument()
    expect(screen.getByText('Umsetzungsgrad')).toBeInTheDocument()
    expect(screen.queryByText('Readiness Score')).not.toBeInTheDocument()
  })

  it('nennt umgesetzt und in Bearbeitung neben dem Prozentwert und die Zählregel', () => {
    renderPage()
    const head = screen.getByTestId('readiness-head')
    expect(within(head).getByText('1 / 3 umgesetzt · 1 in Bearbeitung · 50%')).toBeInTheDocument()
    expect(within(head).getByText(/in Bearbeitung zur Hälfte/)).toBeInTheDocument()
    // Ein Begriff: „In Bearbeitung" wie in der Statusauswahl, kein „Teilweise".
    expect(within(head).getByText('In Bearbeitung')).toBeInTheDocument()
    expect(screen.queryByText('Teilweise')).not.toBeInTheDocument()
  })

  it('kein zweiter Zählblock im Reiter (UX-16)', () => {
    renderPage()
    // „Nicht anwendbar" als Zähler-Beschriftung nur noch im Kopf.
    const labels = screen.getAllByText('Nicht anwendbar').filter((el) => !el.closest('[role="combobox"]') && !el.closest('tr'))
    expect(labels).toHaveLength(1)
    expect(screen.queryByText(/von \d+ Controls umgesetzt/)).not.toBeInTheDocument()
  })
})

describe('FrameworkDetailPage — abgeleiteter Status ist gekennzeichnet (UX-17)', () => {
  it('kennzeichnet nur die Zeile, deren Status niemand gesetzt hat', () => {
    renderPage()
    const marks = screen.getAllByTestId('status-derived')
    expect(marks).toHaveLength(1)
    expect(marks[0]).toHaveTextContent('aus 1 Nachweis abgeleitet')
    expect(marks[0].closest('tr')).toHaveTextContent('A.5.5')
    expect(marks[0]).toHaveAttribute('title', expect.stringMatching(/niemand gesetzt/))
  })
})
