const API_BASE = '/api/v1'

// User identity for client-side use. Source of truth lives on the server;
// after page reload the SPA fetches /auth/me to rehydrate. We no longer
// persist this object in localStorage (audit F032 — no PII at rest in the
// browser). Auth presence is signalled by the httpOnly cookie set by the
// backend on login.
export interface UserInfo {
  id: string
  email: string
  role: string
  display_name?: string
  roles?: string[]
}

export interface AuthMe {
  id: string
  email: string
  display_name: string
  roles: string[]
  csrf_token?: string
}

export async function fetchMe(): Promise<AuthMe | null> {
  try {
    const res = await fetch(`${API_BASE}/auth/me`, {
      credentials: 'include',
      headers: { 'Content-Type': 'application/json' },
    })
    if (!res.ok) return null
    const me = (await res.json()) as AuthMe
    setCsrfToken(me.csrf_token)
    return me
  } catch {
    return null
  }
}

// Session-ID (refresh_sessions.id) wird beim Login vom Backend zurückgegeben
// und nur dazu verwendet, in der SessionsPage die aktuelle Session zu markieren
// und beim Revoke-All sich selbst auszuschließen. Kein Sicherheitsmechanismus —
// rein UX.
export function getSessionId(): string | null {
  try {
    return localStorage.getItem('vakt_session_id')
  } catch {
    return null
  }
}

export function setSessionId(id: string | null): void {
  if (id) localStorage.setItem('vakt_session_id', id)
  else localStorage.removeItem('vakt_session_id')
}

export class FeatureLockedError extends Error {
  constructor(public readonly feature: string) {
    super(`Pro feature required: ${feature}`)
    this.name = 'FeatureLockedError'
  }
}

export class MFARequiredError extends Error {
  constructor() {
    super('MFA_REQUIRED')
    this.name = 'MFARequiredError'
  }
}

export class RateLimitedError extends Error {
  constructor(public readonly retryAfterSeconds: number) {
    super(`Zu viele Anfragen — bitte ${retryAfterSeconds.toString()} Sekunden warten`)
    this.name = 'RateLimitedError'
  }
}

// Retry idempotent methods (GET/HEAD/OPTIONS) on transient network failures and
// 5xx responses. Non-idempotent methods (POST/PUT/PATCH/DELETE) are retried only
// on a true network failure (where no request actually reached the server), never
// on a server response, since we cannot tell whether the action was applied.
const RETRYABLE_STATUS = new Set([500, 502, 503, 504])
const IDEMPOTENT_METHODS = new Set(['GET', 'HEAD', 'OPTIONS'])
const MAX_RETRIES = 3
const BASE_BACKOFF_MS = 300

// In-memory fallback for the CSRF token. Some reverse proxies/CDNs in front
// of an instance rewrite Set-Cookie headers (e.g. adding HttpOnly), which
// makes the csrf_token cookie unreadable via document.cookie even though the
// browser still sends it correctly on requests — every mutation then 403s
// with "CSRF header missing". The backend echoes the same token value in the
// login/refresh/me response bodies (see AuthResponse.CSRFToken), which no
// proxy can interfere with; setCsrfToken() below caches it here.
let inMemoryCsrfToken: string | null = null
export function setCsrfToken(token: string | null | undefined): void {
  inMemoryCsrfToken = token ?? null
}

// Read the CSRF token from the `csrf_token` cookie (set by the backend on
// login/refresh). The cookie is intentionally NOT HttpOnly so we can echo it
// back in the X-CSRF-Token header — the double-submit-cookie pattern. Falls
// back to the in-memory value from setCsrfToken() when the cookie isn't
// JS-readable (see above).
function readCsrfToken(): string | null {
  const match = document.cookie.match(/(?:^|;\s*)csrf_token=([^;]+)/)
  return match ? decodeURIComponent(match[1]) : inMemoryCsrfToken
}

function backoffDelay(attempt: number): number {
  // Exponential with full jitter: random(0, base * 2^attempt), capped at 5s
  const capped = Math.min(BASE_BACKOFF_MS * 2 ** attempt, 5000)
  return Math.floor(Math.random() * capped)
}

function parseRetryAfter(headerValue: string | null): number {
  if (!headerValue) return 1
  const seconds = parseInt(headerValue, 10)
  if (!isNaN(seconds) && seconds >= 0) return seconds
  // HTTP-date format — best effort
  const date = Date.parse(headerValue)
  if (!isNaN(date)) {
    return Math.max(1, Math.ceil((date - Date.now()) / 1000))
  }
  return 1
}

// onUnauthorized is called by apiFetch when the server returns 401, so the
// auth store can clear in-memory state before the redirect. Wired up in
// shared/stores/auth.ts to avoid a static import cycle.
let onUnauthorized: (() => void) | null = null
export function registerUnauthorizedHandler(fn: () => void): void {
  onUnauthorized = fn
}

export async function apiFetch<T>(
  path: string,
  options?: Omit<RequestInit, 'headers'> & { headers?: Record<string, string> },
): Promise<T> {
  // Guard against double-prefix: callers must use relative paths like /vakthr/...
  // not /api/v1/vakthr/... (apiFetch already prepends API_BASE).
  if (path.startsWith('/api/v1/')) {
    if (import.meta.env.DEV) {
      throw new Error(
        `apiFetch: path must not include the API base prefix. Got: "${path}". Use "${path.slice('/api/v1'.length)}" instead.`,
      )
    }
    // In production: strip silently so the app keeps working.
    path = path.slice('/api/v1'.length)
  }
  const method = (options?.method ?? 'GET').toUpperCase()
  const isIdempotent = IDEMPOTENT_METHODS.has(method)

  // Attach the CSRF token to every state-changing request. The backend's
  // CSRF middleware ignores safe methods, so this is a no-op for those —
  // we attach unconditionally to keep the code simple and to support cases
  // where a GET endpoint is later upgraded to mutate state.
  const csrfHeader: Record<string, string> = {}
  if (!isIdempotent) {
    const token = readCsrfToken()
    if (token) csrfHeader['X-CSRF-Token'] = token
  }

  // X-Vakt-Session-Id: rein kosmetischer Hint fürs Backend, damit die
  // SessionsPage die "diese hier"-Markierung setzen + Revoke-All-Others
  // sich selbst ausnehmen kann.
  const sessionHeader: Record<string, string> = {}
  const sessionId = getSessionId()
  if (sessionId) sessionHeader['X-Vakt-Session-Id'] = sessionId

  let lastError: unknown = null
  for (let attempt = 0; attempt <= MAX_RETRIES; attempt++) {
    let res: Response
    try {
      res = await fetch(`${API_BASE}${path}`, {
        credentials: 'include', // send httpOnly cookie automatically
        headers: {
          'Content-Type': 'application/json',
          ...csrfHeader,
          ...sessionHeader,
          ...(options?.headers ?? {}),
        },
        ...options,
      })
    } catch (err) {
      // Network failure — retry only if we have attempts left.
      // Safe for non-idempotent methods too: no request reached the server.
      lastError = err
      if (attempt < MAX_RETRIES) {
        await new Promise(resolve => setTimeout(resolve, backoffDelay(attempt)))
        continue
      }
      throw err
    }

    if (res.status === 401) {
      onUnauthorized?.()
      setSessionId(null)
      // S90-8 (#10): a full-page navigation (not react-router `navigate`) is
      // deliberate. On session invalidation we want a hard reset of ALL
      // in-memory state — Zustand stores, React component state, TanStack Query
      // cache — so no stale authenticated data survives the logout. A soft SPA
      // navigation would preserve that memory. The minor UX cost (lost router
      // state) is an acceptable trade for the guaranteed clean slate.
      window.location.href = '/login'
      throw new Error('Unauthorized')
    }

    if (res.status === 402) {
      const body = (await res.json().catch(() => ({}))) as { feature?: string }
      throw new FeatureLockedError(body.feature ?? 'unknown')
    }

    if (res.status === 403) {
      const body = (await res.json().catch(() => ({}))) as { code?: string; error?: string }
      if (body.code === 'MFA_REQUIRED') {
        window.location.href = '/account'
        throw new MFARequiredError()
      }
      throw new Error(body.error ?? 'Keine Berechtigung für diese Aktion')
    }

    if (res.status === 429) {
      const retryAfter = parseRetryAfter(res.headers.get('Retry-After'))
      if (isIdempotent && attempt < MAX_RETRIES) {
        const delayMs = Math.min(retryAfter * 1000, 5000)
        await new Promise(resolve => setTimeout(resolve, delayMs))
        continue
      }
      throw new RateLimitedError(retryAfter)
    }

    if (RETRYABLE_STATUS.has(res.status) && isIdempotent && attempt < MAX_RETRIES) {
      await new Promise(resolve => setTimeout(resolve, backoffDelay(attempt)))
      continue
    }

    if (!res.ok) {
      const body = (await res.json().catch(() => ({}))) as { error?: string }
      // Map common HTTP status codes to user-friendly German messages
      const fallback =
        res.status >= 500
          ? 'Interner Fehler — bitte erneut versuchen'
          : `HTTP ${res.status.toString()}`
      throw new Error(body.error ?? fallback)
    }

    if (res.status === 204) return undefined as T

    const contentType = res.headers.get('content-type') ?? ''
    if (contentType.includes('application/octet-stream') || contentType.includes('text/csv')) {
      return res.blob() as Promise<T>
    }
    return res.json() as Promise<T>
  }
  throw lastError instanceof Error ? lastError : new Error('apiFetch: retry budget exhausted')
}
