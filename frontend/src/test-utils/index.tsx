import { type ReactNode } from 'react'
import { render, type RenderResult } from '@testing-library/react'
import { MemoryRouter } from 'react-router-dom'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'

export function makeQueryClient(): QueryClient {
  return new QueryClient({
    defaultOptions: {
      queries: { retry: false, gcTime: 0 },
      mutations: { retry: false },
    },
  })
}

interface ProviderOptions {
  initialPath?: string
}

/**
 * Wraps UI in QueryClientProvider + MemoryRouter — the two providers every
 * page component needs. Use makeQueryClient() directly when you need a named
 * reference to invalidate caches or inspect query state in a test.
 */
export function renderWithProviders(
  ui: ReactNode,
  { initialPath = '/' }: ProviderOptions = {},
): RenderResult {
  const client = makeQueryClient()
  return render(
    <QueryClientProvider client={client}>
      <MemoryRouter initialEntries={[initialPath]}>
        {ui}
      </MemoryRouter>
    </QueryClientProvider>,
  )
}
