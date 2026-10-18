import { describe, it, expect, beforeEach, afterEach } from 'vitest'
import { render, screen, act } from '@testing-library/react'
import { MemoryRouter } from 'react-router-dom'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'

import { GlobalSearch } from './GlobalSearch'
import { useEnabledModules } from '../moduleVisibility'

const RECENT_KEY = 'vakt_search_recent'

function renderOpen() {
  const qc = new QueryClient({ defaultOptions: { queries: { retry: false } } })
  render(
    <QueryClientProvider client={qc}>
      <MemoryRouter>
        <GlobalSearch />
      </MemoryRouter>
    </QueryClientProvider>,
  )
  act(() => { window.dispatchEvent(new CustomEvent('vakt:open-search')) })
}

describe('GlobalSearch — zuletzt angesehen', () => {
  beforeEach(() => {
    localStorage.setItem(RECENT_KEY, JSON.stringify([
      { id: '1', entity_type: 'finding', title: 'Offener Port', subtitle: '', url: '/vaktscan/findings/1' },
      { id: '2', entity_type: 'risk', title: 'Ransomware', subtitle: '', url: '/vaktcomply/risks/2' },
    ]))
  })

  afterEach(() => {
    localStorage.removeItem(RECENT_KEY)
    useEnabledModules.setState({ modules: null })
  })

  it('blendet Verlaufseinträge aus abgeschalteten Modulen aus', () => {
    useEnabledModules.setState({ modules: new Set(['vaktcomply']) })
    renderOpen()
    expect(screen.getByText('Ransomware')).toBeInTheDocument()
    expect(screen.queryByText('Offener Port')).not.toBeInTheDocument()
  })

  it('zeigt Verlaufseinträge aktivierter Module', () => {
    useEnabledModules.setState({ modules: new Set(['vaktcomply', 'vaktscan']) })
    renderOpen()
    expect(screen.getByText('Offener Port')).toBeInTheDocument()
  })
})
