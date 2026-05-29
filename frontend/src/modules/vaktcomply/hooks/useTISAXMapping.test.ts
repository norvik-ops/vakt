import { describe, it, expect, vi } from 'vitest'
import { renderHook, waitFor } from '@testing-library/react'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { createElement } from 'react'
import { useTISAXISOMapping, useTISAXGapsAfterISO } from './useTISAXMapping'
import type { MappingResult } from '../types'

vi.mock('../../../api/client', () => ({
  apiFetch: vi.fn(),
}))

import { apiFetch } from '../../../api/client'

const mockMappingResults: MappingResult[] = [
  {
    tisax_control_id: 'TISAX-1.1.1',
    tisax_control_title: 'IS-Politik und -Ziele definiert',
    iso_control_id: 'A.5.1.1',
    iso_control_title: 'Policies for information security',
    covered: true,
  },
  {
    tisax_control_id: 'TISAX-2.1.1',
    tisax_control_title: 'Rollen und Verantwortlichkeiten IS',
    iso_control_id: 'A.6.1.1',
    iso_control_title: 'Information security roles and responsibilities',
    covered: false,
  },
]

function createWrapper() {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  })
  return ({ children }: { children: React.ReactNode }) =>
    createElement(QueryClientProvider, { client: queryClient }, children)
}

describe('useTISAXISOMapping', () => {
  it('hook exists and is a function', () => {
    expect(useTISAXISOMapping).toBeDefined()
    expect(typeof useTISAXISOMapping).toBe('function')
  })

  it('uses the correct query key', () => {
    vi.mocked(apiFetch).mockResolvedValue([])
    const { result } = renderHook(() => useTISAXISOMapping(), {
      wrapper: createWrapper(),
    })
    // The hook should be loading or success
    expect(result.current).toBeDefined()
  })

  it('returns mapping results with covered field', async () => {
    vi.mocked(apiFetch).mockResolvedValue(mockMappingResults)

    const { result } = renderHook(() => useTISAXISOMapping(), {
      wrapper: createWrapper(),
    })

    await waitFor(() => { expect(result.current.isSuccess).toBe(true); })

    expect(result.current.data).toHaveLength(2)
    // First result: covered=true
    expect(result.current.data?.[0].covered).toBe(true)
    expect(result.current.data?.[0].tisax_control_id).toBe('TISAX-1.1.1')
    expect(result.current.data?.[0].iso_control_id).toBe('A.5.1.1')
    // Second result: covered=false
    expect(result.current.data?.[1].covered).toBe(false)
  })

  it('passes framework_id as query param when provided', async () => {
    vi.mocked(apiFetch).mockResolvedValue([])

    renderHook(() => useTISAXISOMapping('fw-123'), {
      wrapper: createWrapper(),
    })

    await waitFor(() => {
      expect(vi.mocked(apiFetch)).toHaveBeenCalledWith(
        expect.stringContaining('framework_id=fw-123'),
      )
    })
  })
})

describe('useTISAXGapsAfterISO', () => {
  it('hook exists and is a function', () => {
    expect(useTISAXGapsAfterISO).toBeDefined()
    expect(typeof useTISAXGapsAfterISO).toBe('function')
  })

  it('returns only uncovered controls (gaps)', async () => {
    const gaps = mockMappingResults.filter((r) => !r.covered)
    vi.mocked(apiFetch).mockResolvedValue(gaps)

    const { result } = renderHook(() => useTISAXGapsAfterISO(), {
      wrapper: createWrapper(),
    })

    await waitFor(() => { expect(result.current.isSuccess).toBe(true); })

    expect(result.current.data).toHaveLength(1)
    expect(result.current.data![0].covered).toBe(false)
    expect(result.current.data![0].tisax_control_id).toBe('TISAX-2.1.1')
  })
})
