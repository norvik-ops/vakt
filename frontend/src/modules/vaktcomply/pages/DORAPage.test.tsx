import { describe, it, expect, vi } from 'vitest'
import { render, screen } from '@testing-library/react'
import { MemoryRouter, Route, Routes } from 'react-router-dom'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import DORAPage, { groupDoraControlsByArticle } from './DORAPage'
import type { Control } from '../types'

// ── helpers ──────────────────────────────────────────────────────────────────

function makeControl(overrides: Partial<Control> = {}): Control {
  return {
    id: 'c-1',
    framework_id: 'fw-1',
    control_id: 'DORA-1.1',
    title: 'Test Control',
    description: 'desc',
    domain: 'ICT-Risikomanagement',
    status: 'missing',
    not_applicable: false,
    ...overrides,
  }
}

const MOCK_CONTROLS: Control[] = [
  makeControl({ id: 'c-1', control_id: 'DORA-1.1', domain: 'ICT-Risikomanagement', title: 'Control 1.1' }),
  makeControl({ id: 'c-2', control_id: 'DORA-2.1', domain: 'Vorfallmanagement', title: 'Control 2.1' }),
  makeControl({ id: 'c-3', control_id: 'DORA-3.1', domain: 'Resilienztests', title: 'Control 3.1' }),
  makeControl({ id: 'c-4', control_id: 'DORA-4.1', domain: 'Drittparteienrisiken', title: 'Control 4.1' }),
  makeControl({ id: 'c-5', control_id: 'DORA-5.1', domain: 'Informationsaustausch', title: 'Control 5.1' }),
]

vi.mock('../hooks/useFrameworks', () => ({
  useFrameworkControls: () => ({ data: MOCK_CONTROLS, isLoading: false }),
}))

function renderDORAPage(frameworkId = 'fw-1') {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } })
  return render(
    <QueryClientProvider client={client}>
      <MemoryRouter initialEntries={[`/vaktcomply/dora/${frameworkId}`]}>
        <Routes>
          <Route path="/vaktcomply/dora/:frameworkId" element={<DORAPage />} />
        </Routes>
      </MemoryRouter>
    </QueryClientProvider>,
  )
}

// ── Tests: DORAPage sections ──────────────────────────────────────────────────

describe('DORAPage', () => {
  it('renders five article group sections', () => {
    renderDORAPage()
    expect(screen.getByText('Art. 5–16')).toBeInTheDocument()
    expect(screen.getByText('Art. 17–23')).toBeInTheDocument()
    expect(screen.getByText('Art. 24–27')).toBeInTheDocument()
    expect(screen.getByText('Art. 28–44')).toBeInTheDocument()
    expect(screen.getByText('Art. 45–49')).toBeInTheDocument()
  })
})

// ── Tests: groupDoraControlsByArticle ────────────────────────────────────────

describe('groupDoraControlsByArticle', () => {
  it('maps ICT-Risikomanagement to Art. 5–16', () => {
    const controls = [makeControl({ domain: 'ICT-Risikomanagement' })]
    const result = groupDoraControlsByArticle(controls)
    expect(result['Art. 5–16']).toHaveLength(1)
  })

  it('maps Vorfallmanagement to Art. 17–23', () => {
    const controls = [makeControl({ domain: 'Vorfallmanagement' })]
    const result = groupDoraControlsByArticle(controls)
    expect(result['Art. 17–23']).toHaveLength(1)
  })

  it('maps Resilienztests to Art. 24–27', () => {
    const controls = [makeControl({ domain: 'Resilienztests' })]
    const result = groupDoraControlsByArticle(controls)
    expect(result['Art. 24–27']).toHaveLength(1)
  })

  it('maps Drittparteienrisiken to Art. 28–44', () => {
    const controls = [makeControl({ domain: 'Drittparteienrisiken' })]
    const result = groupDoraControlsByArticle(controls)
    expect(result['Art. 28–44']).toHaveLength(1)
  })

  it('maps Informationsaustausch to Art. 45–49', () => {
    const controls = [makeControl({ domain: 'Informationsaustausch' })]
    const result = groupDoraControlsByArticle(controls)
    expect(result['Art. 45–49']).toHaveLength(1)
  })

  it('groups multiple controls correctly across all five domains', () => {
    const result = groupDoraControlsByArticle(MOCK_CONTROLS)
    expect(result['Art. 5–16']).toHaveLength(1)
    expect(result['Art. 17–23']).toHaveLength(1)
    expect(result['Art. 24–27']).toHaveLength(1)
    expect(result['Art. 28–44']).toHaveLength(1)
    expect(result['Art. 45–49']).toHaveLength(1)
  })
})
