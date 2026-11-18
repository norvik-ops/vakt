import { describe, it, expect, vi } from 'vitest'
import { render, screen } from '@testing-library/react'
import { MemoryRouter, Route, Routes } from 'react-router-dom'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import FrameworkDetailPage from './FrameworkDetailPage'
import type { Framework } from '../types'

// Die Überschrift lautete „EUAIACT Details" — interner Schlüssel plus Füllwort.
// Sie zeigt jetzt den Anzeigenamen aus dem Framework-Katalog.

const AI_ACT: Framework = {
  id: 'fw-ai',
  org_id: 'org-1',
  name: 'EUAIACT',
  is_builtin: true,
  version: '2024',
  created_at: new Date().toISOString(),
}

vi.mock('../hooks/useFrameworks', () => ({
  useFramework: () => ({ data: AI_ACT, isLoading: false }),
  useReadinessReport: () => ({ data: undefined, isLoading: false }),
  useGapAnalysis: () => ({ data: { framework_id: 'fw-ai', gaps: [] }, isLoading: false }),
  useFrameworkControls: () => ({ data: [], isLoading: false }),
  useDownloadFrameworkPDF: () => () => {},
}))

vi.mock('../hooks/useAuditorLinks', () => ({
  useAuditorLinks: () => ({ data: [], isLoading: false }),
  useRevokeAuditorLink: () => ({ mutate: vi.fn(), isPending: false }),
  useCreateAuditorLink: () => ({ mutate: vi.fn(), isPending: false }),
}))

vi.mock('../hooks/useControls', () => ({
  useUpdateControl: () => ({ mutate: vi.fn(), isPending: false }),
  useBulkUpdateControls: () => ({
    mutate: vi.fn(),
    mutateAsync: vi.fn().mockResolvedValue({ updated: 0 }),
    isPending: false,
  }),
  useExportControl: () => ({ mutate: vi.fn(), isPending: false }),
}))

describe('FrameworkDetailPage — Anzeigename', () => {
  it('zeigt „EU AI Act" als Überschrift, ohne Schlüssel und ohne „Details"', () => {
    const client = new QueryClient({ defaultOptions: { queries: { retry: false } } })
    render(
      <QueryClientProvider client={client}>
        <MemoryRouter initialEntries={['/vaktcomply/frameworks/fw-ai']}>
          <Routes>
            <Route path="/vaktcomply/frameworks/:id" element={<FrameworkDetailPage />} />
          </Routes>
        </MemoryRouter>
      </QueryClientProvider>,
    )
    expect(screen.getByRole('heading', { name: 'EU AI Act', level: 1 })).toBeInTheDocument()
    expect(screen.queryByText(/EUAIACT/)).not.toBeInTheDocument()
    expect(screen.queryByText(/Details$/)).not.toBeInTheDocument()
  })
})
