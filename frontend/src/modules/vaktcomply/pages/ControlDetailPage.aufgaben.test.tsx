/**
 * UX-32: Die Kontrollseite hatte drei Aufgabenlisten — „Umsetzungsschritte",
 * „Empfohlene Maßnahmen", „Aufgaben" — ohne Erklärung, verteilt über die Seite.
 * Sie sind fachlich verschieden (Katalog-Vorschläge / Checkliste / zugewiesene
 * Aufgaben mit Person und Frist), stehen jetzt beieinander und sagen je in einem
 * Satz, wofür sie da sind.
 */
import { describe, it, expect, vi } from 'vitest'
import { render, screen } from '@testing-library/react'
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

const uploadMutate = vi.fn()
const addMutate = vi.fn()

vi.mock('../hooks/useMeasures', () => ({
  useMeasures: () => ({ data: [], isLoading: false }),
  useCreateMeasure: () => ({ mutate: vi.fn(), isPending: false }),
  useDeleteMeasure: () => ({ mutate: vi.fn(), isPending: false }),
}))

vi.mock('../hooks/useTasks', () => ({
  useTasks: () => ({ data: [], isLoading: false }),
  useCreateTask: () => ({ mutate: vi.fn(), isPending: false }),
  useUpdateTask: () => ({ mutate: vi.fn(), isPending: false }),
  useDeleteTask: () => ({ mutate: vi.fn(), isPending: false }),
}))

vi.mock('../hooks/useControls', () => ({
  useControl: () => ({ data: CONTROL, isLoading: false }),
  useAddEvidence: () => ({ mutate: addMutate, isPending: false }),
  useUploadEvidence: () => ({ mutate: uploadMutate, isPending: false }),
  useCollectEvidence: () => ({ mutate: vi.fn(), isPending: false }),
  useExportControl: () => () => {},
  useUpdateControl: () => ({ mutate: vi.fn(), isPending: false }),
}))

vi.mock('../hooks/useEvidence', () => ({
  useEvidence: () => ({
    data: [{
      id: 'ev-1', control_id: 'ctrl-a52', org_id: 'org-1', title: 'Offener Nachweis',
      source: 'manual', status: 'pending', version: 1,
      created_at: '2026-11-17T10:00:00Z', updated_at: '2026-11-17T10:00:00Z',
    }],
    isLoading: false,
  }),
  useReviewEvidence: () => ({ mutate: vi.fn(), isPending: false }),
}))

vi.mock('../hooks/useFrameworks', () => ({
  useFrameworkControls: () => ({ data: [], isLoading: false }),
  useFramework: () => ({ data: undefined }),
}))

vi.mock('../hooks/useControlTasks', () => ({
  useControlTasks: () => ({ data: [], isLoading: false }),
  useCreateControlTask: () => ({ mutate: vi.fn(), isPending: false }),
  useToggleControlTask: () => ({ mutate: vi.fn(), isPending: false }),
  useDeleteControlTask: () => ({ mutate: vi.fn(), isPending: false }),
}))

function renderPage() {
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

describe('ControlDetailPage — Aufgabenlisten (UX-32)', () => {
  it('zeigt Umsetzungshilfe, Umsetzungsschritte und zugewiesene Aufgaben direkt nacheinander, je mit Zweck', () => {
    signInAs('Admin')
    const { container } = renderPage()
    const titles = [
      i18n.t('vaktcomply.measuresList.panelTitle'),
      i18n.t('vaktcomply.controlDetailPage.implementationSteps'),
      i18n.t('vaktcomply.tasksPanel.panelTitle'),
    ]
    // Reihenfolge im Dokument = Reihenfolge auf der Seite.
    const text = container.textContent ?? ''
    const pos = titles.map((t) => text.indexOf(t))
    expect(pos.every((p) => p >= 0)).toBe(true)
    expect([...pos].sort((a, b) => a - b)).toEqual(pos)
    // Nichts anderes dazwischen: der Abstand ist kleiner als z. B. der Ausnahmen-Block.
    const exceptions = text.indexOf(i18n.t('vaktcomply.controlDetailPage.exceptionsTitle'))
    expect(exceptions).toBeGreaterThan(pos[2])
    for (const hint of ['vaktcomply.measuresList.panelHint', 'vaktcomply.controlDetailPage.implementationStepsHint', 'vaktcomply.tasksPanel.panelHint']) {
      expect(screen.getByText(i18n.t(hint))).toBeInTheDocument()
    }
    // Der alte, mehrdeutige Titel ist weg.
    expect(screen.queryByText('Empfohlene Maßnahmen')).toBeNull()
  })
})
