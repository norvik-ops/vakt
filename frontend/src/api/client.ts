const API_BASE = '/api/v1'

let _token: string | null = null

export function setAuthToken(token: string | null) {
  _token = token
  if (token) localStorage.setItem('ss_token', token)
  else localStorage.removeItem('ss_token')
}

export function getAuthToken(): string | null {
  if (!_token) _token = localStorage.getItem('ss_token')
  return _token
}

export class FeatureLockedError extends Error {
  constructor(public readonly feature: string) {
    super(`Pro feature required: ${feature}`)
    this.name = 'FeatureLockedError'
  }
}

export async function apiFetch<T>(
  path: string,
  options?: Omit<RequestInit, 'headers'> & { headers?: Record<string, string> },
): Promise<T> {
  const token = getAuthToken()
  const res = await fetch(`${API_BASE}${path}`, {
    headers: {
      'Content-Type': 'application/json',
      ...(token ? { Authorization: `Bearer ${token}` } : {}),
      ...(options?.headers ?? {}),
    },
    ...options,
  })

  if (res.status === 401) {
    setAuthToken(null)
    window.location.href = '/login'
    throw new Error('Unauthorized')
  }

  if (res.status === 402) {
    const body = (await res.json().catch(() => ({}))) as { feature?: string }
    throw new FeatureLockedError(body.feature ?? 'unknown')
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
