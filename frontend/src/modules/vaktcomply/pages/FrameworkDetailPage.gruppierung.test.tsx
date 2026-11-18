/**
 * UX-19: ISO 27001 im Framework-Detail nach Anhang-A-Kapiteln wie die SoA
 * (A.5–A.8, Reihenfolge der Norm), nicht nach den Seed-Themen, die A.5 und A.8
 * mischten. Andere Frameworks bleiben nach Thema gruppiert.
 */
import { describe, it, expect, vi } from 'vitest'
import { render, screen } from '@testing-library/react'
import { MemoryRouter, Route, Routes } from 'react-router-dom'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import FrameworkDetailPage from './FrameworkDetailPage'
import type { Control, Framework, ReadinessReport } from '../types'
import { compareControlIds, groupControls, isISO27001 } from '../utils/controlGroups'

function fw(name: string): Framework {
  return { id: 'fw-1', org_id: 'org-1', name, is_builtin: true, version: '2022', created_at: new Date().toISOString() }
}

const REPORT: ReadinessReport = {
  framework_id: 'fw-1', framework_name: 'ISO27001', readiness_score: 0,
  total_controls: 5, covered: 0, partial: 0, missing: 5, not_applicable: 0, by_domain: [],
}

function ctrl(id: string, domain: string): Control {
  return { id, framework_id: 'fw-1', control_id: id, title: `Kontrolle ${id}`, domain, status: 'missing' } as Control
}

// Seed-Themen wie in service_helpers.go: „Zugangskontrolle" mischt A.5 und A.8.
const ISO_CONTROLS: Control[] = [
  ctrl('A.8.5', 'Zugangskontrolle'),
  ctrl('A.5.15', 'Zugangskontrolle'),
  ctrl('A.5.10', 'Asset Management'),
  ctrl('A.5.2', 'Richtlinien'),
  ctrl('A.7.1', 'Physische Sicherheit'),
  ctrl('A.6.3', 'Personal'),
]

const state = vi.hoisted(() => ({ framework: undefined as Framework | undefined, controls: [] as Control[] }))

vi.mock('../hooks/useFrameworks', () => ({
  useFramework: () => ({ data: state.framework, isLoading: false }),
  useReadinessReport: () => ({ data: REPORT, isLoading: false }),
  useGapAnalysis: () => ({ data: { framework_id: 'fw-iso', gaps: [] }, isLoading: false }),
  useFrameworkControls: () => ({ data: state.controls, isLoading: false }),
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

function renderPage(name: string, controls: Control[]) {
  state.framework = fw(name)
  state.controls = controls
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } })
  render(
    <QueryClientProvider client={client}>
      <MemoryRouter initialEntries={['/vaktcomply/frameworks/fw-1']}>
        <Routes>
          <Route path="/vaktcomply/frameworks/:id" element={<FrameworkDetailPage />} />
        </Routes>
      </MemoryRouter>
    </QueryClientProvider>,
  )
}

const LABELS = { '5': 'A.5 Organisatorisch', '6': 'A.6 Personen', '7': 'A.7 Physisch', '8': 'A.8 Technologisch' }

describe('groupControls', () => {
  it('gruppiert ISO 27001 nach Kapiteln in Normreihenfolge, A.5.2 vor A.5.10', () => {
    const groups = groupControls(ISO_CONTROLS, 'ISO27001', LABELS)
    expect(groups.map(([h]) => h)).toEqual(['A.5 Organisatorisch', 'A.6 Personen', 'A.7 Physisch', 'A.8 Technologisch'])
    expect(groups[0][1].map((c) => c.control_id)).toEqual(['A.5.2', 'A.5.10', 'A.5.15'])
  })

  it('hängt eigene ISO-Kontrollen ohne Anhang-A-ID unter ihrem Thema an', () => {
    const groups = groupControls([...ISO_CONTROLS, ctrl('ORG-1', 'Eigene Ergänzungen')], 'ISO 27001', LABELS)
    expect(groups[groups.length - 1][0]).toBe('Eigene Ergänzungen')
  })

  it('lässt andere Frameworks nach Thema in Eingangsreihenfolge', () => {
    const nis2 = [ctrl('NIS2-B.1', 'Risikomanagement'), ctrl('NIS2-A.1', 'Governance'), ctrl('NIS2-B.2', 'Risikomanagement')]
    expect(groupControls(nis2, 'NIS2', LABELS).map(([h, l]) => [h, l.length])).toEqual([['Risikomanagement', 2], ['Governance', 1]])
  })

  it('erkennt nur ISO 27001, nicht 27017/27018', () => {
    expect(isISO27001('ISO27001')).toBe(true)
    expect(isISO27001('iso-27001')).toBe(true)
    expect(isISO27001('ISO27017')).toBe(false)
    expect(isISO27001(undefined)).toBe(false)
  })

  it('vergleicht Kontroll-IDs numerisch', () => {
    expect(['A.8.1', 'A.5.10', 'A.5.9'].sort(compareControlIds)).toEqual(['A.5.9', 'A.5.10', 'A.8.1'])
  })
})

describe('FrameworkDetailPage — Gruppierung (UX-19)', () => {
  it('zeigt ISO 27001 unter denselben Kapitelüberschriften wie die SoA', () => {
    renderPage('ISO27001', ISO_CONTROLS)
    for (const h of ['A.5 Organisatorische Maßnahmen', 'A.6 Personenbezogene Maßnahmen', 'A.7 Physische Maßnahmen', 'A.8 Technologische Maßnahmen']) {
      expect(screen.getByText(h)).toBeInTheDocument()
    }
    expect(screen.queryByText('Zugangskontrolle')).not.toBeInTheDocument()
    const rows = screen.getAllByRole('row').map((r) => r.textContent ?? '').filter((t) => t.startsWith('A.') || /A\.\d/.test(t))
    const ids = rows.map((t) => /A\.\d+\.\d+/.exec(t)?.[0])
    expect(ids).toEqual(['A.5.2', 'A.5.10', 'A.5.15', 'A.6.3', 'A.7.1', 'A.8.5'])
  })

  it('behält für NIS2 die Themen', () => {
    renderPage('NIS2', [ctrl('NIS2-A.1', 'Governance'), ctrl('NIS2-B.1', 'Risikomanagement')])
    expect(screen.getByText('Governance')).toBeInTheDocument()
    expect(screen.getByText('Risikomanagement')).toBeInTheDocument()
  })
})
