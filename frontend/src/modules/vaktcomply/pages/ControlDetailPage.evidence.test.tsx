/**
 * Nachweis-Tabelle gegen die echte Antwortform (Kernweg 9, UX-Durchgang 2026-11-17).
 *
 * GET /vaktcomply/controls/:id/evidence liefert policy.Evidence: `status` ist
 * "pending" (nicht "pending_review"), und es gibt kein Feld `type`, sondern
 * `source` (+ `file_size` bei hochgeladenen Dokumenten). Vorher waren deshalb
 * „Typ" leer und „Status" eine leere Plakette bei jedem Nachweis, und der
 * Prüfen-Knopf erschien nie. Die Zeilen unten sind die Form, die der Handler
 * sendet (json-Tags aus backend/internal/modules/vaktcomply/policy/models.go).
 */
import { describe, it, expect, vi } from 'vitest'
import { render, screen, within } from '@testing-library/react'
import { MemoryRouter, Route, Routes } from 'react-router-dom'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import i18n from '../../../i18n'
import ControlDetailPage from './ControlDetailPage'
import { signInAs } from '../../../test-utils/signInAs'
import type { Control } from '../types'

const CONTROL: Control = {
  id: 'ctrl-a52',
  framework_id: 'fw-iso',
  control_id: 'A.5.2',
  title: 'Rollen und Verantwortlichkeiten',
  description: '',
  domain: '5',
  status: 'partial',
  not_applicable: false,
}

// Wire form of policy.Evidence. Deliberately typed loosely: the point is what
// the backend actually sends, including a status the frontend does not know.
const EVIDENCE_ON_THE_WIRE = [
  {
    id: 'ev-doc', control_id: 'ctrl-a52', org_id: 'org-1', title: 'Rollenkonzept 2026',
    source: 'manual', file_size: 2048, status: 'pending', version: 1,
    created_at: '2026-11-17T10:00:00Z', updated_at: '2026-11-17T10:00:00Z',
  },
  {
    id: 'ev-manual', control_id: 'ctrl-a52', org_id: 'org-1', title: 'Protokoll Leitungsrunde',
    description: 'Freigabe der Rollen', source: 'manual', status: 'approved', version: 1,
    created_at: '2026-11-16T10:00:00Z', updated_at: '2026-11-16T10:00:00Z',
  },
  {
    id: 'ev-ci', control_id: 'ctrl-a52', org_id: 'org-1', title: 'CI-Lauf',
    source: 'ci_webhook', status: 'rejected', version: 1,
    created_at: '2026-11-15T10:00:00Z', updated_at: '2026-11-15T10:00:00Z',
  },
  {
    id: 'ev-odd', control_id: 'ctrl-a52', org_id: 'org-1', title: 'Altbestand',
    source: '', status: 'archived', version: 1,
    created_at: '2026-11-14T10:00:00Z', updated_at: '2026-11-14T10:00:00Z',
  },
]

vi.mock('../hooks/useControls', () => ({
  useControl: () => ({ data: CONTROL, isLoading: false }),
  useAddEvidence: () => ({ mutate: vi.fn(), isPending: false }),
  useUploadEvidence: () => ({ mutate: vi.fn(), isPending: false }),
  useCollectEvidence: () => ({ mutate: vi.fn(), isPending: false }),
  useExportControl: () => () => {},
  useUpdateControl: () => ({ mutate: vi.fn(), isPending: false }),
}))

vi.mock('../hooks/useEvidence', () => ({
  useEvidence: () => ({ data: EVIDENCE_ON_THE_WIRE, isLoading: false }),
  useReviewEvidence: () => ({ mutate: vi.fn(), isPending: false }),
}))

vi.mock('../hooks/useFrameworks', () => ({
  useFrameworkControls: () => ({ data: [], isLoading: false }),
  useFramework: () => ({ data: undefined }),
}))

vi.mock('../hooks/useTasks', () => ({
  useTasks: () => ({ data: [], isLoading: false }),
  useCreateTask: () => ({ mutate: vi.fn(), isPending: false }),
  useUpdateTask: () => ({ mutate: vi.fn(), isPending: false }),
  useDeleteTask: () => ({ mutate: vi.fn(), isPending: false }),
}))

function renderPage() {
  signInAs('Admin')
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } })
  return render(
    <QueryClientProvider client={client}>
      <MemoryRouter initialEntries={['/vaktcomply/controls/ctrl-a52?frameworkId=fw-iso']}>
        <Routes>
          <Route path="/vaktcomply/controls/:id" element={<ControlDetailPage />} />
        </Routes>
      </MemoryRouter>
    </QueryClientProvider>,
  )
}

const k = (key: string, opts?: Record<string, string>) => i18n.t(`vaktcomply.controlDetailPage.${key}`, opts)

/** Cells of the evidence row with the given title: [title, type, status, …]. */
function cells(title: string): HTMLElement[] {
  const row = screen.getByText(title).closest('tr')
  if (!row) throw new Error(`no row for ${title}`)
  return within(row as HTMLElement).getAllByRole('cell')
}

describe('ControlDetailPage — Nachweis-Tabelle (echte API-Werte)', () => {
  it('zeigt Typ aus source/file_size und Status aus pending/approved/rejected', () => {
    renderPage()
    const doc = cells('Rollenkonzept 2026')
    expect(doc[1].textContent).toBe(k('evidenceTypeDocumentLabel'))
    expect(doc[2].textContent).toBe(k('evidenceStatusPending'))

    const manual = cells('Protokoll Leitungsrunde')
    expect(manual[1].textContent).toBe(k('evidenceTypeManualLabel'))
    expect(manual[2].textContent).toBe(k('evidenceStatusApproved'))

    const ci = cells('CI-Lauf')
    expect(ci[1].textContent).toBe(k('evidenceTypeAutomatedLabel'))
    expect(ci[2].textContent).toBe(k('evidenceStatusRejected'))
  })

  it('gibt unbekannten Werten ein sichtbares Fallback-Label statt einer leeren Plakette', () => {
    renderPage()
    const odd = cells('Altbestand')
    expect(odd[1].textContent).toBe(k('evidenceTypeUnknownLabel'))
    expect(odd[2].textContent).toBe(k('evidenceStatusUnknown', { status: 'archived' }))
    for (const title of EVIDENCE_ON_THE_WIRE.map((e) => e.title)) {
      const c = cells(title)
      expect(c[1].textContent?.trim(), `Typ von ${title}`).not.toBe('')
      expect(c[2].textContent?.trim(), `Status von ${title}`).not.toBe('')
    }
  })

  it('bietet „Prüfen" nur für den ausstehenden Nachweis an', () => {
    renderPage()
    const review = k('review')
    const pendingRow = screen.getByText('Rollenkonzept 2026').closest('tr') as HTMLElement
    expect(within(pendingRow).getByRole('button', { name: review })).toBeInTheDocument()
    const approvedRow = screen.getByText('Protokoll Leitungsrunde').closest('tr') as HTMLElement
    expect(within(approvedRow).queryByRole('button', { name: review })).toBeNull()
  })
})
