import { toast } from '../hooks/useToast'

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
 */
export async function downloadBlob(
  url: string,
  filename: string,
  init?: RequestInit,
): Promise<boolean> {
  try {
    const res = await fetch(url, { credentials: 'include', ...init })
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
