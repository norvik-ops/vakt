import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest'
import { downloadBlob } from './downloadBlob'
import { setCsrfToken } from '../../api/client'

/**
 * R1-W2B-03 — downloadBlob takes a RequestInit and forwards it, so a caller can
 * hand it `{method: 'POST'}`. It used to attach no X-CSRF-Token, which would
 * have made any such call 403. All three call sites are GET today, so the
 * defect was latent rather than live, and the fix landed in 508d69af with no
 * test behind it — leaving the behaviour un-pinned and free to regress
 * silently, since no user-visible path exercises it yet.
 *
 * The static gate cannot cover this either, and says so in its own "does NOT
 * check" section: check_fe_csrf.py sees the wrapper's single `fetch`, not its
 * call sites, so a caller that starts writing through here is invisible to it.
 * That leaves this file as the only thing holding the behaviour.
 */

// .bind(URL) rather than a bare read: @typescript-eslint/unbound-method
// flags detaching a method from its object, and binding is its stated remedy.
const origCreateObjectURL = URL.createObjectURL.bind(URL)
const origRevokeObjectURL = URL.revokeObjectURL.bind(URL)
let fetchMock: ReturnType<typeof vi.fn>

function lastHeaders(): Record<string, string> {
  return (fetchMock.mock.calls[0][1] as RequestInit).headers as Record<string, string>
}

beforeEach(() => {
  document.cookie = 'csrf_token=cookie-token'
  fetchMock = vi.fn(() => Promise.resolve(
    new Response(new Blob(['x']), { status: 200, headers: { 'Content-Type': 'application/pdf' } }),
  ))
  vi.stubGlobal('fetch', fetchMock)
  URL.createObjectURL = () => 'blob:stub'
  URL.revokeObjectURL = () => { /* nothing to release */ }
  vi.spyOn(HTMLAnchorElement.prototype, 'click').mockImplementation(() => { /* no navigation in jsdom */ })
})

afterEach(() => {
  URL.createObjectURL = origCreateObjectURL
  URL.revokeObjectURL = origRevokeObjectURL
  vi.unstubAllGlobals()
  vi.restoreAllMocks()
})

describe('downloadBlob — CSRF on writes (R1-W2B-03)', () => {
  it('attaches X-CSRF-Token when the caller asks for a POST', async () => {
    await downloadBlob('/api/v1/vaktcomply/report', 'report.pdf', { method: 'POST' })
    expect(lastHeaders()['X-CSRF-Token']).toBe('cookie-token')
  })

  it('attaches it on DELETE too, not just POST', async () => {
    await downloadBlob('/api/v1/vaktcomply/report', 'report.pdf', { method: 'DELETE' })
    expect(lastHeaders()['X-CSRF-Token']).toBe('cookie-token')
  })

  it('leaves it off safe methods, which the middleware ignores anyway', async () => {
    await downloadBlob('/api/v1/vaktcomply/report', 'report.pdf')
    expect(lastHeaders()['X-CSRF-Token']).toBeUndefined()
  })

  it("does not let a caller's own headers drop the token", async () => {
    // The spread order is the point: `headers` used to be able to replace the
    // whole object, which is how the original CSRF-header-missing bug worked.
    await downloadBlob('/api/v1/x', 'x.pdf', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
    })
    expect(lastHeaders()['X-CSRF-Token']).toBe('cookie-token')
    expect(lastHeaders()['Content-Type']).toBe('application/json')
  })

  it('falls back to the in-memory token when the cookie is not JS-readable', async () => {
    // A reverse proxy that rewrites Set-Cookie to add HttpOnly makes the cookie
    // unreadable while the browser still sends it. That is exactly the case the
    // private copies this helper used to carry could not handle.
    document.cookie = 'csrf_token=; expires=Thu, 01 Jan 1970 00:00:00 GMT'
    setCsrfToken('from-login-response')
    await downloadBlob('/api/v1/x', 'x.pdf', { method: 'POST' })
    expect(lastHeaders()['X-CSRF-Token']).toBe('from-login-response')
    setCsrfToken(null)
  })
})
