import { useQuery } from '@tanstack/react-query'
import { apiFetch } from '../../api/client'

interface LicenseStatus {
  tier: string
  is_pro: boolean
  features: string[]
}

function useLicenseStatus() {
  return useQuery<LicenseStatus>({
    queryKey: ['license'],
    queryFn: () => apiFetch<LicenseStatus>('/license'),
    staleTime: 5 * 60 * 1000,
    retry: false,
  })
}

/**
 * Returns whether a named Pro feature is enabled on the current license.
 * Use this to render the ProGate *before* the first failing API call,
 * eliminating the API-roundtrip flicker for Community users.
 */
export function useFeature(feature: string): {
  enabled: boolean
  loading: boolean
} {
  const { data, isLoading } = useLicenseStatus()
  if (isLoading) return { enabled: true, loading: true }
  if (!data) return { enabled: true, loading: false }
  const enabled = data.is_pro && data.features.includes(feature)
  return { enabled, loading: false }
}
