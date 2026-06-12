import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { apiFetch, RateLimitedError, FeatureLockedError, MFARequiredError } from './client'

const realFetch = globalThis.fetch
const realCookie = Object.getOwnPropertyDescriptor(document, 'cookie')

beforeEach(() => {
  // Reset cookies between tests so CSRF state doesn't bleed across.
  Object.defineProperty(document, 'cookie', { value: '', writable: true, configurable: true })
})

afterEach(() => {
  globalThis.fetch = realFetch
  if (realCookie) Object.defineProperty(document, 'cookie', realCookie)
})

// ── Double-prefix guard ───────────────────────────────────────────────────────

describe('apiFetch — double-prefix guard', () => {
  it('throws in DEV when path starts with /api/v1/', async () => {
    // The guard exists so callers cannot accidentally produce /api/v1/api/v1/...
    await expect(apiFetch('/api/v1/vakthr/contractors')).rejects.toThrow(
      'apiFetch: path must not include the API base prefix',
    )
  })

  it('uses the correct full URL for a valid relative path', async () => {
    const spy = vi.fn().mockResolvedValue({
      ok: true, status: 200,
      headers: new Headers({ 'content-type': 'application/json' }),
      json: () => Promise.resolve([]),
    })
    globalThis.fetch = spy

    await apiFetch('/vakthr/contractors')

    expect(spy).toHaveBeenCalledOnce()
    const [url] = spy.mock.calls[0] as [string]
    expect(url).toBe('/api/v1/vakthr/contractors')
    expect(url).not.toContain('/api/v1/api/v1/')
  })
})

// ── CSRF token plumbing ──────────────────────────────────────────────────────

describe('apiFetch — CSRF', () => {
  it('attaches the csrf_token cookie value to X-CSRF-Token header on POST', async () => {
    document.cookie = 'csrf_token=test-token-abc'
    const spy = vi.fn().mockResolvedValue({
      ok: true, status: 200, headers: new Headers({ 'content-type': 'application/json' }),
      json: () => Promise.resolve({ ok: true }),
    })
    globalThis.fetch = spy

    await apiFetch('/test', { method: 'POST', body: '{}' })

    expect(spy).toHaveBeenCalled()
    const [, opts] = spy.mock.calls[0] as [string, RequestInit]
    expect((opts.headers as Record<string, string>)['X-CSRF-Token']).toBe('test-token-abc')
  })

  it('does NOT attach the CSRF header on safe (GET) methods', async () => {
    document.cookie = 'csrf_token=should-not-be-sent'
    const spy = vi.fn().mockResolvedValue({
      ok: true, status: 200, headers: new Headers({ 'content-type': 'application/json' }),
      json: () => Promise.resolve([]),
    })
    globalThis.fetch = spy

    await apiFetch('/test')

    const [, opts] = spy.mock.calls[0] as [string, RequestInit]
    expect((opts.headers as Record<string, string>)['X-CSRF-Token']).toBeUndefined()
  })
})

// ── Retry / backoff ──────────────────────────────────────────────────────────

describe('apiFetch — retries', () => {
  it('retries idempotent (GET) requests on 503 and ultimately succeeds', async () => {
    let call = 0
    globalThis.fetch = vi.fn().mockImplementation(() => {
      call += 1
      if (call < 2) {
        return Promise.resolve({
          ok: false, status: 503, headers: new Headers(),
          json: () => Promise.resolve({}),
        } as unknown as Response)
      }
      return Promise.resolve({
        ok: true, status: 200, headers: new Headers({ 'content-type': 'application/json' }),
        json: () => Promise.resolve({ retrieved: true }),
      } as unknown as Response)
    })

    const result = await apiFetch<{ retrieved: boolean }>('/test')
    expect(result.retrieved).toBe(true)
    expect(call).toBe(2)
  })

  it('does NOT retry POSTs on 5xx — they may have side-effects', async () => {
    let call = 0
    globalThis.fetch = vi.fn().mockImplementation(() => {
      call += 1
      return Promise.resolve({
        ok: false, status: 503, headers: new Headers(),
        json: () => Promise.resolve({ error: 'unavailable' }),
      } as unknown as Response)
    })

    await expect(apiFetch('/test', { method: 'POST', body: '{}' })).rejects.toThrow()
    expect(call).toBe(1)
  })
})

// ── Error mapping ────────────────────────────────────────────────────────────

describe('apiFetch — error mapping', () => {
  it('throws RateLimitedError on 429 (non-idempotent) with parsed Retry-After', async () => {
    globalThis.fetch = vi.fn().mockResolvedValue({
      ok: false, status: 429,
      headers: new Headers({ 'Retry-After': '12' }),
      json: () => Promise.resolve({}),
    })

    try {
      await apiFetch('/test', { method: 'POST', body: '{}' })
      throw new Error('expected throw')
    } catch (err) {
      expect(err).toBeInstanceOf(RateLimitedError)
      expect((err as RateLimitedError).retryAfterSeconds).toBe(12)
    }
  })

  it('throws FeatureLockedError on 402 with feature name', async () => {
    globalThis.fetch = vi.fn().mockResolvedValue({
      ok: false, status: 402, headers: new Headers(),
      json: () => Promise.resolve({ feature: 'ai_advisor' }),
    })

    try {
      await apiFetch('/test')
      throw new Error('expected throw')
    } catch (err) {
      expect(err).toBeInstanceOf(FeatureLockedError)
      expect((err as FeatureLockedError).feature).toBe('ai_advisor')
    }
  })

  it('redirects to /account and throws MFARequiredError when 403 + MFA_REQUIRED', async () => {
    globalThis.fetch = vi.fn().mockResolvedValue({
      ok: false, status: 403, headers: new Headers(),
      json: () => Promise.resolve({ code: 'MFA_REQUIRED' }),
    })
    // jsdom: window.location.href is settable but reload would fire — stub it.
    const originalLocation = window.location
    Object.defineProperty(window, 'location', {
      configurable: true,
      value: { ...(originalLocation as unknown as Record<string, unknown>), href: '' },
    })

    try {
      await apiFetch('/test')
      throw new Error('expected throw')
    } catch (err) {
      expect(err).toBeInstanceOf(MFARequiredError)
      expect(window.location.href).toBe('/account')
    } finally {
      Object.defineProperty(window, 'location', { configurable: true, value: originalLocation })
    }
  })
})
