import { describe, it, expect, vi, beforeEach } from 'vitest'
import { renderHook, waitFor } from '@testing-library/react'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { createElement } from 'react'
import { useFrameworkControls } from './useFrameworks'
import type { Control } from '../types'

vi.mock('../../../api/client', () => ({
  apiFetch: vi.fn(),
}))

import { apiFetch } from '../../../api/client'

function ctrl(n: number): Control {
  return { id: `id-${String(n)}`, control_id: `A.${String(n)}`, title: `C ${String(n)}`, status: 'missing' } as Control
}

function createWrapper() {
  const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } })
  return ({ children }: { children: React.ReactNode }) =>
    createElement(QueryClientProvider, { client: queryClient }, children)
}

// R1-KW-5: ISO 27001 has 93 controls; the hook used to load only the first
// page (limit=25) and never paged on. It must follow next_cursor to the end.
describe('useFrameworkControls (R1-KW-5)', () => {
  beforeEach(() => { vi.mocked(apiFetch).mockReset() })

  it('lädt alle Seiten über den Cursor, bis has_more false ist', async () => {
    // 230 > 2 × Seitengröße 100 — erzwingt drei Seiten.
    const all = Array.from({ length: 230 }, (_, i) => ctrl(i + 1))
    vi.mocked(apiFetch).mockImplementation((path: string) => {
      const url = new URL(path, 'http://x')
      const cursor = url.searchParams.get('cursor')
      const start = cursor ? Number(cursor) : 0
      const limit = Number(url.searchParams.get('limit'))
      const data = all.slice(start, start + limit)
      const hasMore = start + limit < all.length
      return Promise.resolve({
        data,
        pagination: { limit, has_more: hasMore, next_cursor: hasMore ? String(start + limit) : undefined },
      })
    })

    const { result } = renderHook(() => useFrameworkControls('fw-1'), { wrapper: createWrapper() })
    await waitFor(() => { expect(result.current.isSuccess).toBe(true) })

    expect(result.current.data).toHaveLength(230)
    expect(result.current.data?.[229].control_id).toBe('A.230')
    const calls = vi.mocked(apiFetch).mock.calls.map((c) => c[0])
    expect(calls).toHaveLength(3)
    for (const p of calls) {
      expect(p).toMatch(/^\/vaktcomply\/frameworks\/fw-1\/controls\?/)
      // Cursor-Zweig: kein page-Parameter (der Offset-Zweig ist deprecated).
      expect(p).not.toMatch(/[?&]page=/)
    }
  })

  it('bricht nach höchstens 50 Seiten mit einem Fehler ab (Subreview PR #182)', async () => {
    let n = 0
    vi.mocked(apiFetch).mockImplementation(() => {
      n++
      return Promise.resolve({
        data: [ctrl(n)],
        pagination: { limit: 100, has_more: true, next_cursor: `c${String(n)}` },
      })
    })
    const { result } = renderHook(() => useFrameworkControls('fw-1'), { wrapper: createWrapper() })
    await waitFor(() => { expect(result.current.isError).toBe(true) })
    expect(vi.mocked(apiFetch)).toHaveBeenCalledTimes(50)
    expect(result.current.error?.message).toMatch(/50/)
  })

  it('bricht ab statt endlos zu laden, wenn der Server denselben Cursor wiederholt', async () => {
    vi.mocked(apiFetch).mockResolvedValue({
      data: [ctrl(1)],
      pagination: { limit: 100, has_more: true, next_cursor: 'same' },
    })
    const { result } = renderHook(() => useFrameworkControls('fw-1'), { wrapper: createWrapper() })
    await waitFor(() => { expect(result.current.isError).toBe(true) })
  })
})
