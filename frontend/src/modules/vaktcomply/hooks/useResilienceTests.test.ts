import { describe, it, expect, vi } from 'vitest'
import { renderHook, waitFor } from '@testing-library/react'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { createElement } from 'react'
import { useResilienceTests } from './useResilienceTests'
import type { ResilienceTestsResponse } from '../types'

vi.mock('../../../api/client', () => ({
  apiFetch: vi.fn(),
}))

import { apiFetch } from '../../../api/client'

const mockResponse: ResilienceTestsResponse = {
  tests: [
    {
      id: 'rt-1',
      org_id: 'org-1',
      type: 'tlpt',
      test_date: '2024-01-01T00:00:00Z',
      remediation_status: 'open',
      created_at: '2024-01-01T00:00:00Z',
      updated_at: '2024-01-01T00:00:00Z',
    },
  ],
  tlpt_overdue_warning: false,
}

function createWrapper() {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  })
  return ({ children }: { children: React.ReactNode }) =>
    createElement(QueryClientProvider, { client: queryClient }, children)
}

describe('useResilienceTests', () => {
  it('hook exists and can be called', () => {
    expect(useResilienceTests).toBeDefined()
    expect(typeof useResilienceTests).toBe('function')
  })

  it('returns expected shape from API response', async () => {
    vi.mocked(apiFetch).mockResolvedValue(mockResponse)

    const { result } = renderHook(() => useResilienceTests(), {
      wrapper: createWrapper(),
    })

    await waitFor(() => { expect(result.current.isSuccess).toBe(true); })

    expect(result.current.data).toBeDefined()
    expect(result.current.data?.tests).toHaveLength(1)
    expect(result.current.data?.tlpt_overdue_warning).toBe(false)
    expect(result.current.data?.tests[0].type).toBe('tlpt')
  })

  it('returns tlpt_overdue_warning: true when API signals overdue', async () => {
    const overdueResponse: ResilienceTestsResponse = { tests: [], tlpt_overdue_warning: true }
    vi.mocked(apiFetch).mockResolvedValue(overdueResponse)

    const { result } = renderHook(() => useResilienceTests(), {
      wrapper: createWrapper(),
    })

    await waitFor(() => { expect(result.current.isSuccess).toBe(true); })

    expect(result.current.data?.tlpt_overdue_warning).toBe(true)
  })
})
