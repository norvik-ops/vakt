import { describe, it, expect, vi } from 'vitest'
import { render, screen } from '@testing-library/react'
import { MemoryRouter } from 'react-router-dom'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import FrameworksPage from './FrameworksPage'
import type { Framework } from '../types'

// 1 von 93 umgesetzt → 1,075… % (backend policy.ReadinessScore, ADR-0086).
const ISO: Framework = {
  id: 'fw-iso',
  org_id: 'org-1',
  name: 'ISO27001',
  is_builtin: true,
  version: '2022',
  readiness_score: (1 / 93) * 100,
  created_at: new Date().toISOString(),
}

vi.mock('../hooks/useFrameworks', () => ({
  useFrameworks: () => ({ data: [ISO], isLoading: false, isError: false }),
  useEnableFramework: () => ({ mutate: vi.fn(), isPending: false }),
  useDeleteFramework: () => ({ mutate: vi.fn(), isPending: false }),
  useSwitchDORAVariant: () => ({ mutate: vi.fn(), isPending: false }),
}))

// R1-KW-6: die Karte zeigte fest „0 %" (ScoreCircle score={0}), Detail und
// Dashboard 1 %. Sie zeigt jetzt readiness_score, gerundet wie das Detail.
describe('FrameworksPage (R1-KW-6)', () => {
  it('zeigt auf der Karte den Bereitschaftsgrad aus readiness_score, gerundet wie das Framework-Detail', () => {
    const client = new QueryClient({ defaultOptions: { queries: { retry: false } } })
    render(
      <QueryClientProvider client={client}>
        <MemoryRouter>
          <FrameworksPage />
        </MemoryRouter>
      </QueryClientProvider>,
    )
    // Die einzige Prozentzahl der Seite ist der Kreis auf der ISO-Karte.
    expect(screen.getByText('1%')).toBeInTheDocument()
    expect(screen.queryByText('0%')).not.toBeInTheDocument()
  })
})
