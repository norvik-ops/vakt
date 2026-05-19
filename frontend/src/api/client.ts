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

export async function apiFetch<T>(
  path: string,
  options?: Omit<RequestInit, 'headers'> & { headers?: Record<string, string> },
): Promise<T> {
  const res = await fetch(`${API_BASE}${path}`, {
    credentials: 'include', // send httpOnly cookie automatically
    headers: {
      'Content-Type': 'application/json',
      ...(options?.headers ?? {}),
    },
    ...options,
  })

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
