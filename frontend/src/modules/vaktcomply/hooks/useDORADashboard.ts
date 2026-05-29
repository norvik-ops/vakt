import { useQuery } from '@tanstack/react-query'
import { FeatureLockedError } from '../../../api/client'
import type { DORADashboard } from '../types'

interface DORADashboardResult {
  data: DORADashboard | null
  notEnabled: boolean
}

async function fetchDORADashboard(): Promise<DORADashboardResult> {
  const res = await fetch('/api/v1/vaktcomply/dora/dashboard', {
    credentials: 'include',
    headers: { 'Content-Type': 'application/json' },
  })

  if (res.status === 404) {
    return { data: null, notEnabled: true }
  }

  if (res.status === 401) {
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

  const data = (await res.json()) as DORADashboard
  return { data, notEnabled: false }
}

export function useDORADashboard() {
  return useQuery<DORADashboardResult>({
    queryKey: ['vaktcomply', 'dora', 'dashboard'],
    queryFn: fetchDORADashboard,
    staleTime: 5 * 60 * 1000,
  })
}
