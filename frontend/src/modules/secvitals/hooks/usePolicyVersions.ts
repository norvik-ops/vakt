import { useQuery } from '@tanstack/react-query'
import { apiFetch } from '../../../api/client'

export interface PolicyVersion {
  id: string
  org_id: string
  policy_id: string
  version: number
  title: string
  content: string
  status: string
  version_note: string
  updated_by: string
  created_at: string
}

export function usePolicyVersions(policyId: string | undefined) {
  return useQuery<PolicyVersion[]>({
    queryKey: ['secvitals', 'policies', policyId, 'versions'],
    queryFn: () => apiFetch<PolicyVersion[]>(`/secvitals/policies/${policyId}/versions`),
    enabled: !!policyId,
    staleTime: 2 * 60 * 1000,
  })
}

export function usePolicyVersion(policyId: string | undefined, version: number | undefined) {
  return useQuery<PolicyVersion>({
    queryKey: ['secvitals', 'policies', policyId, 'versions', version],
    queryFn: () => apiFetch<PolicyVersion>(`/secvitals/policies/${policyId}/versions/${version}`),
    enabled: !!policyId && version !== undefined && version >= 1,
    staleTime: 5 * 60 * 1000,
  })
}
