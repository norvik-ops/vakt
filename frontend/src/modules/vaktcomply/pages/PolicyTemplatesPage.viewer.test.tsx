/**
 * Vorlagenbibliothek: Lesen dürfen alle, „Diese Vorlage verwenden“ legt eine
 * Richtlinie an (POST /vaktcomply/policies, rw-Guard) und gehört nur Admin und
 * SecurityAnalyst. Leser sehen die Vorschau ohne den Knopf und den Hinweis
 * „Nur Lesezugriff“.
 */
import { describe, it, expect, vi } from 'vitest'
import { render, screen, fireEvent } from '@testing-library/react'
import { MemoryRouter } from 'react-router-dom'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import i18n from '../../../i18n'
import PolicyTemplatesPage from './PolicyTemplatesPage'
import { signInAs } from '../../../test-utils/signInAs'

vi.mock('../../../api/client', async (importOriginal) => ({
  ...(await importOriginal<typeof import('../../../api/client')>()),
  apiFetch: vi.fn(() => Promise.resolve([{
    id: 'tpl-1', category: 'policy', name: 'Passwortrichtlinie', description: 'Regeln für Passwörter',
    content: '# Passwortrichtlinie', tags: ['iso27001'], created_at: '2026-11-01T00:00:00Z',
  }])),
}))

vi.mock('../hooks/usePolicies', () => ({
  useCreatePolicy: () => ({ mutate: vi.fn(), isPending: false }),
}))

function renderPage() {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } })
  return render(
    <QueryClientProvider client={client}>
      <MemoryRouter>
        <PolicyTemplatesPage />
      </MemoryRouter>
    </QueryClientProvider>,
  )
}

describe('PolicyTemplatesPage — Schreibknopf nach Rolle', () => {
  it.each(['Viewer', 'AuditorReadOnly'] as const)('%s sieht die Vorschau, aber keinen „Diese Vorlage verwenden“', async (role) => {
    signInAs(role)
    renderPage()
    fireEvent.click(await screen.findByText('Passwortrichtlinie'))
    expect(await screen.findByText('# Passwortrichtlinie')).toBeInTheDocument()
    expect(screen.queryByRole('button', { name: /Diese Vorlage verwenden/ })).toBeNull()
    expect(screen.getByText(i18n.t('common.readOnlyHint'))).toBeInTheDocument()
  })

  it.each(['Admin', 'SecurityAnalyst'] as const)('%s kann die Vorlage verwenden', async (role) => {
    signInAs(role)
    renderPage()
    fireEvent.click(await screen.findByText('Passwortrichtlinie'))
    expect(await screen.findByRole('button', { name: /Diese Vorlage verwenden/ })).toBeInTheDocument()
    expect(screen.queryByText(i18n.t('common.readOnlyHint'))).toBeNull()
  })
})
