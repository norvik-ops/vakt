import { describe, it, expect, vi, beforeEach } from 'vitest'
import { render, screen, fireEvent } from '@testing-library/react'
import { MemoryRouter, Route, Routes } from 'react-router-dom'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import PolicyDetailPage from './PolicyDetailPage'
import { useAuthStore } from '../../../shared/stores/auth'
import type { Policy } from '../types'

// UX-23: Der Richtlinientext stand als roher Markdown in einem kleinen Feld.
// Jetzt: gerendert lesen, zum Ändern auf den Editor umschalten; Leser sehen nur
// die gerenderte Fassung.

const POLICY_ID = '11111111-2222-3333-4444-555555555555'
const mutate = vi.fn()
let policy: Policy

const TEXT = '# Passwortrichtlinie\n\n## 1. Geltungsbereich\n\n- Mindestlänge: **12 Zeichen**\n\n<script>window.__x = 1</script>'

function makePolicy(over: Partial<Policy> = {}): Policy {
  return {
    id: POLICY_ID,
    org_id: 'org-1',
    title: 'Passwortrichtlinie',
    status: 'draft',
    version: '1.0',
    version_num: 2,
    version_note: '',
    last_updated_by: '',
    created_at: '2026-01-01T00:00:00Z',
    updated_at: '2026-01-02T00:00:00Z',
    ...over,
  }
}

vi.mock('../hooks/usePolicies', () => ({
  usePolicy: () => ({ data: policy, isLoading: false, isError: false }),
  useUpdatePolicy: () => ({ mutate, isPending: false }),
}))

vi.mock('../components/PolicyVersionHistory', () => ({
  default: () => null,
}))

function signIn(roles: string[]) {
  useAuthStore.setState({
    user: { id: 'u-1', email: 'erika@example.test', display_name: 'Erika Muster', roles },
    hydrating: false,
  })
}

function renderPage() {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } })
  return render(
    <QueryClientProvider client={client}>
      <MemoryRouter initialEntries={[`/vaktcomply/policies/${POLICY_ID}`]}>
        <Routes>
          <Route path="/vaktcomply/policies/:id" element={<PolicyDetailPage />} />
        </Routes>
      </MemoryRouter>
    </QueryClientProvider>,
  )
}

beforeEach(() => {
  mutate.mockReset()
  policy = makePolicy({ description: TEXT })
})

describe('PolicyDetailPage — Richtlinientext (UX-23)', () => {
  it('zeigt vorhandenen Text gerendert, ohne Markdown-Zeichen und ohne Skript', () => {
    signIn(['Admin'])
    const { container } = renderPage()
    expect(screen.getByRole('heading', { level: 2, name: 'Passwortrichtlinie' })).toBeInTheDocument()
    expect(screen.getByRole('heading', { level: 3, name: '1. Geltungsbereich' })).toBeInTheDocument()
    expect(screen.getByText('12 Zeichen').tagName).toBe('STRONG')
    const view = screen.getByTestId('markdown-view')
    expect(view.textContent).not.toContain('##')
    expect(container.querySelector('script')).toBeNull()
    expect(screen.queryByRole('textbox', { name: 'Beschreibung / Zweck' })).toBeNull()
  })

  it('schaltet für Schreibrollen auf den Editor um; Änderung erscheint in der Vorschau', () => {
    signIn(['SecurityAnalyst'])
    renderPage()
    fireEvent.click(screen.getByRole('button', { name: 'Bearbeiten' }))
    const editor = screen.getByRole('textbox', { name: 'Beschreibung / Zweck' })
    expect(editor).toHaveValue(TEXT)
    fireEvent.change(editor, { target: { value: '## Neu\n\nText *kursiv*' } })
    fireEvent.click(screen.getByRole('button', { name: 'Vorschau' }))
    expect(screen.getByRole('heading', { level: 3, name: 'Neu' })).toBeInTheDocument()
    expect(screen.getByText('kursiv').tagName).toBe('EM')
  })

  it('öffnet ohne Text direkt den Editor', () => {
    signIn(['Admin'])
    policy = makePolicy({ description: '' })
    renderPage()
    expect(screen.getByRole('textbox', { name: 'Beschreibung / Zweck' })).toBeInTheDocument()
  })

  it('Leser sehen nur die gerenderte Fassung, keinen Umschalter', () => {
    signIn(['Viewer'])
    renderPage()
    expect(screen.getByTestId('markdown-view')).toBeInTheDocument()
    expect(screen.queryByRole('button', { name: 'Bearbeiten' })).toBeNull()
    expect(screen.queryByRole('textbox', { name: 'Beschreibung / Zweck' })).toBeNull()
  })
})
