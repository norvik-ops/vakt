/**
 * UX-24: „KI-Entwurf generieren" wurde angeboten, obwohl die KI aus war
 * (`/vaktcomply/ai/status` liefert dann 404, die /ai/*-Routen sind nicht
 * registriert) — der Dialog öffnete sich trotzdem. Jetzt gilt für alle
 * KI-Knöpfe: nur anbieten, wenn der KI-Anbieter erreichbar ist.
 */
import { describe, it, expect, vi, beforeEach } from 'vitest'
import { render, screen, waitFor } from '@testing-library/react'
import { MemoryRouter, Route, Routes } from 'react-router-dom'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import PoliciesPage from './PoliciesPage'
import RiskDetailPage from './RiskDetailPage'
import { signInAs } from '../../../test-utils/signInAs'

let aiAvailable = false
let narrative: string | null = null
const statusCalls: string[] = []

const RISK = {
  id: 'r-1', org_id: 'o-1', title: 'Ransomware verschlüsselt den Fileserver', description: '', category: 'Cyber',
  likelihood: 4, impact: 5, risk_score: 20, owner: 'CISO', status: 'open', treatment: 'mitigate',
  treatment_notes: '', treatment_option: 'mitigate', treatment_plan: '', treatment_owner: '',
  treatment_due_date: null, treatment_status: 'pending',
  residual_likelihood: 2, residual_impact: 2, residual_score: 4,
  created_at: '2026-11-17T10:00:00Z', updated_at: '2026-11-17T10:00:00Z',
}

vi.mock('../../../api/client', () => ({
  apiFetch: (path: string) => {
    if (path === '/vaktcomply/ai/status') {
      statusCalls.push(path)
      return aiAvailable
        ? Promise.resolve({ available: true, model: 'qwen2.5:7b', provider_host: 'ollama' })
        : Promise.reject(Object.assign(new Error('Not Found'), { status: 404 }))
    }
    if (path.startsWith('/vaktcomply/policies?')) {
      return Promise.resolve({ data: [], pagination: { page: 1, limit: 25, total: 0, total_pages: 1 } })
    }
    if (path === '/vaktcomply/risks/r-1') return Promise.resolve({ ...RISK, ai_narrative: narrative })
    return Promise.resolve([])
  },
  registerUnauthorizedHandler: vi.fn(),
}))

function renderAt(path: string) {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } })
  return render(
    <QueryClientProvider client={client}>
      <MemoryRouter initialEntries={[path]}>
        <Routes>
          <Route path="/vaktcomply/policies" element={<PoliciesPage />} />
          <Route path="/vaktcomply/risks/:id" element={<RiskDetailPage />} />
        </Routes>
      </MemoryRouter>
    </QueryClientProvider>,
  )
}

beforeEach(() => {
  aiAvailable = false
  narrative = null
  statusCalls.length = 0
  signInAs('Admin')
})

describe('KI-Knöpfe ohne KI (UX-24)', () => {
  it('Richtlinien: „KI-Entwurf generieren" fehlt, wenn /ai/status 404 liefert', async () => {
    renderAt('/vaktcomply/policies')
    await screen.findByRole('button', { name: /Neue Richtlinie|Richtlinie erstellen|Richtlinie anlegen/ })
    await waitFor(() => { expect(statusCalls.length).toBeGreaterThan(0) })
    expect(screen.queryByRole('button', { name: /KI-Entwurf generieren/ })).toBeNull()
  })

  it('Richtlinien: mit laufender KI ist der Knopf da', async () => {
    aiAvailable = true
    renderAt('/vaktcomply/policies')
    expect(await screen.findByRole('button', { name: /KI-Entwurf generieren/ })).toBeInTheDocument()
  })

  it('Risiko: ohne KI und ohne frühere Narrative kein KI-Kasten', async () => {
    renderAt('/vaktcomply/risks/r-1')
    await screen.findByRole('heading', { name: RISK.title, level: 1 })
    await waitFor(() => { expect(statusCalls.length).toBeGreaterThan(0) })
    expect(screen.queryByText('KI-Risikonarrative')).toBeNull()
    expect(screen.queryByRole('button', { name: /KI-Narrative generieren/ })).toBeNull()
  })

  it('Risiko: eine frühere Narrative bleibt lesbar, aber ohne Erzeugen-Knopf', async () => {
    narrative = 'Früher erzeugter Text'
    renderAt('/vaktcomply/risks/r-1')
    expect(await screen.findByText('Früher erzeugter Text')).toBeInTheDocument()
    await waitFor(() => { expect(statusCalls.length).toBeGreaterThan(0) })
    expect(screen.queryByRole('button', { name: /Neu generieren|KI-Narrative generieren/ })).toBeNull()
  })

  it('Risiko: mit laufender KI ist der Knopf da', async () => {
    aiAvailable = true
    renderAt('/vaktcomply/risks/r-1')
    expect(await screen.findByRole('button', { name: /KI-Narrative generieren/ })).toBeInTheDocument()
  })
})
