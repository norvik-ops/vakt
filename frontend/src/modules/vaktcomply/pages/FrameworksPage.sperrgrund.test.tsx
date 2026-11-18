/**
 * UX-12: Gesperrte Katalogkarten sagen sichtbar, warum sie gesperrt sind —
 * vorher stand der Grund nur im Tooltip eines ausgegrauten Knopfs.
 *   - nicht im Angebot (TISAX, DORA, ISO 42001) → seit 2026-11-18 gar nicht im
 *     Katalog, außer schon aktiv (Bestandsdaten bleiben sichtbar)
 *   - Entwurf (prEN 18286)                     → „noch nicht veröffentlicht"
 *   - Pro-Framework ohne Pro-Lizenz            → „Braucht eine Vakt-Pro-Lizenz" + Link
 *   - Pro-Framework mit Lizenz / ohne Gate     → kein Hinweis
 */
import { describe, it, expect, vi, beforeEach } from 'vitest'
import { render, screen, within } from '@testing-library/react'
import i18n from '../../../i18n'
import { MemoryRouter } from 'react-router-dom'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import FrameworksPage from './FrameworksPage'
import { signInAs } from '../../../test-utils/signInAs'

interface License { tier: string; is_pro: boolean; features: string[] }
const license = vi.hoisted(() => ({ value: undefined as License | undefined }))
const enabled = vi.hoisted(() => ({ value: [] as { id: string; name: string; version: string; created_at: string }[] }))

vi.mock('../../../shared/hooks/useFeature', () => ({
  useLicenseStatus: () => ({ data: license.value, isLoading: false }),
  useFeature: () => ({ enabled: true, loading: false }),
}))

vi.mock('../hooks/useFrameworks', () => ({
  useFrameworks: () => ({ data: enabled.value, isLoading: false, isError: false }),
  useEnableFramework: () => ({ mutate: vi.fn(), isPending: false, error: null }),
  useDeleteFramework: () => ({ mutate: vi.fn(), isPending: false }),
  useSwitchDORAVariant: () => ({ mutate: vi.fn(), isPending: false }),
}))

function card(container: HTMLElement, key: string): HTMLElement {
  const el = container.querySelector<HTMLElement>(`[data-framework-key="${key}"]`)
  if (!el) throw new Error(`Karte ${key} fehlt`)
  return el
}

function renderPage(role: 'Admin' | 'Viewer' = 'Admin') {
  signInAs(role)
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } })
  return render(
    <QueryClientProvider client={client}>
      <MemoryRouter>
        <FrameworksPage />
      </MemoryRouter>
    </QueryClientProvider>,
  )
}

describe('FrameworksPage — Sperrgrund auf der Karte (UX-12)', () => {
  beforeEach(() => { enabled.value = [] })

  it('Community: Entwurf und Pro tragen je ihren Grund, NIS2/ISO keinen, unverkaufte fehlen', () => {
    license.value = { tier: 'community', is_pro: false, features: [] }
    const { container } = renderPage()
    for (const key of ['TISAX', 'DORA', 'ISO42001']) {
      expect(container.querySelector(`[data-framework-key="${key}"]`), key).toBeNull()
    }
    expect(container.querySelector('[data-testid="feature-lock-hint-unsold"]')).toBeNull()
    expect(within(card(container, 'prEN18286')).getByTestId('feature-lock-hint-draft')).toHaveTextContent('noch nicht veröffentlicht')
    const pro = within(card(container, 'BSI')).getByTestId('feature-lock-hint-pro')
    expect(pro).toHaveTextContent('Braucht eine Vakt-Pro-Lizenz')
    expect(within(pro).getByRole('link', { name: 'Lizenz aktivieren' })).toHaveAttribute('href', '/settings')
    for (const key of ['NIS2', 'ISO27001']) {
      expect(card(container, key).querySelector('[data-testid^="feature-lock-hint"]')).toBeNull()
    }
  })

  it('Pro mit dem Merkmal: kein Pro-Hinweis, Pro-Karten bleiben, unverkaufte fehlen auch hier', () => {
    license.value = { tier: 'pro', is_pro: true, features: ['bsi_grundschutz', 'eu_ai_act', 'cra', 'multi_framework'] }
    const { container } = renderPage()
    expect(container.querySelector('[data-testid="feature-lock-hint-pro"]')).toBeNull()
    expect(within(card(container, 'BSI')).getByRole('button', { name: /Aktivieren/ })).toBeEnabled()
    expect(container.querySelector('[data-framework-key="TISAX"]')).toBeNull()
  })

  it('Community: Pro-Framework bleibt sichtbar mit Sperrhinweis statt ausgeblendet', () => {
    license.value = { tier: 'community', is_pro: false, features: [] }
    const { container } = renderPage()
    for (const key of ['BSI', 'EUAIACT', 'CRA']) {
      expect(within(card(container, key)).getByTestId('feature-lock-hint-pro')).toBeInTheDocument()
    }
  })

  it('schon aktives unverkauftes Framework (Bestand) bleibt im Katalog und oben sichtbar', () => {
    license.value = { tier: 'community', is_pro: false, features: [] }
    enabled.value = [{ id: 'fw-dora', name: 'DORA', version: '2022/2554', created_at: '2026-01-01T00:00:00Z' }]
    const { container } = renderPage()
    const dora = card(container, 'DORA')
    expect(within(dora).getByText(i18n.t('vaktcomply.frameworksPage.activated'))).toBeInTheDocument()
    expect(within(dora).queryByTestId('feature-lock-hint-unsold')).toBeNull()
    expect(within(dora).getByRole('button', { name: i18n.t('vaktcomply.frameworksPage.view') })).toBeInTheDocument()
    // Oben in „Aktive Frameworks" steht DORA ebenfalls.
    expect(screen.getAllByText('DORA').length).toBeGreaterThanOrEqual(2)
    // Die anderen unverkauften bleiben ausgeblendet.
    expect(container.querySelector('[data-framework-key="TISAX"]')).toBeNull()
  })

  it('Lizenz noch nicht geladen: kein Pro-Hinweis (kein Flackern für Pro-Kunden)', () => {
    license.value = undefined
    const { container } = renderPage()
    expect(container.querySelector('[data-testid="feature-lock-hint-pro"]')).toBeNull()
  })

  it('Viewer sieht den Grund auch ohne Aktivieren-Knopf', () => {
    license.value = { tier: 'community', is_pro: false, features: [] }
    const { container } = renderPage('Viewer')
    expect(within(card(container, 'BSI')).getByTestId('feature-lock-hint-pro')).toBeInTheDocument()
    expect(within(card(container, 'BSI')).queryByRole('button')).toBeNull()
  })
})
