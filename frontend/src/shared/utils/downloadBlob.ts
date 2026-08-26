import { toast } from '../hooks/useToast'
import { readCsrfToken } from '../../api/client'

const SAFE_METHODS = new Set(['GET', 'HEAD', 'OPTIONS'])

/**
 * downloadBlob fetches a URL and triggers a file download — but ONLY after
 * checking res.ok.
 *
 * S131-D1 (R-M06/D18-04): the raw `fetch(...).then(r => r.blob())` pattern
 * scattered across the export buttons never checked res.ok, so a 500 response
 * body (e.g. `{"error":"export failed"}`, 26 bytes) was saved verbatim as
 * `audit-paket-<date>.zip` — a JSON error masquerading as a ZIP, with NO error
 * shown. A user believed the export succeeded and could hand a corrupt file to
 * an auditor. On a non-ok response this parses the error, toasts it, and downloads
 * nothing.
 *
 * Returns true on success, false when the download was aborted due to an error.
 *
 * R1-18-D4 follow-up: `init` lets a caller pick the method, so this helper can
 * issue a write — and it used to send no CSRF token, which would have 403'd.
 * Every call site is a GET today, so this was latent rather than broken, but a
 * wrapper that quietly drops the header is how the original defect class got
 * in. The token is attached for non-safe methods, from the canonical
 * readCsrfToken() so the in-memory fallback survives a proxy that rewrites
 * Set-Cookie.
 */
export async function downloadBlob(
  url: string,
  filename: string,
  init?: RequestInit,
): Promise<boolean> {
  const method = (init?.method ?? 'GET').toUpperCase()
  const csrf = SAFE_METHODS.has(method) ? null : readCsrfToken()
  try {
    const res = await fetch(url, {
      credentials: 'include',
      ...init,
      // After ...init so a caller's headers cannot drop the token — the
      // v0.42.22 spread-order rule.
      headers: { ...(init?.headers as Record<string, string> | undefined), ...(csrf ? { 'X-CSRF-Token': csrf } : {}) },
    })
    if (!res.ok) {
      const body = (await res.json().catch(() => ({}))) as { error?: string; message?: string }
      throw new Error(body.error ?? body.message ?? `HTTP ${res.status.toString()}`)
    }
    const blob = await res.blob()
    const objectURL = URL.createObjectURL(blob)
    const a = document.createElement('a')
    a.href = objectURL
    a.download = filename
    document.body.appendChild(a)
    a.click()
    a.remove()
    URL.revokeObjectURL(objectURL)
    return true
  } catch (err) {
    toast(err instanceof Error ? err.message : 'Export fehlgeschlagen', 'error')
    return false
  }
}
