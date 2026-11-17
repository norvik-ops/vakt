/**
 * UX-06 (Kernweg 3): Ein Viewer sah „Risiko erfassen", „Aus Katalog" und den
 * Papierkorb an jedem Risiko — die Absage kam erst nach dem Klick. Die Knöpfe
 * richten sich jetzt nach derselben Rollenliste wie das Backend
 * (auth.RequireRole("Admin", "SecurityAnalyst")); das Backend bleibt die Sperre.
 */
import { describe, it, expect, vi } from 'vitest'
import { render, screen } from '@testing-library/react'
import { renderHook } from '@testing-library/react'
import { MemoryRouter } from 'react-router-dom'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import i18n from '../../../i18n'
import RisksPage from './RisksPage'
import { signInAs } from '../../../test-utils/signInAs'
import { useCanWrite, canWriteWithRoles } from '../../../shared/hooks/useCanWrite'
import { useAuthStore } from '../../../shared/stores/auth'

const RISK = {
  id: 'r-1', org_id: 'o-1', title: 'Ransomware', description: '', category: 'IT',
  likelihood: 4, impact: 5, risk_score: 20, owner: '', status: 'open', treatment: 'mitigate',
  created_at: '2026-11-17T10:00:00Z', updated_at: '2026-11-17T10:00:00Z',
}

vi.mock('../../../api/client', () => ({
  apiFetch: (path: string) => Promise.resolve(
    path.startsWith('/vaktcomply/risks')
      ? { data: [RISK], pagination: { page: 1, limit: 25, total: 1, total_pages: 1 } }
      : [],
  ),
  registerUnauthorizedHandler: vi.fn(),
}))

function renderPage() {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } })
  return render(
    <QueryClientProvider client={client}>
      <MemoryRouter initialEntries={['/vaktcomply/risks']}>
        <RisksPage />
      </MemoryRouter>
    </QueryClientProvider>,
  )
}

const addRisk = () => i18n.t('vaktcomply.risksPage.addRisk')

describe('RisksPage — Schreibknöpfe nach Rolle', () => {
  it('Viewer: kein „Risiko erfassen", kein Papierkorb, keine Auswahl — dafür der Nur-Lesen-Hinweis', async () => {
    signInAs('Viewer')
    renderPage()
    await screen.findByText('Ransomware')
    expect(screen.queryByRole('button', { name: addRisk() })).toBeNull()
    expect(screen.queryByRole('button', { name: /löschen/ })).toBeNull()
    expect(screen.queryByRole('checkbox')).toBeNull()
    expect(screen.getByRole('note')).toHaveTextContent(i18n.t('common.readOnlyHint'))
  })

  it('Editor: Knöpfe da, kein Hinweis', async () => {
    signInAs('SecurityAnalyst')
    renderPage()
    await screen.findByText('Ransomware')
    expect(screen.getByRole('button', { name: addRisk() })).toBeEnabled()
    expect(screen.getByRole('button', { name: /Ransomware.*löschen/ })).toBeInTheDocument()
    expect(screen.queryByRole('note')).toBeNull()
  })
})

describe('useCanWrite', () => {
  it('folgt der Rollenliste der Schreibrouten', () => {
    expect(canWriteWithRoles(['Admin'])).toBe(true)
    expect(canWriteWithRoles(['SecurityAnalyst'])).toBe(true)
    expect(canWriteWithRoles(['Viewer'])).toBe(false)
    expect(canWriteWithRoles(['AuditorReadOnly'])).toBe(false)
    expect(canWriteWithRoles([])).toBe(false)
    expect(canWriteWithRoles(undefined)).toBe(false)
  })

  it('liest die Rolle aus dem Auth-Store; ohne Anmeldung kein Schreiben', () => {
    useAuthStore.setState({ user: null })
    expect(renderHook(() => useCanWrite()).result.current).toBe(false)
    signInAs('Admin')
    expect(renderHook(() => useCanWrite()).result.current).toBe(true)
  })
})
