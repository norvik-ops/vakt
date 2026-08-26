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

// Statuses that mean "the server did not answer the question", as opposed to
// "the server answered: you are not signed in". Kept separate from
// RETRYABLE_STATUS below because that set drives apiFetch's retry policy and
// deliberately excludes 429 (apiFetch handles rate limits on their own path).
const HYDRATE_TRANSIENT_STATUS = new Set([429, 500, 502, 503, 504])
const HYDRATE_MAX_ATTEMPTS = 3
const HYDRATE_BACKOFF_MS = 300

/**
 * Resolve the current user for the auth store, or null if there is none.
 *
 * R1-SA21-D1 (expiry path): this used to be `if (!res.ok) return null`, which
 * read *every* non-2xx answer as "not signed in" — a 429 from the rate limiter,
 * a 502 while the API restarts, a dropped connection on a flaky network. The
 * store then cleared the user and the route guard bounced a perfectly valid
 * session to /login. Only 401/403 actually carry that meaning; the rest mean
 * "ask again".
 *
 * This is the twin of the sign-out defect, pointing the other way: there the
 * client tore the session down locally without telling the server, here it tore
 * it down locally although the server never said to.
 *
 * The safe default is unchanged: once the small retry budget is spent we still
 * return null, so an API that is genuinely down lands the user on /login rather
 * than in a half-rendered shell. Bounded on purpose — three attempts with a
 * 300 ms base back off in well under two seconds, and the route guard shows a
 * spinner meanwhile, so the cost of the retry is a slightly longer splash, not
 * a hang.
 */
export async function fetchMe(): Promise<AuthMe | null> {
  for (let attempt = 0; attempt < HYDRATE_MAX_ATTEMPTS; attempt++) {
    const isLastAttempt = attempt === HYDRATE_MAX_ATTEMPTS - 1
    let res: Response
    try {
      res = await fetch(`${API_BASE}/auth/me`, {
        credentials: 'include',
        headers: { 'Content-Type': 'application/json' },
      })
    } catch {
      // No answer at all — the most transient case there is.
      if (isLastAttempt) return null
      await new Promise((resolve) => setTimeout(resolve, HYDRATE_BACKOFF_MS * 2 ** attempt))
      continue
    }

    if (res.ok) {
      try {
        const me = (await res.json()) as AuthMe
        setCsrfToken(me.csrf_token)
        return me
      } catch {
        // 2xx with an unreadable body is a broken contract, not a session state.
        return null
      }
    }

    if (HYDRATE_TRANSIENT_STATUS.has(res.status) && !isLastAttempt) {
      const waitMs =
        res.status === 429
          ? Math.min(parseRetryAfter(res.headers.get('Retry-After')) * 1000, 2000)
          : HYDRATE_BACKOFF_MS * 2 ** attempt
      await new Promise((resolve) => setTimeout(resolve, waitMs))
      continue
    }

    return null
  }
  return null
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

// MFAStepUpError is thrown when a sensitive WRITE needs a fresh TOTP (org opted
// into require_mfa_sensitive_calls) and the user cancelled the challenge or no
// challenge UI is mounted. Distinct from MFARequiredError (login-time enrolment):
// this is a per-action step-up and must NOT log the user out.
export class MFAStepUpError extends Error {
  constructor() {
    super('MFA_STEP_UP_CANCELLED')
    this.name = 'MFAStepUpError'
  }
}

export class RateLimitedError extends Error {
  constructor(public readonly retryAfterSeconds: number) {
    super(`Zu viele Anfragen — bitte ${retryAfterSeconds.toString()} Sekunden warten`)
    this.name = 'RateLimitedError'
  }
}

/**
 * A failed request that carries the HTTP status that caused it.
 *
 * apiFetch used to throw a bare `new Error(message)` for every non-2xx answer,
 * which flattens two very different situations into one: "the server rejected
 * this" and "the server never managed to answer". A caller that has to tell
 * them apart — sign-out being the case in point, where only the second means
 * the session may still be alive — was left matching on message strings.
 *
 * Additive on purpose: it is still an Error and the message is unchanged, so
 * existing catch blocks (including the `message === 'Unauthorized'` check in
 * main.tsx) keep working untouched.
 */
export class ApiError extends Error {
  constructor(message: string, public readonly status: number) {
    super(message)
    this.name = 'ApiError'
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
//
// Exported for the handful of writes that genuinely cannot use apiFetch —
// SSE streams that need the raw Response body, and downloads that return a
// blob apiFetch would try to parse as JSON. Those must attach the header
// themselves, and they must do it from HERE: several files grew their own
// private copy that reads only document.cookie and therefore loses the
// in-memory fallback above, which is exactly what keeps CSRF working behind a
// proxy that rewrites Set-Cookie.
export function readCsrfToken(): string | null {
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

// onMFAChallenge is invoked by apiFetch when a sensitive write returns
// MFA_TOKEN_REQUIRED / MFA_TOKEN_INVALID. It must resolve with the 6-digit TOTP
// code the user entered, or null if they cancelled. Wired by MFAChallengeProvider.
// `invalid` is true when the previous code was rejected (so the UI can say so).
let onMFAChallenge: ((invalid: boolean) => Promise<string | null>) | null = null
export function registerMFAChallengeHandler(
  fn: (invalid: boolean) => Promise<string | null>,
): void {
  onMFAChallenge = fn
}
const MAX_MFA_PROMPTS = 3

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

  // Multipart uploads (R1-18-D4). A FormData body carries a boundary that only
  // the browser can generate, and it does so only when NO Content-Type header
  // is present — any value we write, including the JSON default below or one a
  // caller copied from a JSON hook, replaces it with a boundary-less string the
  // server cannot parse. Before this, every FormData upload therefore went
  // around apiFetch with a raw fetch() and lost X-CSRF-Token, so all seven of
  // them 403'd from the day they were written. Handling it here keeps the CSRF
  // and session headers on one path instead of seven hand-rolled copies.
  const isFormData = typeof FormData !== 'undefined' && options?.body instanceof FormData

  const buildHeaders = (mfa: string): Record<string, string> => {
    // Spread order mirrors the fetch() call below: caller headers merge into
    // the constructed object per key, they never replace it wholesale.
    const headers: Record<string, string> = {
      ...(isFormData ? {} : { 'Content-Type': 'application/json' }),
      ...csrfHeader,
      ...sessionHeader,
      ...(options?.headers ?? {}),
      // After the caller's headers so a step-up token is never clobbered.
      ...(mfa ? { 'X-MFA-Token': mfa } : {}),
    }
    if (!isFormData) return headers
    // Strip any Content-Type a caller supplied — on FormData it is always
    // wrong, whatever its casing.
    return Object.fromEntries(
      Object.entries(headers).filter(([key]) => key.toLowerCase() !== 'content-type'),
    )
  }

  // Step-up MFA (S131-R-H24): filled after the server asks for a TOTP on a
  // sensitive write; re-sent on the retry. mfaPrompts caps the challenge loop so
  // repeated wrong codes cannot spin forever.
  let mfaToken = ''
  let mfaPrompts = 0

  let lastError: unknown = null
  for (let attempt = 0; attempt <= MAX_RETRIES; attempt++) {
    let res: Response
    try {
      res = await fetch(`${API_BASE}${path}`, {
        ...options,
        credentials: 'include', // send httpOnly cookie automatically
        // Spread after ...options (not before): options.headers (e.g. every
        // mutation hook passes { 'Content-Type': 'application/json' }) would
        // otherwise silently replace this whole object at the top level,
        // wiping out X-CSRF-Token and X-Vakt-Session-Id on every request that
        // sets its own headers — the actual cause of the CSRF-header-missing
        // bug, unrelated to cookie readability.
        headers: buildHeaders(mfaToken),
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
      // Step-up MFA on a sensitive write returns 401 with an MFA code — this is
      // NOT a session-expiry logout. Prompt for a TOTP and retry with the token.
      const mfaBody = (await res
        .clone()
        .json()
        .catch(() => ({}))) as { code?: string }
      if (mfaBody.code === 'MFA_TOKEN_REQUIRED' || mfaBody.code === 'MFA_TOKEN_INVALID') {
        if (onMFAChallenge && mfaPrompts < MAX_MFA_PROMPTS) {
          mfaPrompts++
          const code = await onMFAChallenge(mfaBody.code === 'MFA_TOKEN_INVALID')
          if (code) {
            mfaToken = code
            attempt-- // the step-up re-submit must not consume a network-retry
            continue
          }
        }
        // Cancelled, no UI mounted, or too many wrong tries — surface as a
        // step-up error. Crucially: do NOT log the user out.
        throw new MFAStepUpError()
      }

      onUnauthorized?.()
      setSessionId(null)
      // S90-8 (#10): a full-page navigation (not react-router `navigate`) is
      // deliberate. On session invalidation we want a hard reset of ALL
      // in-memory state — Zustand stores, React component state, TanStack Query
      // cache — so no stale authenticated data survives the logout. A soft SPA
      // navigation would preserve that memory. The minor UX cost (lost router
      // state) is an acceptable trade for the guaranteed clean slate.
      window.location.href = '/login'
      throw new ApiError('Unauthorized', 401)
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
      throw new ApiError(body.error ?? fallback, res.status)
    }

    if (res.status === 204) return undefined as T

    // Binary responses come back as a Blob. application/pdf and .zip belong
    // here for the same reason octet-stream does: res.json() on them throws,
    // so an endpoint returning one could not use apiFetch at all — which is
    // precisely why the NIS2 PDF export hand-rolled its own fetch and lost its
    // CSRF header (R1-18-D4). Any caller that was already receiving one of
    // these through apiFetch was broken before this change, so widening the
    // branch cannot regress a working path.
    const contentType = res.headers.get('content-type') ?? ''
    const isBinary =
      contentType.includes('application/octet-stream') ||
      contentType.includes('application/pdf') ||
      contentType.includes('application/zip') ||
      contentType.includes('text/csv')
    if (isBinary) {
      return res.blob() as Promise<T>
    }
    return res.json() as Promise<T>
  }
  throw lastError instanceof Error ? lastError : new Error('apiFetch: retry budget exhausted')
}
