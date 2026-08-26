import { describe, it, expect, afterEach, vi } from 'vitest'
import { fetchMe } from './client'

/**
 * R1-SA21-D1, expiry path — hydration must not read "the server did not
 * answer" as "you are not signed in".
 *
 * fetchMe used to be `if (!res.ok) return null`. Every non-2xx collapsed into
 * the same answer, so a 429 from the rate limiter or a 502 during an API
 * restart cleared the auth store and the route guard bounced a perfectly valid
 * session to /login. Only 401/403 carry that meaning.
 *
 * These tests stub `fetch` rather than fetchMe's caller, because the whole
 * defect lives in how the raw Response is classified — a test one level up
 * cannot see the difference between "asked again" and "gave up immediately".
 */

function jsonResponse(status: number, body: unknown, headers: Record<string, string> = {}): Response {
  return new Response(JSON.stringify(body), {
    status,
    headers: { 'Content-Type': 'application/json', ...headers },
  })
}

const me = { id: 'u1', email: 't@example.com', display_name: 'Testnutzer', roles: ['Admin'] }

afterEach(() => {
  vi.unstubAllGlobals()
  vi.restoreAllMocks()
})

describe('fetchMe — transient failures are not a logout', () => {
  it('retries a 429 and keeps the session that the rate limiter interrupted', async () => {
    const fetchMock = vi
      .fn<typeof fetch>()
      // Retry-After: 0 keeps the test instant; the cap only matters in production.
      .mockResolvedValueOnce(jsonResponse(429, { error: 'rate limited' }, { 'Retry-After': '0' }))
      .mockResolvedValueOnce(jsonResponse(200, me))
    vi.stubGlobal('fetch', fetchMock)

    await expect(fetchMe()).resolves.toMatchObject({ id: 'u1' })
    expect(fetchMock).toHaveBeenCalledTimes(2)
  })

  it('retries a 503 — an API that is restarting must not sign anyone out', async () => {
    const fetchMock = vi
      .fn<typeof fetch>()
      .mockResolvedValueOnce(jsonResponse(503, { error: 'restarting' }))
      .mockResolvedValueOnce(jsonResponse(200, me))
    vi.stubGlobal('fetch', fetchMock)

    await expect(fetchMe()).resolves.toMatchObject({ id: 'u1' })
    expect(fetchMock).toHaveBeenCalledTimes(2)
  })

  it('retries a dropped connection', async () => {
    const fetchMock = vi
      .fn<typeof fetch>()
      .mockRejectedValueOnce(new TypeError('Failed to fetch'))
      .mockResolvedValueOnce(jsonResponse(200, me))
    vi.stubGlobal('fetch', fetchMock)

    await expect(fetchMe()).resolves.toMatchObject({ id: 'u1' })
    expect(fetchMock).toHaveBeenCalledTimes(2)
  })

  it('does not retry a 401 — that answer is the one that means "not signed in"', async () => {
    // The safe default has to stay sharp in both directions: retrying a real
    // sign-out would delay the redirect to /login for no gain.
    const fetchMock = vi.fn<typeof fetch>().mockResolvedValue(jsonResponse(401, { error: 'unauthorized' }))
    vi.stubGlobal('fetch', fetchMock)

    await expect(fetchMe()).resolves.toBeNull()
    expect(fetchMock).toHaveBeenCalledTimes(1)
  })

  it('does not retry a 403', async () => {
    const fetchMock = vi.fn<typeof fetch>().mockResolvedValue(jsonResponse(403, { error: 'forbidden' }))
    vi.stubGlobal('fetch', fetchMock)

    await expect(fetchMe()).resolves.toBeNull()
    expect(fetchMock).toHaveBeenCalledTimes(1)
  })

  it('gives up after a bounded number of attempts when the API stays down', async () => {
    // The safe default is unchanged: an API that is genuinely gone lands the
    // user on /login rather than hanging the shell forever.
    const fetchMock = vi.fn<typeof fetch>().mockResolvedValue(jsonResponse(503, { error: 'down' }))
    vi.stubGlobal('fetch', fetchMock)

    await expect(fetchMe()).resolves.toBeNull()
    expect(fetchMock).toHaveBeenCalledTimes(3)
  })
})
