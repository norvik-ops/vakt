/**
 * apiFetch must be usable for multipart uploads (R1-18-D4).
 *
 * Seven authenticated writes went around apiFetch with a raw fetch(), all of
 * them FormData uploads, because apiFetch unconditionally set
 * `Content-Type: application/json` — and a Content-Type we write ourselves
 * replaces the multipart boundary the browser generates, which no server can
 * parse. The way out is not seven hand-rolled header blocks (seven copies are
 * the next drift source) but making the single central path FormData-aware.
 *
 * Two invariants, and both must hold at once: the request carries
 * X-CSRF-Token, and it carries NO Content-Type. Satisfying one by breaking the
 * other is not a fix.
 */
import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { apiFetch, setCsrfToken } from './client'

const realFetch = globalThis.fetch
const realCookie = Object.getOwnPropertyDescriptor(document, 'cookie')

function okJson(): ReturnType<typeof vi.fn> {
  return vi.fn().mockResolvedValue({
    ok: true,
    status: 200,
    headers: new Headers({ 'content-type': 'application/json' }),
    json: () => Promise.resolve({ ok: true }),
  })
}

function headersOf(spy: ReturnType<typeof vi.fn>): Record<string, string> {
  const [, init] = spy.mock.calls[0] as [string, RequestInit]
  return (init.headers ?? {}) as Record<string, string>
}

function contentTypeKeys(headers: Record<string, string>): string[] {
  return Object.keys(headers).filter((k) => k.toLowerCase() === 'content-type')
}

beforeEach(() => {
  Object.defineProperty(document, 'cookie', { value: '', writable: true, configurable: true })
  setCsrfToken(null)
})

afterEach(() => {
  globalThis.fetch = realFetch
  if (realCookie) Object.defineProperty(document, 'cookie', realCookie)
})

describe('apiFetch — FormData bodies', () => {
  it('sends X-CSRF-Token and omits Content-Type so the browser sets the boundary', async () => {
    document.cookie = 'csrf_token=multipart-token'
    const spy = okJson()
    globalThis.fetch = spy as unknown as typeof fetch

    const fd = new FormData()
    fd.append('file', new File(['data'], 'evidence.pdf', { type: 'application/pdf' }))

    await apiFetch('/vaktcomply/controls/abc/evidence/upload', { method: 'POST', body: fd })

    const headers = headersOf(spy)
    expect(headers['X-CSRF-Token']).toBe('multipart-token')
    expect(contentTypeKeys(headers)).toEqual([])
  })

  it('passes the FormData body through untouched', async () => {
    const spy = okJson()
    globalThis.fetch = spy as unknown as typeof fetch

    const fd = new FormData()
    fd.append('file', new File(['data'], 'x.csv', { type: 'text/csv' }))

    await apiFetch('/vaktscan/assets/import', { method: 'POST', body: fd })

    const [, init] = spy.mock.calls[0] as [string, RequestInit]
    expect(init.body).toBe(fd)
    expect(init.credentials).toBe('include')
  })

  it('drops a caller-supplied Content-Type on FormData — it is always wrong', async () => {
    // A caller copying the JSON hook pattern would pass
    // { 'Content-Type': 'application/json' }; honouring it would send a
    // boundary-less multipart body that the server rejects. There is no case
    // in which a hand-written Content-Type on FormData is correct, so the
    // central path removes it rather than trusting the caller.
    document.cookie = 'csrf_token=t'
    const spy = okJson()
    globalThis.fetch = spy as unknown as typeof fetch

    const fd = new FormData()
    fd.append('file', new File(['data'], 'x.csv'))

    await apiFetch('/vaktscan/assets/import', {
      method: 'POST',
      body: fd,
      headers: { 'Content-Type': 'application/json' },
    })

    expect(contentTypeKeys(headersOf(spy))).toEqual([])
    expect(headersOf(spy)['X-CSRF-Token']).toBe('t')
  })

  it('still sets application/json for non-FormData bodies', async () => {
    document.cookie = 'csrf_token=t'
    const spy = okJson()
    globalThis.fetch = spy as unknown as typeof fetch

    await apiFetch('/vaktcomply/ai/report', { method: 'POST', body: JSON.stringify({ a: 1 }) })

    const headers = headersOf(spy)
    expect(headers['Content-Type']).toBe('application/json')
    expect(headers['X-CSRF-Token']).toBe('t')
  })
})
