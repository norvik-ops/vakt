/**
 * „Nachweis hinzufügen" mit Datei (UX-30) und Schreibknöpfe nach Rolle (UX-06).
 *
 * - Eine Datei im Dialog geht über den bestehenden Upload-Pfad
 *   (POST /controls/:id/evidence/upload → Anhang des neuen Nachweises), auch beim
 *   Typ „Manuell" — vorher gab es das Dateifeld nur hinter Typ „Dokument".
 * - Ohne Datei bleibt es beim JSON-Nachweis (POST /controls/:id/evidence).
 * - Ein Viewer sieht weder „Nachweis hinzufügen" noch „Prüfen"; das Backend
 *   bleibt die Sperre (evidence_register_test.go), die Oberfläche spart den Klick.
 */
import { describe, it, expect, vi, beforeEach } from 'vitest'
import { render, screen, fireEvent, within } from '@testing-library/react'
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

const k = (key: string) => i18n.t(`vaktcomply.controlDetailPage.${key}`)

function openDialog(): HTMLElement {
  fireEvent.click(screen.getByRole('button', { name: k('addEvidence') }))
  return screen.getByRole('dialog', { name: k('addEvidenceTitle') })
}

beforeEach(() => {
  uploadMutate.mockReset()
  addMutate.mockReset()
})

describe('ControlDetailPage — Nachweis mit Datei', () => {
  it('lädt eine im Dialog gewählte Datei über den Upload-Pfad hoch, auch beim Typ „Manuell"', () => {
    signInAs('SecurityAnalyst')
    renderPage()
    const dlg = openDialog()
    fireEvent.change(within(dlg).getByLabelText(k('evidenceLabelTitle')), { target: { value: 'Rollenkonzept' } })
    const file = new File(['%PDF-1.4'], 'rollen.pdf', { type: 'application/pdf' })
    fireEvent.change(within(dlg).getByLabelText(k('evidenceLabelFileOptional')), { target: { files: [file] } })
    fireEvent.click(within(dlg).getByRole('button', { name: k('addEvidenceSubmit') }))

    expect(addMutate).not.toHaveBeenCalled()
    expect(uploadMutate).toHaveBeenCalledTimes(1)
    const fd = uploadMutate.mock.calls[0][0] as FormData
    expect(fd.get('title')).toBe('Rollenkonzept')
    expect((fd.get('file') as File).name).toBe('rollen.pdf')
  })

  it('legt ohne Datei einen Nachweis ohne Upload an', () => {
    signInAs('Admin')
    renderPage()
    const dlg = openDialog()
    fireEvent.change(within(dlg).getByLabelText(k('evidenceLabelTitle')), { target: { value: 'Protokoll' } })
    fireEvent.click(within(dlg).getByRole('button', { name: k('addEvidenceSubmit') }))
    expect(uploadMutate).not.toHaveBeenCalled()
    expect(addMutate).toHaveBeenCalledTimes(1)
    expect(addMutate.mock.calls[0][0]).toMatchObject({ title: 'Protokoll', source: 'manual' })
  })
})

describe('ControlDetailPage — Viewer', () => {
  it('zeigt einem Viewer keine Schreibknöpfe, aber den Hinweis auf Nur-Lesen', () => {
    signInAs('Viewer')
    renderPage()
    expect(screen.queryByRole('button', { name: k('addEvidence') })).toBeNull()
    expect(screen.queryByRole('button', { name: k('review') })).toBeNull()
    expect(screen.getByRole('note')).toHaveTextContent(i18n.t('common.readOnlyHint'))
    // Status selection is visible but cannot be used.
    const fieldsets = screen.getAllByTestId('write-fieldset')
    expect(fieldsets.length).toBeGreaterThan(0)
    for (const fs of fieldsets) expect(fs).toBeDisabled()
  })

  it('zeigt einem Editor die Schreibknöpfe und keinen Hinweis', () => {
    signInAs('SecurityAnalyst')
    renderPage()
    expect(screen.getByRole('button', { name: k('addEvidence') })).toBeEnabled()
    expect(screen.queryByRole('note')).toBeNull()
  })
})
