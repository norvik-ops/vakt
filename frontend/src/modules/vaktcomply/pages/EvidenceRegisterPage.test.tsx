/**
 * Seite „Nachweise" (UX-29, Kernweg 9): alle Nachweise der Organisation mit
 * Kontrolle, Framework, Datei ja/nein und Filtern. Die Zeilen haben die Form,
 * die GET /vaktcomply/evidence sendet (EvidenceListItem in openapi.yaml).
 */
import { describe, it, expect, vi } from 'vitest'
import { render, screen, fireEvent, within } from '@testing-library/react'
import { MemoryRouter } from 'react-router-dom'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import i18n from '../../../i18n'
import EvidenceRegisterPage from './EvidenceRegisterPage'
import type { EvidenceListItem } from '../hooks/useEvidenceRegister'

const soon = new Date(Date.now() + 5 * 86_400_000).toISOString()

const ROWS: EvidenceListItem[] = [
  {
    id: 'ev-1', title: 'Rollenkonzept 2026', source: 'manual', status: 'approved', version: 1,
    created_at: '2026-11-17T10:00:00Z', expires_at: soon,
    control_id: 'ctl-a52', control_ref: 'A.5.2', control_title: 'Rollen und Verantwortlichkeiten',
    framework_id: 'fw-iso', framework_name: 'ISO 27001', file_count: 2, has_file: true,
  },
  {
    id: 'ev-2', title: 'Protokoll Leitungsrunde', source: 'manual', status: 'pending', version: 1,
    created_at: '2026-11-16T10:00:00Z',
    control_id: 'ctl-nis', control_ref: 'NIS2-A.1', control_title: 'Governance',
    framework_id: 'fw-nis', framework_name: 'NIS2', file_count: 0, has_file: false,
  },
]

const apiFetch = vi.fn()
vi.mock('../../../api/client', () => ({
  apiFetch: (...args: unknown[]) => apiFetch(...args) as unknown,
  registerUnauthorizedHandler: vi.fn(),
}))

function renderPage() {
  apiFetch.mockImplementation((path: string) => {
    if (path === '/vaktcomply/evidence') return Promise.resolve(ROWS)
    return Promise.resolve([])
  })
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } })
  return render(
    <QueryClientProvider client={client}>
      <MemoryRouter initialEntries={['/vaktcomply/evidence']}>
        <EvidenceRegisterPage />
      </MemoryRouter>
    </QueryClientProvider>,
  )
}

const k = (key: string, opts?: Record<string, unknown>) => i18n.t(`vaktcomply.evidenceRegister.${key}`, opts)

function row(title: string): HTMLElement {
  const r = screen.getByText(title).closest('tr')
  if (!r) throw new Error(`no row for ${title}`)
  return r
}

describe('EvidenceRegisterPage', () => {
  it('listet alle Nachweise mit Kontrolle (verlinkt), Framework und Datei-Angabe', async () => {
    renderPage()
    await screen.findByText('Rollenkonzept 2026')
    expect(apiFetch).toHaveBeenCalledWith('/vaktcomply/evidence')

    const withFile = row('Rollenkonzept 2026')
    const link = within(withFile).getByRole('link')
    expect(link).toHaveAttribute('href', '/vaktcomply/controls/ctl-a52?frameworkId=fw-iso')
    expect(link).toHaveTextContent('A.5.2')
    expect(withFile).toHaveTextContent('ISO 27001')
    expect(withFile).toHaveTextContent(k('fileYes'))
    expect(withFile).toHaveTextContent('(2)')

    const noFile = row('Protokoll Leitungsrunde')
    expect(noFile).toHaveTextContent('NIS2')
    expect(noFile).toHaveTextContent(k('fileNo'))
    expect(screen.getByText(k('count', { shown: 2, total: 2 }))).toBeInTheDocument()
  })

  it('filtert auf Nachweise ohne Datei', async () => {
    renderPage()
    await screen.findByText('Rollenkonzept 2026')
    fireEvent.click(screen.getByLabelText(k('filterNoFile', { count: 1 })))
    expect(screen.queryByText('Rollenkonzept 2026')).toBeNull()
    expect(screen.getByText('Protokoll Leitungsrunde')).toBeInTheDocument()
  })

  it('filtert auf ablaufende Nachweise und nach Framework', async () => {
    renderPage()
    await screen.findByText('Rollenkonzept 2026')
    fireEvent.click(screen.getByLabelText(k('filterExpiring', { days: 30 })))
    expect(screen.getByText('Rollenkonzept 2026')).toBeInTheDocument()
    expect(screen.queryByText('Protokoll Leitungsrunde')).toBeNull()

    fireEvent.click(screen.getByLabelText(k('filterExpiring', { days: 30 })))
    fireEvent.change(screen.getByLabelText(k('filterFramework')), { target: { value: 'fw-nis' } })
    expect(screen.queryByText('Rollenkonzept 2026')).toBeNull()
    expect(screen.getByText('Protokoll Leitungsrunde')).toBeInTheDocument()
  })

  it('verlinkt die automatisch gesammelten Belege als zweiten Reiter', async () => {
    renderPage()
    await screen.findByText('Rollenkonzept 2026')
    expect(screen.getByRole('link', { name: k('tabAuto') })).toHaveAttribute('href', '/vaktcomply/evidence/auto')
    expect(screen.getByRole('link', { name: k('tabAll') })).toHaveAttribute('aria-current', 'page')
  })
})
