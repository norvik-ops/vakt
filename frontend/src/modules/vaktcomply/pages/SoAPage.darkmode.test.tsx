/**
 * Dunkelmodus-Regression der SoA-Seite (Kernweg 6, gemeldet 2026-11-17 auf v0.46.5).
 *
 * Die Tabellenzellen erben `text-primary` aus `components/ui/table.tsx` — im
 * Dunkelmodus fast weiß. Der Tabellen-Container und die Karte „Kontrollen gesamt“
 * trugen ein fest verdrahtetes `bg-white` ohne `dark:`-Variante. Ergebnis: weiße
 * Schrift auf weißem Grund, Maßnahmen-Namen und die Zahl 93 unsichtbar.
 *
 * jsdom wendet kein CSS an, also prüft dieser Test die Klassen: Kein Vorfahre der
 * Namenszelle bzw. der Kennzahl darf einen festen hellen Hintergrund ohne
 * `dark:bg-*`-Gegenstück tragen, und der Text selbst nutzt das Theme-Token.
 * Was er NICHT prüft: den tatsächlichen Kontrast im Browser.
 */
import { describe, it, expect, vi, beforeEach } from 'vitest'
import { render, screen } from '@testing-library/react'
import { MemoryRouter } from 'react-router-dom'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import SoAPage from './SoAPage'

vi.mock('react-i18next', () => ({
  useTranslation: () => ({ t: (k: string) => k }),
}))

vi.mock('../../../api/client', () => ({ apiFetch: vi.fn(), registerUnauthorizedHandler: vi.fn() }))
const { apiFetch } = await import('../../../api/client')
const mockApiFetch = vi.mocked(apiFetch)

const ENTRY = {
  id: 'e-1',
  org_id: 'org-1',
  version: 1,
  control_ref: 'A.5.1',
  control_name: 'Informationssicherheitsrichtlinien',
  control_group: '5',
  applicable: true,
  justification: '',
  exclusion_reason: '',
  implementation_status: 'not_started',
  manually_set: false,
  ck_control_id: null,
  evidence_reference: '',
  is_approved: false,
  notes: '',
  created_at: '2026-01-01T00:00:00Z',
  updated_at: '2026-01-01T00:00:00Z',
}

const SUMMARY = {
  version: 1,
  status: 'draft',
  applicable_count: 1,
  excluded_count: 0,
  implemented_count: 0,
  partial_count: 0,
  planned_count: 0,
  not_started_count: 1,
  implementation_pct: 0,
}

// Feste helle Hintergründe, die im Dunkelmodus hell bleiben, wenn kein
// `dark:bg-*` daneben steht.
const FIXED_LIGHT_BG = /^bg-(white|gray-50|gray-100|slate-50|slate-100)$/

function lightOnlyBackgrounds(el: HTMLElement): string[] {
  const offenders: string[] = []
  for (let n: HTMLElement | null = el; n; n = n.parentElement) {
    const classes = Array.from(n.classList)
    const hasDarkBg = classes.some(c => c.startsWith('dark:bg-'))
    for (const c of classes) {
      if (FIXED_LIGHT_BG.test(c) && !hasDarkBg) offenders.push(`<${n.tagName.toLowerCase()}> ${c}`)
    }
  }
  return offenders
}

function renderPage() {
  mockApiFetch.mockImplementation((path: string) => {
    if (path.endsWith('/soa/summary')) return Promise.resolve(SUMMARY as never)
    return Promise.resolve([ENTRY] as never)
  })
  const client = new QueryClient({ defaultOptions: { queries: { retry: false, gcTime: 0 } } })
  return render(
    <QueryClientProvider client={client}>
      <MemoryRouter><SoAPage /></MemoryRouter>
    </QueryClientProvider>,
  )
}

beforeEach(() => { mockApiFetch.mockReset() })

describe('SoAPage — Dunkelmodus lesbar', () => {
  it('Maßnahmen-Name nutzt das Text-Token und liegt auf keinem fest weißen Grund', async () => {
    renderPage()
    const name = await screen.findByTestId('soa-control-name')
    expect(name.textContent).toBe('Informationssicherheitsrichtlinien')
    expect(name.classList).toContain('text-primary')
    expect(lightOnlyBackgrounds(name)).toEqual([])
  })

  it('Kennzahl „Kontrollen gesamt“ nutzt Token für Text und Karte', async () => {
    renderPage()
    await screen.findByTestId('soa-control-name')
    const card = screen.getByTestId('soa-stat-total')
    const number = card.firstElementChild as HTMLElement
    expect(number.textContent).toBe('1')
    expect(number.classList).toContain('text-primary')
    expect(card.classList).toContain('bg-surface')
    expect(lightOnlyBackgrounds(number)).toEqual([])
  })

  it('Status-Badge bricht nicht um und hat eine Dunkelmodus-Variante', async () => {
    renderPage()
    await screen.findByTestId('soa-control-name')
    const badge = screen.getByText('vaktcomply.controlDetailPage.statusMissing')
    expect(badge.classList).toContain('whitespace-nowrap')
    expect(Array.from(badge.classList).some(c => c.startsWith('dark:bg-'))).toBe(true)
  })
})
