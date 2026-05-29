import { describe, it, expect, vi } from 'vitest'
import { render, screen, fireEvent, waitFor } from '@testing-library/react'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { MemoryRouter, Route, Routes } from 'react-router-dom'
import AIDocumentationPage from './AIDocumentationPage'
import type { AISystem } from '../types'

vi.mock('../../../shared/stores/auth', () => ({
  useAuthStore: (selector: (s: { token: string | null }) => unknown) =>
    selector({ token: 'test-token' }),
}))

const mockSystem: AISystem = {
  id: 'sys-1',
  org_id: 'org-1',
  name: 'Recruiting AI',
  autonomy_level: 'partial',
  status: 'approved',
  risk_class: 'high',
  created_at: '2024-01-01T00:00:00Z',
  updated_at: '2024-01-01T00:00:00Z',
}

const mockMutate = vi.fn()

vi.mock('../hooks/useAISystems', () => ({
  useAISystem: () => ({ data: mockSystem }),
  useAIDocumentation: () => ({ data: null, isLoading: false }),
  useAIDocumentationVersions: () => ({ data: [] }),
  useSaveAIDocumentation: () => ({ mutate: mockMutate, isPending: false }),
  useAISystems: () => ({ data: [], isLoading: false, isError: false }),
  useCreateAISystem: () => ({ mutate: vi.fn(), isPending: false }),
  useUpdateAISystem: () => ({ mutate: vi.fn(), isPending: false }),
  useDeleteAISystem: () => ({ mutate: vi.fn(), isPending: false }),
}))

function wrapper({ children }: { children: React.ReactNode }) {
  const qc = new QueryClient({ defaultOptions: { queries: { retry: false } } })
  return (
    <QueryClientProvider client={qc}>
      <MemoryRouter initialEntries={['/comply/ai-systems/sys-1/documentation']}>
        <Routes>
          <Route path="/comply/ai-systems/:id/documentation" element={children} />
        </Routes>
      </MemoryRouter>
    </QueryClientProvider>
  )
}

describe('AIDocumentationPage', () => {
  it('renders system name in header', () => {
    render(<AIDocumentationPage />, { wrapper })
    expect(screen.getByText(/Recruiting AI/)).toBeTruthy()
  })

  it('renders all Art. 11 Annex IV section fields', () => {
    render(<AIDocumentationPage />, { wrapper })
    expect(screen.getByTestId('doc-field-system_description')).toBeTruthy()
    expect(screen.getByTestId('doc-field-intended_purpose')).toBeTruthy()
    expect(screen.getByTestId('doc-field-training_data')).toBeTruthy()
    expect(screen.getByTestId('doc-field-risk_management')).toBeTruthy()
    expect(screen.getByTestId('doc-field-human_oversight')).toBeTruthy()
    expect(screen.getByTestId('doc-field-logging_audit_trail')).toBeTruthy()
  })

  it('save draft button calls mutate with status=draft', async () => {
    render(<AIDocumentationPage />, { wrapper })
    const saveBtn = screen.getByTestId('save-draft-btn')
    fireEvent.click(saveBtn)
    await waitFor(() => {
      expect(mockMutate).toHaveBeenCalledWith(
        expect.objectContaining({ status: 'draft' }),
        expect.any(Object),
      )
    })
  })

  it('renders export PDF button', () => {
    render(<AIDocumentationPage />, { wrapper })
    expect(screen.getByTestId('export-pdf-btn')).toBeTruthy()
  })
})
