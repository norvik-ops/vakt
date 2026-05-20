const API_BASE = '/api/v1'

// User info stored in localStorage — does NOT include the access token.
// The access token lives in an httpOnly cookie managed by the backend.
export interface UserInfo {
  id: string
  email: string
  role: string
  display_name?: string
  roles?: string[]
}

export function getUserInfo(): UserInfo | null {
  try {
    const raw = localStorage.getItem('vakt_user')
    if (!raw) return null
    return JSON.parse(raw) as UserInfo
  } catch {
    return null
  }
}

export function setUserInfo(user: UserInfo | null): void {
  if (user) localStorage.setItem('vakt_user', JSON.stringify(user))
  else localStorage.removeItem('vakt_user')
}

// Legacy compatibility shim — callers that still invoke setAuthToken(null) on
// logout will clear vakt_user.  New callers should prefer setUserInfo().
export function setAuthToken(token: string | null): void {
  if (!token) setUserInfo(null)
}

// Returns true when a user session exists (cookie is managed by the browser;
// we track session presence via the vakt_user key in localStorage).
export function getAuthToken(): boolean {
  return getUserInfo() !== null
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

// Read the CSRF token from the `csrf_token` cookie (set by the backend on
// login/refresh). The cookie is intentionally NOT HttpOnly so we can echo it
// back in the X-CSRF-Token header — the double-submit-cookie pattern.
function readCsrfToken(): string | null {
  const match = document.cookie.match(/(?:^|;\s*)csrf_token=([^;]+)/)
  return match ? decodeURIComponent(match[1]) : null
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

export async function apiFetch<T>(
  path: string,
  options?: Omit<RequestInit, 'headers'> & { headers?: Record<string, string> },
): Promise<T> {
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

  let lastError: unknown = null
  for (let attempt = 0; attempt <= MAX_RETRIES; attempt++) {
    let res: Response
    try {
      res = await fetch(`${API_BASE}${path}`, {
        credentials: 'include', // send httpOnly cookie automatically
        headers: {
          'Content-Type': 'application/json',
          ...csrfHeader,
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
      setUserInfo(null)
      window.location.href = '/login'
      throw new Error('Unauthorized')
    }

    if (res.status === 402) {
      const body = (await res.json().catch(() => ({}))) as { feature?: string }
      throw new FeatureLockedError(body.feature ?? 'unknown')
    }

    if (res.status === 403) {
      const body = (await res.json().catch(() => ({}))) as { code?: string }
      if (body.code === 'MFA_REQUIRED') {
        window.location.href = '/account'
        throw new MFARequiredError()
      }
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
      throw new Error(body.error ?? `HTTP ${res.status.toString()}`)
    }

    if (res.status === 204) return undefined as T

    const contentType = res.headers.get('content-type') ?? ''
    if (contentType.includes('application/octet-stream') || contentType.includes('text/csv')) {
      return res.blob() as Promise<T>
    }
    return res.json() as Promise<T>
  }
  throw lastError ?? new Error('apiFetch: retry budget exhausted')
}
