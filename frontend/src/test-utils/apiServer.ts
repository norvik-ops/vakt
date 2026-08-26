import { vi } from 'vitest'

/**
 * A fake HTTP layer for tests that must cross the apiFetch boundary.
 *
 * Why this exists (Codeaudit v5b)
 * ------------------------------
 * The vitest suite mocks the *hook* almost everywhere — `vi.mock('../hooks/useX')`
 * returning a fixture. That is fine for rendering, and useless for the defect
 * class that actually shipped: the hook itself misreads the server's body. A
 * test that stubs the hook cannot see it, and twice it froze the wrong
 * expectation in place instead.
 *
 * mockApiServer stubs `globalThis.fetch` instead, so the real hook, the real
 * `apiFetch` and the real response handling all run. Combine it with the
 * `ApiResponse<path, method>` types from `api/contract.ts` to type the fixture
 * against the published OpenAPI contract — then the test asserts against what
 * the server says it returns, not against a body the test author invented.
 *
 * Deliberately strict, so a test cannot pass for the wrong reason:
 *   - An unregistered route responds 404 with a body naming it. A hook that
 *     silently changes its URL fails rather than quietly getting a fixture.
 *   - `requests` records every call, so a test can assert the method, headers
 *     and body that actually went out, not just what came back.
 *   - `restore()` puts the original fetch back; call it in afterEach.
 *
 * What it does NOT do: it is not the backend. It answers exactly what the
 * fixture says. It proves the frontend reads the documented shape correctly;
 * it cannot prove the document matches the Go handler.
 */

export interface RecordedRequest {
  method: string
  /** Path as apiFetch built it, including the /api/v1 prefix and query string. */
  url: string
  headers: Record<string, string>
  body: unknown
}

export interface MockResponse {
  status?: number
  /** Serialised as JSON unless `raw` is set. */
  body?: unknown
  headers?: Record<string, string>
}

export type RouteHandler = MockResponse | ((req: RecordedRequest) => MockResponse)

export interface ApiServerHandle {
  /** Every request that reached the stub, in order. */
  requests: RecordedRequest[]
  /** Replace or add a route after construction (e.g. to make the 2nd page differ). */
  route: (key: string, handler: RouteHandler) => void
  restore: () => void
}

function keyOf(method: string, url: string): string {
  // Match on the path without the query string: tests that care about query
  // params assert them from `requests`, which keeps the routing table readable.
  const path = url.split('?')[0].replace(/^\/api\/v1/, '')
  return `${method.toUpperCase()} ${path}`
}

/**
 * Routes are keyed `"<METHOD> <path>"` with the path written WITHOUT the
 * /api/v1 prefix, e.g. `'GET /vaktprivacy/vvt'`.
 */
export function mockApiServer(routes: Record<string, RouteHandler>): ApiServerHandle {
  const table = new Map(Object.entries(routes))
  const requests: RecordedRequest[] = []
  const original = globalThis.fetch

  const impl = (input: RequestInfo | URL, init?: RequestInit): Promise<Response> => {
    const url = typeof input === 'string' ? input : input instanceof URL ? input.href : input.url
    const method = (init?.method ?? 'GET').toUpperCase()

    const headers: Record<string, string> = {}
    const rawHeaders = init?.headers
    if (rawHeaders) {
      for (const [k, v] of Object.entries(rawHeaders as Record<string, string>)) {
        headers[k] = v
      }
    }

    let body: unknown = init?.body
    if (typeof body === 'string') {
      try {
        body = JSON.parse(body)
      } catch {
        /* not JSON — hand it back verbatim */
      }
    }

    const req: RecordedRequest = { method, url, headers, body }
    requests.push(req)

    const handler = table.get(keyOf(method, url))
    if (!handler) {
      return Promise.resolve(
        new Response(
          JSON.stringify({
            error: `mockApiServer: no route registered for ${keyOf(method, url)}`,
            code: 'TEST_ROUTE_MISSING',
          }),
          { status: 404, headers: { 'Content-Type': 'application/json' } },
        ),
      )
    }

    const res = typeof handler === 'function' ? handler(req) : handler
    const status = res.status ?? 200
    if (status === 204) {
      return Promise.resolve(new Response(null, { status: 204 }))
    }
    return Promise.resolve(
      new Response(JSON.stringify(res.body ?? null), {
        status,
        headers: { 'Content-Type': 'application/json', ...res.headers },
      }),
    )
  }

  vi.stubGlobal('fetch', vi.fn(impl))

  return {
    requests,
    route: (key, handler) => { table.set(key, handler) },
    restore: () => { globalThis.fetch = original },
  }
}
