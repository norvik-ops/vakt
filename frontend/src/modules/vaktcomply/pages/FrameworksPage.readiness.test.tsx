import { describe, it, expect, vi } from 'vitest'
import { render, screen } from '@testing-library/react'
import { MemoryRouter } from 'react-router-dom'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import FrameworksPage from './FrameworksPage'
import type { Framework } from '../types'

// UX-15: Die Karte zeigte nur den Ring „5%" — dass 9 Maßnahmen in Bearbeitung
// halb zählen und 0 umgesetzt sind, stand nirgends. Die Karte nennt die Zählung
// jetzt in derselben Zeile wie Dashboard und Framework-Detail.
const NIS2: Framework = {
  id: 'fw-nis2',
  org_id: 'org-1',
  name: 'NIS2',
  is_builtin: true,
  version: '2022',
  readiness_score: (4.5 / 88) * 100,
  implemented_controls: 0,
  partial_controls: 9,
  applicable_controls: 88,
  created_at: new Date().toISOString(),
}

vi.mock('../hooks/useFrameworks', () => ({
  useFrameworks: () => ({ data: [NIS2], isLoading: false, isError: false }),
  useEnableFramework: () => ({ mutate: vi.fn(), isPending: false }),
  useDeleteFramework: () => ({ mutate: vi.fn(), isPending: false }),
  useSwitchDORAVariant: () => ({ mutate: vi.fn(), isPending: false }),
}))

describe('FrameworksPage — Zählung auf der Karte (UX-15)', () => {
  it('zeigt umgesetzt und in Bearbeitung neben dem Ring', () => {
    const client = new QueryClient({ defaultOptions: { queries: { retry: false } } })
    render(
      <QueryClientProvider client={client}>
        <MemoryRouter>
          <FrameworksPage />
        </MemoryRouter>
      </QueryClientProvider>,
    )
    expect(screen.getByText('5%')).toBeInTheDocument()
    expect(screen.getByText('0 / 88 umgesetzt · 9 in Bearbeitung · 5%')).toBeInTheDocument()
  })
})
