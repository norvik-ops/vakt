import { describe, it, expect, afterEach, vi } from 'vitest'
import { renderHook, waitFor } from '@testing-library/react'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import type { ReactNode } from 'react'
import { useCreateAuditorLink } from './useAuditorLinks'
import { mockApiServer, type ApiServerHandle } from '../../../test-utils/apiServer'
import type { ApiResponse } from '../../../api/contract'

/**
 * L1-01 — creating an auditor link 422'd on every attempt.
 *
 * The hook posted `expires_in_days`; the handler binds `expires_in_hours` with
 * `validate:"required,min=1,max=8760"` and has never known the other name.
 * Echo's Bind silently drops an unknown key, so the required field arrived as
 * zero and the request was rejected — no dialog input could have made it work.
 *
 * The assertions read the body that left the hook. That is the only place the
 * defect was visible: the hook's TypeScript types were internally consistent
 * and the UI had no way to show that a field it filled in never arrived.
 */

let server: ApiServerHandle

function wrapper({ children }: { children: ReactNode }) {
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
  })
  return <QueryClientProvider client={client}>{children}</QueryClientProvider>
}

const CREATED: ApiResponse<'/vaktcomply/frameworks/{id}/auditor-link', 'post'> = {
  auditor_url: '/api/v1/vaktcomply/auditor/tok-123',
}

function armServer() {
  server = mockApiServer({
    'POST /vaktcomply/frameworks/fw-1/auditor-link': { status: 201, body: CREATED },
  })
}

afterEach(() => { server.restore(); vi.unstubAllGlobals() })

describe('useCreateAuditorLink — request contract (L1-01)', () => {
  it('sends expires_in_hours, the field the handler binds', async () => {
    armServer()
    const { result } = renderHook(() => useCreateAuditorLink('fw-1'), { wrapper })

    result.current.mutate({ label: 'Wirtschaftspruefer', expires_in_days: 30 })
    await waitFor(() => { expect(result.current.isSuccess).toBe(true) })

    const body = server.requests[0].body as Record<string, unknown>
    expect(body.expires_in_hours).toBe(720)
    // The old name must not travel any more: it is not part of the contract,
    // and leaving it in would make a future reader think either name works.
    expect(body).not.toHaveProperty('expires_in_days')
  })

  it('never sends 0 hours — that is exactly what the required tag rejected', async () => {
    armServer()
    const { result } = renderHook(() => useCreateAuditorLink('fw-1'), { wrapper })

    result.current.mutate({ label: 'x', expires_in_days: 0 })
    await waitFor(() => { expect(result.current.isSuccess).toBe(true) })

    expect((server.requests[0].body as Record<string, unknown>).expires_in_hours).toBe(24)
  })

  it('clamps to the backend maximum of 8760 hours instead of letting the server 422', async () => {
    armServer()
    const { result } = renderHook(() => useCreateAuditorLink('fw-1'), { wrapper })

    result.current.mutate({ label: 'x', expires_in_days: 4000 })
    await waitFor(() => { expect(result.current.isSuccess).toBe(true) })

    expect((server.requests[0].body as Record<string, unknown>).expires_in_hours).toBe(8760)
  })
})
