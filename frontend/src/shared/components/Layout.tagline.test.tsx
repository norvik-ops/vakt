import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { render, screen } from '@testing-library/react'
import { MemoryRouter } from 'react-router-dom'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import Layout from './Layout'

// Untertitel unter dem Logo: Vakt ist seit der Modul-Archivierung nur noch das
// ISMS — „Security Platform“ beschrieb ein Produkt, das es so nicht mehr gibt.
describe('Layout: Untertitel unter dem Logo', () => {
  beforeEach(() => {
    // Alle Abfragen bleiben offen: der Test braucht nur den Rahmen, keine Daten.
    vi.spyOn(globalThis, 'fetch').mockImplementation(() => new Promise<Response>(() => {}))
  })
  afterEach(() => { vi.restoreAllMocks() })

  it('zeigt „Compliance & ISMS“ und nicht mehr „Security Platform“', () => {
    const client = new QueryClient({ defaultOptions: { queries: { retry: false } } })
    render(
      <QueryClientProvider client={client}>
        <MemoryRouter initialEntries={['/']}>
          <Layout />
        </MemoryRouter>
      </QueryClientProvider>,
    )
    expect(screen.getByText('Compliance & ISMS')).toBeInTheDocument()
    expect(screen.queryByText('Security Platform')).not.toBeInTheDocument()
  })
})
