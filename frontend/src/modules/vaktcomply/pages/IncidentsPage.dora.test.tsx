/**
 * Entscheid 2026-11-18: Frameworks, die Vakt in keinem Tarif verkauft (DORA, TISAX,
 * ISO 42001), werden nirgends mehr zur Auswahl angeboten. Beim Vorfall heißt das:
 * Vorfalltyp „DORA" nur, wenn DORA schon aktiv ist (Bestandsdaten bleiben nutzbar).
 */
import { describe, it, expect, vi, afterEach, beforeEach } from 'vitest'
import { render, screen, cleanup, fireEvent, within } from '@testing-library/react'
import { MemoryRouter } from 'react-router-dom'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import i18n from '../../../i18n'
import { signInAs } from '../../../test-utils/signInAs'
import IncidentsPage from './IncidentsPage'

const frameworks = vi.hoisted(() => ({ value: [] as { id: string; name: string }[] }))

vi.mock('../../../api/client', () => ({
  apiFetch: (path: string) => {
    if (path.startsWith('/vaktcomply/incidents')) {
      return Promise.resolve({ data: [], pagination: { page: 1, limit: 25, total: 0, total_pages: 0 } })
    }
    if (path === '/vaktcomply/frameworks') return Promise.resolve(frameworks.value)
    return Promise.resolve([])
  },
  registerUnauthorizedHandler: vi.fn(),
  FeatureLockedError: class extends Error {},
}))

function renderPage() {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } })
  return render(
    <QueryClientProvider client={client}>
      <MemoryRouter>
        <IncidentsPage />
      </MemoryRouter>
    </QueryClientProvider>,
  )
}

async function openTypeSelect() {
  const create = await screen.findAllByRole('button', { name: new RegExp(i18n.t('vaktcomply.incidentsPage.createIncident')) })
  fireEvent.click(create[0])
  const dialog = await screen.findByRole('dialog')
  const trigger = within(dialog)
    .getAllByRole('combobox')
    .find((el) => el.textContent?.includes(i18n.t('vaktcomply.incidentsPage.typeGeneral')))
  if (!trigger) throw new Error('Auswahl Vorfalltyp fehlt')
  fireEvent.click(trigger)
}

beforeEach(() => {
  signInAs('Admin')
  // jsdom kennt scrollIntoView nicht; die Radix-Auswahl ruft es beim Öffnen auf.
  Element.prototype.scrollIntoView = vi.fn()
})
afterEach(cleanup)

describe('Vorfall anlegen: Vorfalltyp DORA nur bei aktivem DORA', () => {
  it('ohne aktives DORA: NIS2 wählbar, DORA nicht', async () => {
    frameworks.value = [{ id: 'f-1', name: 'NIS2' }]
    renderPage()
    await openTypeSelect()
    expect(await screen.findByRole('option', { name: i18n.t('vaktcomply.incidentsPage.typeNIS2') })).toBeInTheDocument()
    expect(screen.queryByRole('option', { name: i18n.t('vaktcomply.incidentsPage.typeDORA') })).toBeNull()
  })

  it('mit aktivem DORA (Bestand): DORA bleibt wählbar', async () => {
    frameworks.value = [{ id: 'f-1', name: 'NIS2' }, { id: 'f-2', name: 'DORA' }]
    renderPage()
    // Erst warten, bis die Framework-Liste geladen ist.
    await new Promise((r) => setTimeout(r, 0))
    await openTypeSelect()
    expect(await screen.findByRole('option', { name: i18n.t('vaktcomply.incidentsPage.typeDORA') })).toBeInTheDocument()
  })
})
