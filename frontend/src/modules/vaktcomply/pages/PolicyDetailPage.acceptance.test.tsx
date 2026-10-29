import { describe, it, expect, vi } from 'vitest'
import { render, screen, fireEvent } from '@testing-library/react'
import { MemoryRouter, Route, Routes } from 'react-router-dom'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import PolicyDetailPage from './PolicyDetailPage'
import type { Policy } from '../types'

// R1-KW-11 (Paket 6): Die Akzeptanz-Kampagnen (Route policies/:id/acceptance) waren
// von keiner Seite aus verlinkt — die Mitarbeiter-Bestätigung erreichte man nur,
// wenn man die URL von Hand tippte. Die Detailseite der Richtlinie führt jetzt hin.

const POLICY_ID = '11111111-2222-3333-4444-555555555555'

function makePolicy(): Policy {
  const now = new Date().toISOString()
  return {
    id: POLICY_ID,
    org_id: 'org-1',
    title: 'Passwortrichtlinie',
    status: 'active',
    version: '1.0',
    version_num: 1,
    version_note: '',
    last_updated_by: '',
    created_at: now,
    updated_at: now,
  }
}

vi.mock('../hooks/usePolicies', () => ({
  usePolicy: () => ({ data: makePolicy(), isLoading: false, isError: false }),
  useUpdatePolicy: () => ({ mutate: vi.fn(), isPending: false }),
}))

vi.mock('../components/PolicyVersionHistory', () => ({
  default: () => null,
}))

function renderPage() {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } })
  return render(
    <QueryClientProvider client={client}>
      <MemoryRouter initialEntries={[`/vaktcomply/policies/${POLICY_ID}`]}>
        <Routes>
          <Route path="/vaktcomply/policies/:id" element={<PolicyDetailPage />} />
          <Route path="/vaktcomply/policies/:id/acceptance" element={<div>Akzeptanz-Seite erreicht</div>} />
        </Routes>
      </MemoryRouter>
    </QueryClientProvider>,
  )
}

describe('PolicyDetailPage — Weg zur Mitarbeiter-Bestätigung (R1-KW-11)', () => {
  it('führt aus der Richtlinie zu ihren Akzeptanz-Kampagnen', () => {
    renderPage()
    fireEvent.click(screen.getByRole('button', { name: 'Von Mitarbeitern bestätigen lassen' }))
    expect(screen.getByText('Akzeptanz-Seite erreicht')).toBeInTheDocument()
  })
})
