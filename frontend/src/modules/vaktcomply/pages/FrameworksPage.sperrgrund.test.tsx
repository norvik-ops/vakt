/**
 * UX-12: Gesperrte Katalogkarten sagen sichtbar, warum sie gesperrt sind —
 * vorher stand der Grund nur im Tooltip eines ausgegrauten Knopfs.
 *   - nicht im Angebot (TISAX, DORA, ISO 42001) → „in keinem Tarif"
 *   - Entwurf (prEN 18286)                     → „noch nicht veröffentlicht"
 *   - Pro-Framework ohne Pro-Lizenz            → „Braucht eine Vakt-Pro-Lizenz" + Link
 *   - Pro-Framework mit Lizenz / ohne Gate     → kein Hinweis
 */
import { describe, it, expect, vi } from 'vitest'
import { render, within } from '@testing-library/react'
import { MemoryRouter } from 'react-router-dom'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import FrameworksPage from './FrameworksPage'
import { signInAs } from '../../../test-utils/signInAs'

interface License { tier: string; is_pro: boolean; features: string[] }
const license = vi.hoisted(() => ({ value: undefined as License | undefined }))

vi.mock('../../../shared/hooks/useFeature', () => ({
  useLicenseStatus: () => ({ data: license.value, isLoading: false }),
  useFeature: () => ({ enabled: true, loading: false }),
}))

vi.mock('../hooks/useFrameworks', () => ({
  useFrameworks: () => ({ data: [], isLoading: false, isError: false }),
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
  it('Community: unverkauft, Entwurf und Pro tragen je ihren Grund, NIS2/ISO keinen', () => {
    license.value = { tier: 'community', is_pro: false, features: [] }
    const { container } = renderPage()
    for (const key of ['TISAX', 'DORA']) {
      expect(within(card(container, key)).getByTestId('feature-lock-hint-unsold')).toHaveTextContent('in keinem Tarif')
    }
    expect(within(card(container, 'prEN18286')).getByTestId('feature-lock-hint-draft')).toHaveTextContent('noch nicht veröffentlicht')
    const pro = within(card(container, 'BSI')).getByTestId('feature-lock-hint-pro')
    expect(pro).toHaveTextContent('Braucht eine Vakt-Pro-Lizenz')
    expect(within(pro).getByRole('link', { name: 'Lizenz aktivieren' })).toHaveAttribute('href', '/settings')
    for (const key of ['NIS2', 'ISO27001']) {
      expect(card(container, key).querySelector('[data-testid^="feature-lock-hint"]')).toBeNull()
    }
  })

  it('Pro mit dem Merkmal: kein Pro-Hinweis, unverkaufte Karten bleiben erklärt', () => {
    license.value = { tier: 'pro', is_pro: true, features: ['bsi_grundschutz', 'eu_ai_act', 'cra', 'multi_framework'] }
    const { container } = renderPage()
    expect(container.querySelector('[data-testid="feature-lock-hint-pro"]')).toBeNull()
    expect(within(card(container, 'TISAX')).getByTestId('feature-lock-hint-unsold')).toBeInTheDocument()
  })

  it('Lizenz noch nicht geladen: kein Pro-Hinweis (kein Flackern für Pro-Kunden)', () => {
    license.value = undefined
    const { container } = renderPage()
    expect(container.querySelector('[data-testid="feature-lock-hint-pro"]')).toBeNull()
  })

  it('Viewer sieht den Grund auch ohne Aktivieren-Knopf', () => {
    license.value = { tier: 'community', is_pro: false, features: [] }
    const { container } = renderPage('Viewer')
    expect(within(card(container, 'TISAX')).getByTestId('feature-lock-hint-unsold')).toBeInTheDocument()
    expect(within(card(container, 'TISAX')).queryByRole('button')).toBeNull()
  })
})
