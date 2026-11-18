/**
 * Entscheid 2026-11-18: unverkaufte Frameworks (DORA) werden nicht mehr zur Auswahl
 * angeboten — auch nicht als Filter im Bedrohungskatalog. Ist DORA schon aktiv
 * (Bestandsdaten), bleibt der Filter.
 */
import { describe, it, expect, vi, afterEach, beforeEach } from 'vitest'
import { render, screen, cleanup } from '@testing-library/react'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { signInAs } from '../../../test-utils/signInAs'
import { ThreatCatalogDialog } from './ThreatCatalogDialog'

const frameworks = vi.hoisted(() => ({ value: [] as { id: string; name: string }[] }))

vi.mock('../../../api/client', () => ({
  apiFetch: (path: string) => {
    if (path === '/vaktcomply/frameworks') return Promise.resolve(frameworks.value)
    return Promise.resolve([])
  },
  registerUnauthorizedHandler: vi.fn(),
  FeatureLockedError: class extends Error {},
}))

function renderDialog() {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } })
  return render(
    <QueryClientProvider client={client}>
      <ThreatCatalogDialog open onClose={() => undefined} />
    </QueryClientProvider>,
  )
}

beforeEach(() => { signInAs('Admin') })
afterEach(cleanup)

describe('Bedrohungskatalog: Framework-Filter', () => {
  it('ohne aktives DORA: kein DORA-Filter, die verkauften Frameworks schon', async () => {
    frameworks.value = [{ id: 'f-1', name: 'ISO27001' }]
    renderDialog()
    expect(await screen.findByRole('button', { name: 'ISO 27001' })).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'NIS2' })).toBeInTheDocument()
    expect(screen.queryByRole('button', { name: 'DORA' })).toBeNull()
  })

  it('mit aktivem DORA (Bestand): DORA-Filter bleibt', async () => {
    frameworks.value = [{ id: 'f-1', name: 'DORA' }]
    renderDialog()
    expect(await screen.findByRole('button', { name: 'DORA' })).toBeInTheDocument()
  })
})
