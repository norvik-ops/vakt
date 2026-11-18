/**
 * R1-KW-40 (Entscheid Stefan 2026-11-18): Die Karte „Sektor" zeigte Community-
 * Nutzern „Pro-Feature … Demnächst verfügbar" — eine vierte Form der Pro-Sperre
 * für eine Funktion, die es noch nicht gibt. Sie ist für alle ausgeblendet, bis
 * die Funktion fertig ist. Gegenprobe: die Nachbarkarten derselben Registerkarte
 * stehen da, der Test sieht also die richtige Seite.
 */
import { describe, it, expect, vi } from 'vitest'
import { render, screen } from '@testing-library/react'
import { MemoryRouter } from 'react-router-dom'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import i18n from '../../../i18n'
import Settings from './Settings'
import { signInAs } from '../../../test-utils/signInAs'

const license = vi.hoisted(() => ({ is_pro: false }))

vi.mock('../../../api/client', async (orig) => {
  const real = await orig<typeof import('../../../api/client')>()
  return {
    ...real,
    apiFetch: (path: string) => {
      if (path === '/license') return Promise.resolve({ tier: license.is_pro ? 'pro' : 'community', is_pro: license.is_pro, features: [] })
      return Promise.resolve({})
    },
    registerUnauthorizedHandler: vi.fn(),
  }
})

function renderSettings() {
  signInAs('Admin')
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } })
  return render(
    <QueryClientProvider client={client}>
      <MemoryRouter initialEntries={['/settings?tab=platform']}>
        <Settings />
      </MemoryRouter>
    </QueryClientProvider>,
  )
}

describe('Settings — Karte „Sektor" ausgeblendet (R1-KW-40)', () => {
  for (const isPro of [false, true]) {
    it(`fehlt im DOM (${isPro ? 'Pro' : 'Community'})`, async () => {
      license.is_pro = isPro
      renderSettings()
      expect(await screen.findByText(i18n.t('settingsPage.brandingTitle'))).toBeInTheDocument()
      expect(screen.queryByText(i18n.t('settingsPage.sectorTitle'))).toBeNull()
      expect(screen.queryByText(i18n.t('settingsPage.sectorProFeature'))).toBeNull()
      expect(screen.queryByText(i18n.t('settingsPage.comingSoon'))).toBeNull()
      expect(screen.queryByTestId('sector-select')).toBeNull()
      expect(screen.queryByTestId('sector-save-btn')).toBeNull()
    })
  }
})
