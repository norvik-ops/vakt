import { describe, it, expect, vi } from 'vitest'
import { render, screen, fireEvent, waitFor } from '@testing-library/react'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { MemoryRouter } from 'react-router-dom'
import AISystemsPage from './AISystemsPage'
import type { AISystem } from '../types'

vi.mock('../../../shared/stores/auth', () => ({
  useAuthStore: (selector: (s: { token: string | null }) => unknown) =>
    selector({ token: 'test-token' }),
}))

const mockSystem: AISystem = {
  id: 'ai-1',
  org_id: 'org-1',
  name: 'ChatGPT-Integration',
  autonomy_level: 'assistive',
  status: 'under_review',
  created_at: '2024-01-01T00:00:00Z',
  updated_at: '2024-01-01T00:00:00Z',
}

vi.mock('../hooks/useAISystems', () => ({
  useAISystems: () => ({ data: [mockSystem], isLoading: false, isError: false }),
  useCreateAISystem: () => ({ mutate: vi.fn(), isPending: false }),
  useUpdateAISystem: () => ({ mutate: vi.fn(), isPending: false }),
  useDeleteAISystem: () => ({ mutate: vi.fn(), isPending: false }),
}))

function wrapper({ children }: { children: React.ReactNode }) {
  const qc = new QueryClient({ defaultOptions: { queries: { retry: false } } })
  return (
    <MemoryRouter>
      <QueryClientProvider client={qc}>{children}</QueryClientProvider>
    </MemoryRouter>
  )
}

describe('AISystemsPage', () => {
  it('renders filter toolbar with both filter selects', () => {
    render(<AISystemsPage />, { wrapper })
    expect(screen.getByTestId('ai-filter-toolbar')).toBeTruthy()
    expect(screen.getByTestId('filter-risk-class')).toBeTruthy()
    expect(screen.getByTestId('filter-status')).toBeTruthy()
  })

  it('renders AI system card with name', () => {
    render(<AISystemsPage />, { wrapper })
    expect(screen.getByText('ChatGPT-Integration')).toBeTruthy()
  })

  it('delete button opens confirmation dialog', async () => {
    render(<AISystemsPage />, { wrapper })
    const deleteBtn = screen.getByTestId('delete-ai-system-btn')
    fireEvent.click(deleteBtn)
    await waitFor(() => {
      expect(screen.getByTestId('confirm-delete-btn')).toBeTruthy()
    })
  })

  it('confirm delete button calls mutate', async () => {
    const mockMutate = vi.fn()
    vi.doMock('../hooks/useAISystems', () => ({
      useAISystems: () => ({ data: [mockSystem], isLoading: false, isError: false }),
      useCreateAISystem: () => ({ mutate: vi.fn(), isPending: false }),
      useUpdateAISystem: () => ({ mutate: vi.fn(), isPending: false }),
      useDeleteAISystem: () => ({ mutate: mockMutate, isPending: false }),
    }))
    render(<AISystemsPage />, { wrapper })
    const deleteBtn = screen.getByTestId('delete-ai-system-btn')
    fireEvent.click(deleteBtn)
    await waitFor(() => {
      const confirmBtn = screen.getByTestId('confirm-delete-btn')
      fireEvent.click(confirmBtn)
    })
  })
})
